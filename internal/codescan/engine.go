package codescan

import (
	"regexp"
	"strings"
)

type Severity string

const (
	SeverityCritical Severity = "critical"
	SeverityHigh     Severity = "high"
	SeverityMedium   Severity = "medium"
	SeverityLow      Severity = "low"
	SeverityInfo     Severity = "info"
)

type Rule struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Category    string   `json:"category"`
	OWASP       string   `json:"owasp,omitempty"`
	CWE         string   `json:"cwe,omitempty"`
	Severity    Severity `json:"severity"`
	Description string   `json:"description"`
	Languages   []string `json:"languages"`
}

type Finding struct {
	RuleID     string   `json:"ruleId"`
	RuleName   string   `json:"ruleName"`
	File       string   `json:"file"`
	Line       int      `json:"line"`
	Column     int      `json:"column"`
	Severity   Severity `json:"severity"`
	Message    string   `json:"message"`
	OWASP      string   `json:"owasp,omitempty"`
	CWE        string   `json:"cwe,omitempty"`
	Category   string   `json:"category"`
	Suggestion string   `json:"suggestion,omitempty"`
}

type RuleEngine struct {
	rules []RuleDef
}

type RuleDef struct {
	Rule    Rule
	Pattern *regexp.Regexp
	Check   func(content string, file string) []Finding
}

func NewRuleEngine() *RuleEngine {
	re := &RuleEngine{}
	re.rules = buildRules()
	return re
}

func (re *RuleEngine) ScanFile(content string, filePath string, language string) []Finding {
	var findings []Finding
	for _, rd := range re.rules {
		if !matchLanguage(rd.Rule.Languages, language) {
			continue
		}
		if rd.Check != nil {
			findings = append(findings, rd.Check(content, filePath)...)
		} else if rd.Pattern != nil {
			findings = append(findings, patternCheck(rd, content, filePath)...)
		}
	}
	return findings
}

func (re *RuleEngine) ScanFiles(files map[string]string, language string) []Finding {
	var allFindings []Finding
	for filePath, content := range files {
		findings := re.ScanFile(content, filePath, language)
		allFindings = append(allFindings, findings...)
	}
	return allFindings
}

func (re *RuleEngine) ListRules() []Rule {
	rules := make([]Rule, len(re.rules))
	for i, rd := range re.rules {
		rules[i] = rd.Rule
	}
	return rules
}

func (re *RuleEngine) ListRulesByCategory(category string) []Rule {
	var rules []Rule
	for _, rd := range re.rules {
		if rd.Rule.Category == category {
			rules = append(rules, rd.Rule)
		}
	}
	return rules
}

func patternCheck(rd RuleDef, content string, filePath string) []Finding {
	var findings []Finding
	lines := strings.Split(content, "\n")
	for i, line := range lines {
		if rd.Pattern.MatchString(line) {
			findings = append(findings, Finding{
				RuleID:   rd.Rule.ID,
				RuleName: rd.Rule.Name,
				File:     filePath,
				Line:     i + 1,
				Column:   1,
				Severity: rd.Rule.Severity,
				Message:  rd.Rule.Description,
				OWASP:    rd.Rule.OWASP,
				CWE:      rd.Rule.CWE,
				Category: rd.Rule.Category,
			})
		}
	}
	return findings
}

func matchLanguage(languages []string, target string) bool {
	if len(languages) == 0 {
		return true
	}
	for _, l := range languages {
		if l == target || l == "*" {
			return true
		}
	}
	return false
}

func detectLanguageFromExt(ext string) string {
	switch strings.ToLower(ext) {
	case ".go":
		return "go"
	case ".js", ".jsx", ".mjs":
		return "javascript"
	case ".ts", ".tsx":
		return "typescript"
	case ".py":
		return "python"
	case ".java":
		return "java"
	case ".rs":
		return "rust"
	case ".rb":
		return "ruby"
	case ".php":
		return "php"
	case ".cs":
		return "csharp"
	case ".cpp", ".c", ".h":
		return "cpp"
	case ".sql":
		return "sql"
	default:
		return "unknown"
	}
}
