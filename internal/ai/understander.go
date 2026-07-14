package ai

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
	"unicode"

	"StarCore/internal/agent"
)

// --- Understander ---
// The Understander is the "first layer" of the 5-layer architecture:
//   Understand → Route → Execute → Supervise → Deliver
//
// It performs deeper analysis of the user's message before any routing:
//   1. Intent classification (enhanced, building on IntentClassifier)
//   2. Entity extraction (files, functions, line numbers)
//   3. Ambiguity detection (is the request clear enough?)
//   4. Clarification generation (what to ask if ambiguous?)
//
// Design principles:
//   - No LLM calls — pure rule/heuristic engine (fast, free)
//   - Deterministic output — same input always yields same Understanding
//   - Fallback-friendly — when uncertain, asks for clarification instead of guessing
//   - Always provides a default — never blocks the pipeline

// Understanding is the result of the Understander's analysis.
type Understanding struct {
	// Original message (trimmed)
	Message string

	// Intent (enhanced classification)
	Intent     agent.IntentType
	Confidence float64
	Keywords   []string
	Language   string

	// Extracted entities
	Entities Entities

	// Ambiguity assessment
	Ambiguity AmbiguityStatus

	// Clarification (when ambiguous)
	Clarification *Clarification

	// Routing hint
	RouteHint RoutePreference

	// Analysis metadata
	AnalysisMeta AnalysisMetadata
}

// Entities holds extracted references from the message.
type Entities struct {
	Files       []string // file paths mentioned ("main.go", "src/utils/helpers.go")
	Functions   []string // function/method names mentioned
	LineNumbers []int    // line numbers mentioned (42, 100-120)
	Symbols     []string // code symbols (variable/type names)
	Targets     []string // action targets (what to modify/create/delete)
}

// AmbiguityStatus indicates how clear the request is.
type AmbiguityStatus int

const (
	AmbiguityNone   AmbiguityStatus = iota // clear intent, proceed
	AmbiguityLow                           // minor uncertainty, can proceed with defaults
	AmbiguityMedium                        // missing key info, suggest clarification
	AmbiguityHigh                          // very vague, must clarify before proceeding
)

func (a AmbiguityStatus) String() string {
	switch a {
	case AmbiguityNone:
		return "none"
	case AmbiguityLow:
		return "low"
	case AmbiguityMedium:
		return "medium"
	case AmbiguityHigh:
		return "high"
	}
	return "unknown"
}

// Clarification represents a question to ask the user.
type Clarification struct {
	Question string   // the question to show the user
	Options  []string // optional preset choices
	Context  string   // brief explanation why clarification is needed
	Priority int      // 1=must ask, 2=nice to have, 3=optional
}

// RoutePreference hints at the best Route action.
type RoutePreference struct {
	AgentID     string // suggested agent (empty = use default)
	Mode        string // suggested mode (chat/plan/build)
	Complexity  string // simple/moderate/complex
	AutoProceed bool   // whether to proceed without asking
}

// AnalysisMetadata provides debugging/tracing info.
type AnalysisMetadata struct {
	MessageLength int      // chars in the message
	WordCount     int      // approx word count
	HasCodeBlock  bool     // contains ``` markers
	HasFileRef    bool     // contains file path references
	HasActionVerb bool     // contains an action verb (add/fix/etc.)
	AnalysisSteps []string // which analysis steps fired
}

// Understander performs message analysis.
type Understander struct {
	intentClassifier *agent.IntentClassifier
	filePattern      *regexp.Regexp
	funcPattern      *regexp.Regexp
	linePattern      *regexp.Regexp
}

// NewUnderstander creates an Understander with default configuration.
func NewUnderstander() *Understander {
	return &Understander{
		intentClassifier: agent.NewIntentClassifier(),
		// Matches: src/main.go, utils/helpers.ts, path/to/file.py, etc.
		filePattern: regexp.MustCompile(`[\w./\\-]+\.(go|js|ts|jsx|tsx|py|rs|java|c|cpp|h|hpp|css|html|json|yaml|yml|md|sh|bash|sql|xml|vue|svelte)`),
		// Matches: function calls like foo(), bar(baz), MyFunc()
		funcPattern: regexp.MustCompile(`\b([A-Z][a-zA-Z0-9_]*|\w+)\s*\(`),
		// Matches: :42, line 100, L123, lines 10-20
		linePattern: regexp.MustCompile(`(?::(\d+)|line\s*(\d+)|L(\d+)|\bL(\d+)\s*[-–]\s*(\d+))`),
	}
}

// Understand performs full analysis of a user message.
func (u *Understander) Understand(message string) *Understanding {
	message = strings.TrimSpace(message)
	if message == "" {
		return u.emptyUnderstanding()
	}

	intent := u.intentClassifier.Classify(message)
	entities := u.extractEntities(message)
	ambiguity := u.assessAmbiguity(message, intent, entities)
	var clarification *Clarification
	if ambiguity >= AmbiguityMedium {
		clarification = u.generateClarification(message, intent, entities, ambiguity)
	}
	routeHint := u.suggestRoute(intent, entities, ambiguity)
	meta := u.buildMetadata(message, entities)

	return &Understanding{
		Message:       message,
		Intent:        intent.Intent,
		Confidence:    intent.Confidence,
		Keywords:      intent.Keywords,
		Language:      intent.Language,
		Entities:      entities,
		Ambiguity:     ambiguity,
		Clarification: clarification,
		RouteHint:     routeHint,
		AnalysisMeta:  meta,
	}
}

// CanProceed returns true if the Understander recommends proceeding
// without asking for clarification.
func (u *Understander) CanProceed(understanding *Understanding) bool {
	return understanding.Ambiguity < AmbiguityMedium && understanding.RouteHint.AutoProceed
}

// --- Analysis steps ---

func (u *Understander) emptyUnderstanding() *Understanding {
	return &Understanding{
		Message:    "",
		Intent:     agent.IntentChat,
		Confidence: 0,
		Ambiguity:  AmbiguityHigh,
		Clarification: &Clarification{
			Question: "你好！请告诉我你需要什么帮助？",
			Priority: 1,
		},
		RouteHint: RoutePreference{Mode: "chat", AutoProceed: false},
	}
}

func (u *Understander) extractEntities(message string) Entities {
	var entities Entities

	// Extract file paths
	fileMatches := u.filePattern.FindAllString(message, -1)
	seen := make(map[string]bool)
	for _, m := range fileMatches {
		// Filter out common false positives (e.g. "a.go" as a word, not a file)
		clean := strings.TrimSpace(m)
		if len(clean) > 2 && !seen[clean] {
			// Skip if it looks like a sentence ending (e.g. "file.go. But...")
			ext := filepath.Ext(clean)
			if isKnownExt(ext) {
				entities.Files = append(entities.Files, clean)
				seen[clean] = true
			}
		}
	}

	// Extract function references
	funcMatches := u.funcPattern.FindAllString(message, -1)
	seenFunc := make(map[string]bool)
	for _, m := range funcMatches {
		name := strings.TrimSuffix(m, "(")
		name = strings.TrimSpace(name)
		if len(name) >= 2 && !seenFunc[name] && !isCommonWord(name) {
			entities.Functions = append(entities.Functions, name)
			seenFunc[name] = true
		}
	}

	// Extract line numbers
	lineMatches := u.linePattern.FindAllStringSubmatch(message, -1)
	for _, m := range lineMatches {
		for i := 1; i < len(m); i++ {
			if m[i] != "" {
				var num int
				fmt.Sscanf(m[i], "%d", &num)
				if num > 0 {
					entities.LineNumbers = append(entities.LineNumbers, num)
				}
			}
		}
	}

	// Extract action verbs as targets
	actionVerbs := []string{"add", "fix", "create", "remove", "delete", "implement",
		"修改", "添加", "删除", "修复", "实现", "创建", "优化", "重构"}
	msgLower := strings.ToLower(message)
	for _, verb := range actionVerbs {
		if strings.Contains(msgLower, verb) {
			entities.Targets = append(entities.Targets, verb)
		}
	}

	return entities
}

func (u *Understander) assessAmbiguity(message string, intent *agent.IntentResult, entities Entities) AmbiguityStatus {
	msgLen := len([]rune(message))
	wordCount := len(strings.Fields(message))

	// Very short message without clear action
	if wordCount < 3 && intent.Confidence < agent.MediumConfidence {
		return AmbiguityHigh
	}

	// Multiple conflicting intents detected via conjunctions.
	// Check for many "separate request" indicators — if a message strings
	// together 3+ actions with conjunctions, it's ambiguous which to do first.
	multiIndicators := []string{"并且", "还有", "同时", "另外", "还要", "以及", "also", "and then", "plus"}
	conjunctionCount := 0
	msgLower := strings.ToLower(message)
	for _, ind := range multiIndicators {
		if strings.Contains(msgLower, ind) {
			conjunctionCount++
		}
	}
	if conjunctionCount >= 3 {
		return AmbiguityMedium
	}

	// Low confidence intent
	if intent.Confidence < agent.LowConfidence+0.1 {
		return AmbiguityHigh
	}

	// Medium confidence with no entities
	if intent.Confidence < agent.MediumConfidence {
		if len(entities.Files) == 0 && len(entities.Targets) == 0 {
			return AmbiguityMedium
		}
		return AmbiguityLow
	}

	// High confidence but missing specific targets for action intents
	if intent.Intent == agent.IntentCodeEdit || intent.Intent == agent.IntentDebug ||
		intent.Intent == agent.IntentRefactor {
		if len(entities.Files) == 0 && len(entities.Functions) == 0 && msgLen < 30 {
			return AmbiguityMedium
		}
	}

	// Very short technical message (likely ambiguous reference)
	if msgLen < 15 && intent.Intent != agent.IntentChat {
		return AmbiguityLow
	}

	return AmbiguityNone
}

func (u *Understander) generateClarification(
	message string,
	intent *agent.IntentResult,
	entities Entities,
	ambiguity AmbiguityStatus,
) *Clarification {
	switch intent.Intent {
	case agent.IntentCodeEdit, agent.IntentDebug, agent.IntentRefactor:
		if len(entities.Files) == 0 {
			return &Clarification{
				Question: "你想修改哪个文件或哪个功能的代码？",
				Context:  "没有检测到具体的文件参考",
				Priority: 1,
			}
		}
		if len(entities.Targets) == 0 {
			return &Clarification{
				Question: fmt.Sprintf("对 %s 你想做什么操作？", strings.Join(entities.Files, ", ")),
				Options:  []string{"添加功能", "修复问题", "重构优化", "解释说明"},
				Priority: 1,
			}
		}
	case agent.IntentSearch:
		return &Clarification{
			Question: "你想搜索什么？请提供更具体的关键词或代码片段。",
			Priority: 2,
		}
	case agent.IntentPlan:
		if ambiguity == AmbiguityHigh {
			return &Clarification{
				Question: "你想规划什么？请描述项目的目标和范围。",
				Priority: 1,
			}
		}
	case agent.IntentChat:
		// Even for chat, if we're very unsure, offer options
		if ambiguity == AmbiguityHigh {
			return &Clarification{
				Question: "你想让我帮你做什么？",
				Options:  []string{"编写代码", "分析项目", "修复问题", "聊天讨论"},
				Priority: 2,
			}
		}
	}

	// Generic fallback
	return &Clarification{
		Question: "你能提供更多细节吗？",
		Context:  fmt.Sprintf("对意图 '%s' 的置信度较低 (%.0f%%)", intent.Intent, intent.Confidence*100),
		Priority: 2,
	}
}

func (u *Understander) suggestRoute(
	intent *agent.IntentResult,
	entities Entities,
	ambiguity AmbiguityStatus,
) RoutePreference {
	pref := RoutePreference{
		AutoProceed: ambiguity < AmbiguityMedium,
	}

	// Mode selection based on intent
	switch intent.Intent {
	case agent.IntentCodeEdit, agent.IntentDebug, agent.IntentRefactor:
		pref.Mode = "build"
		if len(entities.Files) > 5 {
			pref.Complexity = "complex"
		} else if len(entities.Files) > 2 {
			pref.Complexity = "moderate"
		} else {
			pref.Complexity = "simple"
		}
	case agent.IntentPlan, agent.IntentDoc:
		pref.Mode = "plan"
		pref.Complexity = "moderate"
	case agent.IntentChat, agent.IntentCodeExplain:
		pref.Mode = "chat"
		pref.Complexity = "simple"
	case agent.IntentSearch, agent.IntentReview:
		if len(entities.Files) > 0 {
			pref.Mode = "plan"
		} else {
			pref.Mode = "chat"
		}
		pref.Complexity = "simple"
	default:
		pref.Mode = "build"
		pref.Complexity = "simple"
	}

	// Don't auto-proceed if high ambiguity
	if ambiguity >= AmbiguityMedium {
		pref.AutoProceed = false
	}

	return pref
}

func (u *Understander) buildMetadata(message string, entities Entities) AnalysisMetadata {
	wordCount := len(strings.Fields(message))
	msgLower := strings.ToLower(message)

	actionVerbs := []string{"add", "fix", "create", "remove", "implement",
		"修改", "添加", "删除", "修复", "实现", "创建"}
	hasAction := false
	for _, v := range actionVerbs {
		if strings.Contains(msgLower, v) {
			hasAction = true
			break
		}
	}

	return AnalysisMetadata{
		MessageLength: len([]rune(message)),
		WordCount:     wordCount,
		HasCodeBlock:  strings.Contains(message, "```"),
		HasFileRef:    len(entities.Files) > 0,
		HasActionVerb: hasAction,
	}
}

// --- Helper functions ---

// isKnownExt returns true if the extension is a recognizable source file.
func isKnownExt(ext string) bool {
	known := []string{".go", ".js", ".ts", ".jsx", ".tsx", ".py", ".rs", ".java",
		".c", ".cpp", ".h", ".hpp", ".css", ".html", ".json", ".yaml", ".yml",
		".md", ".sh", ".bash", ".sql", ".xml", ".vue", ".svelte"}
	for _, k := range known {
		if ext == k {
			return true
		}
	}
	return false
}

// isCommonWord returns true if the word is too common to be a function name.
func isCommonWord(word string) bool {
	common := map[string]bool{
		"the": true, "a": true, "an": true, "is": true, "are": true, "was": true,
		"if": true, "for": true, "in": true, "to": true, "of": true, "and": true,
		"or": true, "not": true, "with": true, "on": true, "at": true, "by": true,
		"from": true, "as": true, "this": true, "that": true, "it": true, "be": true,
		"has": true, "have": true, "do": true, "does": true, "will": true, "can": true,
		"should": true, "would": true, "could": true, "what": true, "which": true,
		"how": true, "why": true, "when": true, "where": true, "who": true,
		"print": true, "fmt": true, "log": true, "return": true, "func": true,
		"var": true, "const": true, "type": true, "struct": true, "interface": true,
		"package": true, "import": true, "new": true, "make": true, "len": true,
		"cap": true, "append": true, "range": true, "nil": true, "true": true, "false": true,
	}
	return common[strings.ToLower(word)]
}

// IsChineseMessage returns true if the message is primarily Chinese.
func IsChineseMessage(message string) bool {
	chineseCount := 0
	totalCount := 0
	for _, r := range message {
		if unicode.IsLetter(r) {
			totalCount++
			if r >= 0x4E00 && r <= 0x9FFF {
				chineseCount++
			}
		}
	}
	if totalCount == 0 {
		return false
	}
	return float64(chineseCount)/float64(totalCount) > 0.3
}

// --- Clarification formatting ---

// FormatForFrontend formats a Clarification as a user-facing message.
func (c *Clarification) FormatForFrontend() string {
	var sb strings.Builder
	sb.WriteString(c.Question)
	if len(c.Options) > 0 {
		sb.WriteString("\n\n选项:\n")
		for i, opt := range c.Options {
			sb.WriteString(fmt.Sprintf("  %d. %s\n", i+1, opt))
		}
	}
	if c.Context != "" {
		sb.WriteString(fmt.Sprintf("\n(%s)", c.Context))
	}
	return sb.String()
}

// ToWailsEvent converts the Understanding to a (event, data) pair.
func (u *Understanding) ToWailsEvent() (string, map[string]any) {
	return "ai:understander:result", map[string]any{
		"intent":      string(u.Intent),
		"confidence":  u.Confidence,
		"ambiguity":   u.Ambiguity.String(),
		"language":    u.Language,
		"files":       u.Entities.Files,
		"functions":   u.Entities.Functions,
		"autoProceed": u.RouteHint.AutoProceed,
		"mode":        u.RouteHint.Mode,
	}
}
