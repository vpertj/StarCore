package context

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"StarCore/internal/memory"
	"StarCore/internal/provider"
)

const (
	maxContextFileSize = 50000
	maxAnalysisChars   = 5000
	structureCacheTTL  = 30 * time.Second
)

type structureCacheEntry struct {
	content  string
	cachedAt time.Time
}

// Builder builds context messages for AI chat requests and manages compression.
type RAGSearchFunc func(ctx context.Context, projectPath string, query string, topK int) ([]RAGResult, error)

type RAGResult struct {
	Content string
	Score   float64
	Path    string
}

type Builder struct {
	providerMgr    *provider.Manager
	memoryStore    *memory.Store
	structureMu    sync.RWMutex
	structureCache map[string]structureCacheEntry
	ragSearch      RAGSearchFunc
}

func NewBuilder(providerMgr *provider.Manager, memoryStore *memory.Store) *Builder {
	return &Builder{
		providerMgr:    providerMgr,
		memoryStore:    memoryStore,
		structureCache: make(map[string]structureCacheEntry),
	}
}

func (b *Builder) SetRAGSearch(fn RAGSearchFunc) {
	b.ragSearch = fn
}

// BuildContextMessage constructs a context message from the chat request.
// BuildContextMessage builds the context message with a stable prefix order for maximum cache hits.
// Order: Rules → Structure → Knowledge → RAG → ContextFiles → ActiveFile → SelectedCode
// The stable prefix (Rules + Structure) is the same across requests, maximizing OpenAI/Anthropic cache hits.
func (b *Builder) BuildContextMessage(req provider.ChatRequest) string {
	var stableParts []string    // Stable prefix — same across requests (cacheable)
	var dynamicParts []string   // Dynamic suffix — varies per request

	// Dedup: if active file is already in context files, skip it
	if req.ActiveFile != "" && len(req.ContextFiles) > 0 {
		for _, fp := range req.ContextFiles {
			if fp == req.ActiveFile {
				req.ActiveFile = ""
				req.ActiveFileContent = ""
				break
			}
		}
	}

	// === STABLE PREFIX (cacheable) ===

	// 0. Git Context + Code Structure (branch, recent changes, code analysis)
	if req.ProjectPath != "" {
		gitCtx := b.getGitContext(req.ProjectPath)
		if gitCtx != "" {
			stableParts = append(stableParts, gitCtx)
		}
		// Add code structure summary for better code understanding
		structures := AnalyzeProjectStructure(req.ProjectPath, 50)
		if len(structures) > 0 {
			summary := GetCodeSummary(structures)
			if summary != "" {
				stableParts = append(stableParts, "[Code Structure]\n"+summary)
			}
		}
	}

	// 1. Project Rules (most stable — rarely changes)
	if req.ProjectPath != "" {
		rules := loadProjectRules(req.ProjectPath)
		if rules != "" {
			stableParts = append(stableParts, "[Project Rules]\nThe following project-specific rules apply. Follow these above all other instructions:\n\n"+rules)
		}
	}

	// 2. Project Structure (stable — changes rarely)
	if req.ProjectPath != "" {
		structure := b.getProjectStructure(req.ProjectPath)
		if structure != "" {
			stableParts = append(stableParts, "[Project Structure]\nProject root: "+req.ProjectPath+"\n"+structure)
		}
	}

	// 3. Knowledge Base (relatively stable)
	if req.ProjectPath != "" && b.memoryStore != nil {
		lastUserMsg := ""
		for i := len(req.Messages) - 1; i >= 0; i-- {
			if req.Messages[i].Role == "user" {
				lastUserMsg = req.Messages[i].Content
				break
			}
		}
		knowledge := b.injectKnowledge(req.ProjectPath, lastUserMsg)
		if knowledge != "" {
			stableParts = append(stableParts, knowledge)
		}

		// 4. RAG Results (semi-stable)
		if b.ragSearch != nil && lastUserMsg != "" {
			results, err := b.ragSearch(context.Background(), req.ProjectPath, lastUserMsg, 3)
			if err == nil && len(results) > 0 {
				var ragParts strings.Builder
				ragParts.WriteString("[Relevant Code Context]\nThe following code snippets are semantically relevant to your query:\n\n")
				for i, r := range results {
					if i >= 3 {
						break
					}
					pathInfo := ""
					if r.Path != "" {
						pathInfo = fmt.Sprintf(" (from %s, score: %.2f)", r.Path, r.Score)
					}
					content := r.Content
					if len(content) > 2000 {
						content = content[:2000] + "\n..."
					}
					ragParts.WriteString(fmt.Sprintf("---%s\n%s\n\n", pathInfo, content))
				}
				stableParts = append(stableParts, ragParts.String())
			}
		}
	}

	// === DYNAMIC SUFFIX (varies per request) ===

	// 5. Context Files (user-selected, varies)
	if len(req.ContextFiles) > 0 {
		var fileParts []string
		for _, fp := range req.ContextFiles {
			data, err := os.ReadFile(fp)
			if err != nil {
				fileParts = append(fileParts, fmt.Sprintf("--- File: %s ---\n[Error reading file: %v]", fp, err))
				continue
			}
			content := string(data)
			if len(content) > maxContextFileSize {
				content = smartTruncate(content, maxContextFileSize)
			}
			fileParts = append(fileParts, fmt.Sprintf("--- File: %s ---\n%s", fp, content))
		}
		if len(fileParts) > 0 {
			dynamicParts = append(dynamicParts, "[Context Files]\nThe following files were referenced by the user:\n\n"+strings.Join(fileParts, "\n\n"))
		}
	}

	// 6. Active File (changes frequently)
	if req.ActiveFile != "" && req.ActiveFileContent != "" {
		content := req.ActiveFileContent
		if len(content) > maxContextFileSize {
			content = smartTruncate(content, maxContextFileSize)
		}
		dynamicParts = append(dynamicParts, "[Currently Open File]\nFile: "+req.ActiveFile+"\n\n"+content)
	}

	// 7. Selected Code (most volatile)
	if req.SelectedCode != "" {
		dynamicParts = append(dynamicParts, "[Selected Code]\nThe user has selected the following code:\n\n"+req.SelectedCode)
	}

	// Combine: stable prefix first, then dynamic suffix
	allParts := append(stableParts, dynamicParts...)

	if len(allParts) == 0 {
		return ""
	}

	// Smart trimming: if total context is too large, trim from least important (dynamic suffix)
	totalLen := 0
	for _, p := range allParts {
		totalLen += len(p)
	}

	// Max context size: ~100K chars (~33K tokens)
	const maxContextChars = 100000
	if totalLen > maxContextChars {
		// Keep stable prefix intact, trim dynamic parts
		stableLen := 0
		for _, p := range stableParts {
			stableLen += len(p)
		}
		remaining := maxContextChars - stableLen
		if remaining < 0 {
			remaining = 0
		}

		// Trim dynamic parts from least important (SelectedCode first, then ActiveFile, then ContextFiles)
		trimmed := make([]string, 0, len(dynamicParts))
		for i := len(dynamicParts) - 1; i >= 0; i-- {
			part := dynamicParts[i]
			if len(part) <= remaining {
				trimmed = append([]string{part}, trimmed...)
				remaining -= len(part)
			} else if remaining > 100 {
				// Truncate this part
				trimmed = append([]string{part[:remaining] + "\n... [truncated]"}, trimmed...)
				remaining = 0
			}
			// If remaining <= 100, skip this part entirely
		}

		allParts = append(stableParts, trimmed...)
	}

	return strings.Join(allParts, "\n\n") + "\n\n[End of context. Use the above information to handle the user's request below.]"
}

// GetModelContextWindow estimates the context window size for a model.
func (b *Builder) GetModelContextWindow(_, modelID string) int {
	if w := provider.EstimateContextWindow(modelID); w > 0 {
		return w
	}
	return 128000
}

// SummarizeAndCompressWithFlag compresses messages and returns whether summarization occurred.
// If conversationID is provided, the summary is persisted to the conversation.
func (b *Builder) SummarizeAndCompressWithFlag(messages []provider.Message, maxTokens int, providerID string, conversationID string) ([]provider.Message, bool) {
	original := len(messages)
	result := b.summarizeAndCompress(messages, maxTokens, providerID)
	didSummarize := len(result) < original && original > 8

	// Persist summary to conversation if we summarized and have a conversation ID
	if didSummarize && conversationID != "" && b.memoryStore != nil {
		for _, msg := range result {
			if msg.Role == "system" && strings.HasPrefix(msg.Content, "[对话历史摘要]") {
				summary := strings.TrimPrefix(msg.Content, "[对话历史摘要]\n")
				summary = strings.Split(summary, "\n[请基于")[0]
				if err := b.memoryStore.UpdateConversationSummary(conversationID, summary); err != nil {
					log.Printf("Failed to persist conversation summary: %v", err)
				}
				break
			}
		}
	}

	return result, didSummarize
}

// AnalyzeProject analyzes a project directory using AI.
func (b *Builder) AnalyzeProject(projectPath string) (string, error) {
	if projectPath == "" {
		projectPath = "."
	}
	structure := b.getProjectStructure(projectPath)
	if structure == "" {
		return "Project is empty or could not be read.", nil
	}

	keyFiles := detectKeyFiles(projectPath)
	var fileContents []string
	for _, name := range keyFiles {
		path := filepath.Join(projectPath, name)
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		content := string(data)
		if len(content) > 3000 {
			content = content[:maxAnalysisChars] + "\n... [truncated]"
		}
		fileContents = append(fileContents, fmt.Sprintf("--- %s ---\n%s", name, content))
	}

	analysisPrompt := "你是一个资深软件工程师。分析以下项目结构和关键文件。输出：\n1. 项目类型和用途\n2. 技术栈摘要\n3. 架构概览\n4. 关键文件及其作用\n5. 开发建议\n\n简洁且具体。"

	messages := []provider.Message{
		{Role: "system", Content: analysisPrompt},
		{Role: "user", Content: fmt.Sprintf("Project path: %s\n\nProject Structure:\n%s\n\nKey Files:\n%s", projectPath, structure, strings.Join(fileContents, "\n\n"))},
	}

	resp, err := b.providerMgr.Chat(context.Background(), provider.ChatRequest{
		Messages:  messages,
		MaxTokens: 2000,
	})
	if err != nil {
		return "", fmt.Errorf("analysis failed: %w", err)
	}

	analysis := resp.Content
	if b.memoryStore != nil {
		b.memoryStore.SaveKnowledge(&memory.Knowledge{
			ID:          fmt.Sprintf("project_analysis_%d", time.Now().UnixNano()),
			ProjectPath: projectPath,
			Category:    "analysis",
			Key:         "project_analysis",
			Value:       analysis,
			Source:      "AI",
			UpdatedAt:   time.Now().Format(time.RFC3339),
		})
	}
	return analysis, nil
}

// GetProjectAnalysis retrieves a cached project analysis.
func (b *Builder) GetProjectAnalysis(projectPath string) (string, error) {
	if b.memoryStore == nil {
		return "", nil
	}
	entries, err := b.memoryStore.GetKnowledgeByCategory(projectPath, "analysis")
	if err != nil {
		return "", err
	}
	for _, entry := range entries {
		if entry.Key == "project_analysis" {
			return entry.Value, nil
		}
	}
	return "", nil
}

// --- unexported helpers ---

func (b *Builder) getProjectStructure(projectPath string) string {
	if b.structureCache != nil {
		b.structureMu.RLock()
		if entry, ok := b.structureCache[projectPath]; ok && time.Since(entry.cachedAt) < structureCacheTTL {
			b.structureMu.RUnlock()
			return entry.content
		}
		b.structureMu.RUnlock()
	}

	content := b.scanProjectStructure(projectPath)

	if b.structureCache != nil {
		b.structureMu.Lock()
		b.structureCache[projectPath] = structureCacheEntry{content: content, cachedAt: time.Now()}
		b.structureMu.Unlock()
	}

	return content
}

// languagePriority returns a priority score for file extensions.
// Higher score = more important for context (entry points, configs, core files).
func languagePriority(ext string) int {
	switch ext {
	case ".go", ".py", ".rs", ".java", ".ts", ".tsx", ".jsx":
		return 3 // Source code
	case ".js", ".vue", ".svelte":
		return 3
	case ".json", ".yaml", ".yml", ".toml", ".ini", ".env":
		return 2 // Config files
	case ".md", ".txt", ".rst":
		return 1 // Documentation
	case ".test.js", ".test.ts", "_test.go", ".spec.js":
		return 2 // Tests are important
	default:
		return 0
	}
}

func (b *Builder) scanProjectStructure(projectPath string) string {
	var lines []string
	dirCount := 0
	fileCount := 0
	totalItems := 0
	const maxDirs = 25
	const maxFiles = 40
	const maxDepth = 3
	const maxTotal = 80

	var scanDir func(path string, depth int, prefix string)
	scanDir = func(path string, depth int, prefix string) {
		if depth > maxDepth || totalItems >= maxTotal {
			return
		}

		entries, err := os.ReadDir(path)
		if err != nil {
			return
		}

		// Sort entries: dirs first, then files by priority
		type entryInfo struct {
			entry    os.DirEntry
			priority int
		}
		var dirs []entryInfo
		var files []entryInfo

		for _, entry := range entries {
			name := entry.Name()
			if strings.HasPrefix(name, ".") && name != ".gitignore" {
				continue
			}
			// Skip common non-essential directories
			if entry.IsDir() && (name == "node_modules" || name == "vendor" || name == "__pycache__" ||
				name == ".git" || name == "dist" || name == "build" || name == "target" || name == ".cache") {
				continue
			}
			if entry.IsDir() {
				dirs = append(dirs, entryInfo{entry: entry, priority: 0})
			} else {
				ext := filepath.Ext(name)
				priority := languagePriority(ext)
				files = append(files, entryInfo{entry: entry, priority: priority})
			}
		}

		// Process directories
		for _, ei := range dirs {
			if dirCount >= maxDirs {
				lines = append(lines, prefix+"  ... (more directories)")
				return
			}
			subPath := filepath.Join(path, ei.entry.Name())
			subEntries, err := os.ReadDir(subPath)
			subCount := 0
			if err == nil {
				subCount = len(subEntries)
			}
			lines = append(lines, fmt.Sprintf("%s📁 %s/ (%d items)", prefix, ei.entry.Name(), subCount))
			dirCount++
			totalItems++
			scanDir(subPath, depth+1, prefix+"  ")
		}

		// Process files (sorted by priority, higher first)
		for i := 0; i < len(files) && fileCount < maxFiles && totalItems < maxTotal; i++ {
			ei := files[i]
			info, err := ei.entry.Info()
			var size int64
			if err == nil {
				size = info.Size()
			}
			sizeStr := ""
			if size > 1024*1024 {
				sizeStr = fmt.Sprintf(" (%.1fMB)", float64(size)/1024/1024)
			} else if size > 1024 {
				sizeStr = fmt.Sprintf(" (%dKB)", size/1024)
			}
			lines = append(lines, fmt.Sprintf("%s📄 %s%s", prefix, ei.entry.Name(), sizeStr))
			fileCount++
			totalItems++
		}
	}

	scanDir(projectPath, 0, "")
	if len(lines) == 0 {
		return ""
	}
	return strings.Join(lines, "\n")
}

func estimateTokens(text string) int {
	if len(text) == 0 {
		return 0
	}
	cjk := 0
	asciiWords := 0
	inWord := false
	for _, r := range text {
		if r >= 0x4E00 && r <= 0x9FFF || r >= 0x3400 && r <= 0x4DBF ||
			r >= 0x3000 && r <= 0x303F || r >= 0xFF00 && r <= 0xFFEF ||
			r >= 0x3040 && r <= 0x309F || r >= 0x30A0 && r <= 0x30FF ||
			r >= 0xAC00 && r <= 0xD7AF {
			cjk++
			inWord = false
		} else if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '_' {
			if !inWord {
				asciiWords++
				inWord = true
			}
		} else {
			inWord = false
		}
	}
	nonWordChars := 0
	for _, r := range text {
		if !((r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '_') &&
			!(r >= 0x4E00 && r <= 0x9FFF || r >= 0x3400 && r <= 0x4DBF ||
				r >= 0x3000 && r <= 0x303F || r >= 0xFF00 && r <= 0xFFEF ||
				r >= 0x3040 && r <= 0x309F || r >= 0x30A0 && r <= 0x30FF ||
				r >= 0xAC00 && r <= 0xD7AF) {
			nonWordChars++
		}
	}
	return int(float64(cjk)*1.5+float64(asciiWords)*1.3+float64(nonWordChars)*0.4) + 1
}

func (b *Builder) summarizeAndCompress(messages []provider.Message, maxTokens int, providerID string) []provider.Message {
	totalTokens := 0
	for _, msg := range messages {
		totalTokens += estimateTokens(msg.Content)
	}
	if totalTokens <= maxTokens {
		return messages
	}

	systemMsgs := []provider.Message{}
	otherMsgs := []provider.Message{}
	for _, msg := range messages {
		if msg.Role == "system" || msg.Role == "tool" {
			systemMsgs = append(systemMsgs, msg)
		} else {
			otherMsgs = append(otherMsgs, msg)
		}
	}

	systemTokens := 0
	for _, m := range systemMsgs {
		systemTokens += estimateTokens(m.Content)
	}

	availableForChat := maxTokens - systemTokens
	if availableForChat < 2000 {
		availableForChat = 2000
	}

	// Token budget: 60% recent messages, 40% summary of older messages
	recentBudget := int(float64(availableForChat) * 0.6)
	summaryBudget := availableForChat - recentBudget

	// Calculate how many recent messages fit within recentBudget
	recentStart := len(otherMsgs)
	recentTokens := 0
	for i := len(otherMsgs) - 1; i >= 0; i-- {
		msgTokens := estimateTokens(otherMsgs[i].Content)
		if recentTokens+msgTokens > recentBudget {
			break
		}
		recentTokens += msgTokens
		recentStart = i
	}

	if recentStart <= 0 {
		result := make([]provider.Message, 0, len(systemMsgs)+len(otherMsgs))
		result = append(result, systemMsgs...)
		result = append(result, otherMsgs...)
		return result
	}

	oldMsgs := otherMsgs[:recentStart]
	recentMsgs := otherMsgs[recentStart:]

	summary := b.generateSummaryWithBudget(oldMsgs, providerID, summaryBudget)

	result := make([]provider.Message, 0, len(systemMsgs)+len(recentMsgs)+1)
	result = append(result, systemMsgs...)

	if summary != "" {
		result = append(result, provider.Message{
			Role:    "system",
			Content: "[对话历史摘要]\n" + summary + "\n[请基于以上上下文和下方最近的对话继续。]",
		})
	} else if len(oldMsgs) > 0 {
		result = append(result, provider.Message{
			Role:    "system",
			Content: fmt.Sprintf("[为节省上下文，省略了 %d 条更早的消息]", len(oldMsgs)),
		})
	}

	result = append(result, recentMsgs...)
	return result
}

func (b *Builder) generateSummary(messages []provider.Message, providerID string) string {
	return b.generateSummaryWithBudget(messages, providerID, 3000)
}

func (b *Builder) generateSummaryWithBudget(messages []provider.Message, providerID string, maxSummaryTokens int) string {
	if len(messages) == 0 || providerID == "" {
		return ""
	}

	if maxSummaryTokens < 500 {
		maxSummaryTokens = 500
	}

	var conversation strings.Builder
	maxContentLen := 2000
	if maxSummaryTokens < 1500 {
		maxContentLen = 800
	}
	for i, msg := range messages {
		content := msg.Content
		if len(content) > maxContentLen {
			content = content[:maxContentLen] + "..."
		}
		conversation.WriteString(fmt.Sprintf("[%s #%d]: %s\n", msg.Role, i+1, content))
	}

	prompt := fmt.Sprintf(`总结以下对话片段。必须保留全部内容：
1. 用户的请求、需求和偏好
2. 关键技术决策及其依据
3. 已完成的代码变更（哪些文件、改了什么、为什么）
4. 发现的 bug、根因及修复方法
5. 讨论过的重要文件路径和代码模式
6. 任何约束条件或特别注意事项

简洁但完整。此摘要将替换原始消息。目标token数约%d。

对话内容：
%s`, maxSummaryTokens, conversation.String())

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	resp, err := b.providerMgr.Chat(ctx, provider.ChatRequest{
		ProviderID:  providerID,
		Messages:    []provider.Message{{Role: "user", Content: prompt}},
		Temperature: 0.1,
		MaxTokens:   maxSummaryTokens,
		Stream:      false,
	})

	if err != nil {
		log.Printf("AI summarization failed: %v", err)
		return ""
	}

	return resp.Content
}

// injectKnowledge retrieves relevant knowledge entries and formats them for context injection.
// It uses the user's message as a hint to select the most relevant entries.
func (b *Builder) injectKnowledge(projectPath, userMessage string) string {
	entries, err := b.memoryStore.GetKnowledge(projectPath)
	if err != nil || len(entries) == 0 {
		return ""
	}

	maxKnowledgeEntries := 5
	maxKnowledgeChars := 4000

	var selected []memory.Knowledge

	// Try semantic search first (RAG-based)
	if b.ragSearch != nil && userMessage != "" {
		results, ragErr := b.ragSearch(context.Background(), projectPath, userMessage, 5)
		if ragErr == nil && len(results) > 0 {
			// Map RAG results back to knowledge entries by path
			for _, r := range results {
				if len(selected) >= maxKnowledgeEntries {
					break
				}
				for _, entry := range entries {
					if entry.Key == r.Path || strings.Contains(entry.Value, r.Path) {
						selected = append(selected, entry)
						break
					}
				}
			}
		}
	}

	// Fall back to keyword matching if RAG didn't find enough
	if len(selected) < maxKnowledgeEntries {
		keywords := extractKeywords(userMessage)
		for _, entry := range entries {
			if len(selected) >= maxKnowledgeEntries {
				break
			}
			// Skip if already selected
			alreadySelected := false
			for _, s := range selected {
				if s.ID == entry.ID {
					alreadySelected = true
					break
				}
			}
			if alreadySelected {
				continue
			}
			if entry.Category == "analysis" || entry.Category == "preference" || entry.Category == "pattern" {
				selected = append(selected, entry)
				continue
			}
			if len(keywords) > 0 && containsAnyKeyword(entry.Value, keywords) {
				selected = append(selected, entry)
			}
		}
	}

	if len(selected) == 0 {
		return ""
	}

	var buf strings.Builder
	buf.WriteString("[Knowledge Base]\nThe following knowledge entries from previous sessions are relevant:\n\n")
	totalChars := 0
	for _, entry := range selected {
		value := entry.Value
		if len(value) > 1500 {
			value = value[:1500] + "..."
		}
		if totalChars+len(value) > maxKnowledgeChars {
			break
		}
		buf.WriteString(fmt.Sprintf("--- [%s] %s (source: %s, updated: %s) ---\n%s\n\n",
			entry.Category, entry.Key, entry.Source, entry.UpdatedAt, value))
		totalChars += len(value)
	}
	return buf.String()
}

func extractKeywords(text string) []string {
	if len(text) == 0 {
		return nil
	}
	words := strings.Fields(strings.ToLower(text))
	keywords := make([]string, 0, len(words))
	for _, w := range words {
		if len(w) >= 3 {
			keywords = append(keywords, w)
		}
	}
	if len(keywords) > 10 {
		keywords = keywords[:10]
	}
	return keywords
}

func containsAnyKeyword(text string, keywords []string) bool {
	lower := strings.ToLower(text)
	for _, kw := range keywords {
		if strings.Contains(lower, kw) {
			return true
		}
	}
	return false
}

// loadProjectRules reads a .starcorerules file from the project root.
// This works like Claude Code's CLAUDE.md or Cursor's .cursorrules.
func loadProjectRules(projectPath string) string {
	paths := []string{
		filepath.Join(projectPath, ".starcorerules"),
		filepath.Join(projectPath, ".cursorrules"),
		filepath.Join(projectPath, "CLAUDE.md"),
	}
	for _, p := range paths {
		data, err := os.ReadFile(p)
		if err == nil {
			content := string(data)
			if len(content) > 4000 {
				content = content[:4000] + "\n... [truncated]"
			}
			return content
		}
	}
	return ""
}

func detectKeyFiles(projectPath string) []string {
	entries, err := os.ReadDir(projectPath)
	if err != nil {
		return nil
	}

	ecosystems := map[string][]string{
		"go":     {"go.mod", "go.sum", "main.go", "go.work"},
		"node":   {"package.json", "package-lock.json", "yarn.lock", "pnpm-lock.yaml", "index.js", "app.js"},
		"python": {"requirements.txt", "setup.py", "pyproject.toml", "setup.cfg", "Pipfile", "main.py"},
		"rust":   {"Cargo.toml", "Cargo.lock", "main.rs"},
		"java":   {"pom.xml", "build.gradle", "settings.gradle", "build.gradle.kts"},
		"dotnet": {"*.csproj", "*.sln", "Program.cs"},
		"ruby":   {"Gemfile", "Rakefile"},
		"php":    {"composer.json", "index.php"},
		"web":    {"index.html", "vite.config.js", "webpack.config.js", "next.config.js"},
	}
	universal := []string{"README.md", "README", "Makefile", "docker-compose.yml", "Dockerfile", ".gitignore", ".env.example"}

	seen := make(map[string]bool)
	var found []string

	for _, name := range universal {
		path := filepath.Join(projectPath, name)
		if _, err := os.Stat(path); err == nil && !seen[name] {
			found = append(found, name)
			seen[name] = true
		}
	}

	for _, names := range ecosystems {
		for _, name := range names {
			path := filepath.Join(projectPath, name)
			if _, err := os.Stat(path); err == nil && !seen[name] {
				found = append(found, name)
				seen[name] = true
			}
		}
	}

	for _, e := range entries {
		name := e.Name()
		if seen[name] || e.IsDir() {
			continue
		}
		if name == "main.go" || name == "main.py" || name == "index.js" ||
			name == "app.js" || name == "server.js" || name == "main.rs" {
			found = append(found, name)
			seen[name] = true
		}
	}

	return found
}

// smartTruncate keeps the head (imports/declarations) and tail (closing code) of a file.
// For code files, this preserves structure better than naive truncation.
func smartTruncate(content string, maxLen int) string {
	if len(content) <= maxLen {
		return content
	}

	headRatio := 0.75
	tailRatio := 0.25
	headLen := int(float64(maxLen) * headRatio)
	tailLen := int(float64(maxLen) * tailRatio)

	head := content[:headLen]
	tail := content[len(content)-tailLen:]

	// Find a clean line boundary for head
	if idx := strings.LastIndex(head, "\n"); idx > headLen/2 {
		head = head[:idx+1]
	}
	// Find a clean line boundary for tail
	if idx := strings.Index(tail, "\n"); idx >= 0 {
		tail = tail[idx+1:]
	}

	omittedLines := strings.Count(content, "\n") - strings.Count(head, "\n") - strings.Count(tail, "\n")
	return head + fmt.Sprintf("\n... [omitted %d lines] ...\n", omittedLines) + tail
}

// getGitContext retrieves git context information for the project.
// Returns branch name, recent commits, and recent diff summary.
func (b *Builder) getGitContext(projectPath string) string {
	var parts []string

	// Get current branch
	if branch := execGit(projectPath, "branch", "--show-current"); branch != "" {
		parts = append(parts, "[Git Context]\nCurrent branch: "+branch)
	}

	// Get recent commits (last 5)
	if log := execGit(projectPath, "log", "--oneline", "-5"); log != "" {
		parts = append(parts, "Recent commits:\n"+log)
	}

	// Get recent diff summary (unstaged changes)
	if diff := execGit(projectPath, "diff", "--stat"); diff != "" {
		parts = append(parts, "Unstaged changes:\n"+diff)
	}

	// Get staged changes summary
	if diff := execGit(projectPath, "diff", "--cached", "--stat"); diff != "" {
		parts = append(parts, "Staged changes:\n"+diff)
	}

	if len(parts) == 0 {
		return ""
	}
	return strings.Join(parts, "\n") + "\n\n"
}

// execGit runs a git command and returns the output.
func execGit(projectPath string, args ...string) string {
	cmd := exec.Command("git", args...)
	cmd.Dir = projectPath
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}
