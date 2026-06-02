package context

import (
	"context"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"StarCore/internal/memory"
	"StarCore/internal/provider"
)

const (
	maxContextFileSize = 50000
	maxAnalysisChars   = 5000
)

// Builder builds context messages for AI chat requests and manages compression.
type Builder struct {
	providerMgr *provider.Manager
	memoryStore *memory.Store
}

// NewBuilder creates a new context builder.
func NewBuilder(providerMgr *provider.Manager, memoryStore *memory.Store) *Builder {
	return &Builder{providerMgr: providerMgr, memoryStore: memoryStore}
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
				content = content[:maxContextFileSize] + "\n... [truncated]"
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
			content = content[:maxContextFileSize] + "\n... [truncated]"
		}
		parts = append(parts, "[Currently Open File]\nFile: "+req.ActiveFile+"\n\n"+content)
	}

	if req.SelectedCode != "" {
		parts = append(parts, "[Selected Code]\nThe user has selected the following code:\n\n"+req.SelectedCode)
	}

	if req.ContextCode != "" {
		parts = append(parts, "[Context Code]\n"+req.ContextCode)
	}

	if len(parts) == 0 {
		return ""
	}

	return strings.Join(parts, "\n\n") + "\n\n[End of context. Please use the above information to answer the user's question.]"
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
		Messages: messages,
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
	cjk := 0
	other := 0
	for _, r := range text {
		if r >= 0x4E00 && r <= 0x9FFF || r >= 0x3400 && r <= 0x4DBF ||
			r >= 0x3000 && r <= 0x303F || r >= 0xFF00 && r <= 0xFFEF ||
			r >= 0x3040 && r <= 0x309F || r >= 0x30A0 && r <= 0x30FF ||
			r >= 0xAC00 && r <= 0xD7AF {
			cjk++
		} else {
			other++
		}
	}
	return int(float64(cjk)*1.5 + float64(other)*0.25)
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

	keepCount := 8

	if len(otherMsgs) < 10 {
		start := 0
		if len(otherMsgs) > keepCount {
			start = len(otherMsgs) - keepCount
		}
		result := make([]provider.Message, 0, len(systemMsgs)+len(otherMsgs)-start)
		result = append(result, systemMsgs...)
		result = append(result, otherMsgs[start:]...)
		return result
	}
	if len(otherMsgs) <= keepCount {
		result := make([]provider.Message, 0, len(systemMsgs)+keepCount)
		result = append(result, systemMsgs...)
		result = append(result, otherMsgs[len(otherMsgs)-keepCount/2:]...)
		return result
	}

	oldMsgs := otherMsgs[:len(otherMsgs)-keepCount]
	recentMsgs := otherMsgs[len(otherMsgs)-keepCount:]

	summary := b.generateSummary(oldMsgs, providerID)

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
	if len(messages) == 0 || providerID == "" {
		return ""
	}

	var conversation strings.Builder
	for i, msg := range messages {
		content := msg.Content
		if len(content) > 2000 {
			content = content[:2000] + "..."
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

简洁但完整。此摘要将替换原始消息。

对话内容：
%s`, conversation.String())

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	resp, err := b.providerMgr.Chat(ctx, provider.ChatRequest{
		ProviderID:  providerID,
		Messages:    []provider.Message{{Role: "user", Content: prompt}},
		Temperature: 0.1,
		MaxTokens:   3000,
		Stream:      false,
	})

	if err != nil {
		log.Printf("AI summarization failed: %v", err)
		return ""
	}

	return resp.Content
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
