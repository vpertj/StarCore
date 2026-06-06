package context

import (
	"context"
	"fmt"
	"io/ioutil"
	"log"
	"os"
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
type Builder struct {
	providerMgr    *provider.Manager
	memoryStore    *memory.Store
	structureMu    sync.RWMutex
	structureCache map[string]structureCacheEntry
}

// NewBuilder creates a new context builder.
func NewBuilder(providerMgr *provider.Manager, memoryStore *memory.Store) *Builder {
	return &Builder{
		providerMgr:    providerMgr,
		memoryStore:    memoryStore,
		structureCache: make(map[string]structureCacheEntry),
	}
}

// BuildContextMessage constructs a context message from the chat request.
func (b *Builder) BuildContextMessage(req provider.ChatRequest) string {
	var parts []string

	if req.ProjectPath != "" {
		structure := b.getProjectStructure(req.ProjectPath)
		if structure != "" {
			parts = append(parts, "[Project Structure]\nProject root: "+req.ProjectPath+"\n"+structure)
		}
	}

	if len(req.ContextFiles) > 0 {
		var fileParts []string
		for _, fp := range req.ContextFiles {
			data, err := ioutil.ReadFile(fp)
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
			parts = append(parts, "[Context Files]\nThe following files were referenced by the user:\n\n"+strings.Join(fileParts, "\n\n"))
		}
	}

	if req.ActiveFile != "" && req.ActiveFileContent != "" {
		content := req.ActiveFileContent
		if len(content) > maxContextFileSize {
			content = smartTruncate(content, maxContextFileSize)
		}
		parts = append(parts, "[Currently Open File]\nFile: "+req.ActiveFile+"\n\n"+content)
	}

	if req.SelectedCode != "" {
		parts = append(parts, "[Selected Code]\nThe user has selected the following code:\n\n"+req.SelectedCode)
	}

	if req.ContextCode != "" {
		parts = append(parts, "[Context Code]\n"+req.ContextCode)
	}

	// Inject project rules (.starcorerules, .cursorrules, CLAUDE.md) — like Cursor/Claude Code
	if req.ProjectPath != "" {
		rules := loadProjectRules(req.ProjectPath)
		if rules != "" {
			parts = append(parts, "[Project Rules]\nThe following project-specific rules apply. Follow these above all other instructions:\n\n"+rules)
		}
	}

	// Auto-inject knowledge base entries for the project
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
			parts = append(parts, knowledge)
		}
	}

	if len(parts) == 0 {
		return ""
	}

	return strings.Join(parts, "\n\n") + "\n\n[End of context. Use the above information to handle the user's request below.]"
}

// GetModelContextWindow estimates the context window size for a model.
func (b *Builder) GetModelContextWindow(_, modelID string) int {
	if w := provider.EstimateContextWindow(modelID); w > 0 {
		return w
	}
	return 128000
}

// SummarizeAndCompressWithFlag compresses messages and returns whether summarization occurred.
func (b *Builder) SummarizeAndCompressWithFlag(messages []provider.Message, maxTokens int, providerID string) ([]provider.Message, bool) {
	original := len(messages)
	result := b.summarizeAndCompress(messages, maxTokens, providerID)
	didSummarize := len(result) < original && original > 8
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
		data, err := ioutil.ReadFile(path)
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

func (b *Builder) scanProjectStructure(projectPath string) string {
	files, err := ioutil.ReadDir(projectPath)
	if err != nil {
		return ""
	}
	var lines []string
	dirCount := 0
	fileCount := 0
	for _, f := range files {
		if strings.HasPrefix(f.Name(), ".") && f.Name() != ".gitignore" {
			continue
		}
		if f.IsDir() {
			subFiles, err := ioutil.ReadDir(filepath.Join(projectPath, f.Name()))
			subCount := 0
			if err == nil {
				subCount = len(subFiles)
			}
			lines = append(lines, fmt.Sprintf("  📁 %s/ (%d items)", f.Name(), subCount))
			dirCount++
			if dirCount >= 20 {
				lines = append(lines, "  ... (more directories)")
				break
			}
		} else {
			size := f.Size()
			sizeStr := ""
			if size > 1024*1024 {
				sizeStr = fmt.Sprintf(" (%.1fMB)", float64(size)/1024/1024)
			} else if size > 1024 {
				sizeStr = fmt.Sprintf(" (%dKB)", size/1024)
			}
			lines = append(lines, fmt.Sprintf("  📄 %s%s", f.Name(), sizeStr))
			fileCount++
			if fileCount >= 30 {
				lines = append(lines, "  ... (more files)")
				break
			}
		}
	}
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
	keywords := extractKeywords(userMessage)

	for _, entry := range entries {
		if len(selected) >= maxKnowledgeEntries {
			break
		}
		if entry.Category == "analysis" || entry.Category == "preference" || entry.Category == "pattern" {
			selected = append(selected, entry)
			continue
		}
		if len(keywords) > 0 && containsAnyKeyword(entry.Value, keywords) {
			selected = append(selected, entry)
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
		data, err := ioutil.ReadFile(p)
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
	entries, err := ioutil.ReadDir(projectPath)
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
