package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"StarCore/internal/agent"
	"StarCore/internal/agent/builtins"
	agentTools "StarCore/internal/agent/tools"
	"StarCore/internal/ai"
	ictx "StarCore/internal/context"
	"StarCore/internal/files"
	iggit "StarCore/internal/git"
	"StarCore/internal/lsp"
	"StarCore/internal/mcp"
	"StarCore/internal/memory"
	"StarCore/internal/provider"
	"StarCore/internal/skill"
	skillBuiltins "StarCore/internal/skill/builtins"
	"StarCore/internal/terminal"
	"StarCore/internal/watcher"

	wailsRuntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

// Type aliases — Wails sees main.X but underlying types are in internal packages.
type (
	FileInfo       = files.FileInfo
	DiffHunk       = files.DiffHunk
	SearchResult   = files.SearchResult
	SearchOptions  = files.SearchOptions
	GitStatusEntry = iggit.StatusEntry
	GitLogEntry    = iggit.LogEntry
)

// ApplyDiffRequest is the request payload for the ApplyDiff method.
type ApplyDiffRequest struct {
	FilePath string     `json:"filePath"`
	Hunks    []DiffHunk `json:"hunks"`
}

// CustomModelEntry stores a user-configured custom AI model.
type CustomModelEntry struct {
	ID           string `json:"id"`
	ModelID      string `json:"modelId"`
	Name         string `json:"name"`
	ProviderID   string `json:"providerId"`
	ProviderName string `json:"providerName"`
	APIKey       string `json:"apiKey"`
	Endpoint     string `json:"endpoint"`
	Enabled      bool   `json:"enabled"`
	MaxTokens    int    `json:"maxTokens"`
}

// configDir returns the StarCore configuration directory.
func configDir() string {
	d, err := os.UserConfigDir()
	if err != nil {
		d, _ = os.Getwd()
	}
	d = filepath.Join(d, "StarCore")
	os.MkdirAll(d, 0755)
	return d
}

// App is the main application struct. It is bound to the Wails frontend.
type App struct {
	ctx context.Context

	providerMgr *provider.Manager
	agentReg    *agent.Registry
	skillReg    *skill.Registry
	skillExec   *skill.Executor
	memoryStore *memory.Store
	toolExec    *agent.ToolExecutor
	mcpMgr      *mcp.ServerManager
	lspMgr      *lsp.Manager
	fileWatcher *watcher.Watcher

	aiService      *ai.Service
	contextBuilder *ictx.Builder
	terminalMgr    *terminal.Manager
	fileService    *files.Service
	gitService     *iggit.Service
}

// ------- Constructor -------

// NewApp creates and initializes the application with all services.
func NewApp() *App {
	dataDir := configDir()

	app := &App{}

	// --- Provider manager ---
	mgr := provider.NewManager(dataDir, func() context.Context { return app.ctx })
	mgr.Register(provider.NewOpenAIProvider())
	mgr.Register(provider.NewAnthropicProvider())
	mgr.Register(provider.NewOllamaProvider())
	mgr.LoadPersistedConfigs()

	// --- Agent registry ---
	reg := agent.NewRegistry()
	for _, a := range builtins.AllAgents() {
		reg.Register(a)
	}

	// --- Skill registry ---
	skillReg := skill.NewRegistry()
	for _, s := range skillBuiltins.AllSkills() {
		skillReg.Register(s)
	}
	skillsDir := skill.GetSkillsDir(dataDir)
	extSkills := skill.LoadSkillsFromDir(skillsDir)
	for _, s := range extSkills {
		skillReg.Register(s)
		log.Printf("Loaded external skill: %s", s.ID)
	}

	// --- Tool executor ---
	toolExec := agent.NewToolExecutor()
	for _, t := range agentTools.AllTools() {
		toolExec.Register(t)
	}

	skillExec := skill.NewExecutor(skillReg, mgr, toolExec)

	agentTools.SkillToolRegistry = skillReg
	agentTools.SkillToolExecutor = skillExec
	toolExec.Register(agentTools.NewSkillTool())
	toolExec.Register(agentTools.NewSubAgentTool())
	agentTools.SubAgentToolExec = toolExec
	agentTools.SubAgentProviderMgr = mgr

	// --- Memory store ---
	memStore, err := memory.NewStore(dataDir)
	if err != nil {
		log.Printf("Warning: failed to init memory store: %v", err)
	}

	// --- MCP ---
	mcpMgr := mcp.NewServerManager(toolExec, dataDir)
	mcpMgr.LoadConfig()

	// --- LSP ---
	lspMgr := lsp.NewManager()
	lspMgr.RegisterServer("go", "gopls", []string{}, []string{".go"})
	lspMgr.RegisterServer("javascript", "typescript-language-server", []string{"--stdio"}, []string{".js", ".jsx", ".mjs"})
	lspMgr.RegisterServer("typescript", "typescript-language-server", []string{"--stdio"}, []string{".ts", ".tsx"})
	lspMgr.RegisterServer("python", "pyright-langserver", []string{"--stdio"}, []string{".py"})

	app.providerMgr = mgr
	app.agentReg = reg
	app.skillReg = skillReg
	app.skillExec = skillExec
	app.memoryStore = memStore
	app.toolExec = toolExec
	app.mcpMgr = mcpMgr
	app.lspMgr = lspMgr
	app.fileWatcher = watcher.New("")

	// --- Internal services ---
	app.contextBuilder = ictx.NewBuilder(mgr, memStore)
	app.terminalMgr = terminal.NewManager(app.emit, nil) // ctxDone set in startup
	app.fileService = files.NewService()
	app.gitService = iggit.NewService()

	// AI service — uses context.Builder for context/compression callbacks
	app.aiService = ai.NewService(
		mgr, toolExec, memStore, reg,
		app.emit,
		app.contextBuilder.BuildContextMessage,
		app.contextBuilder.SummarizeAndCompressWithFlag,
		app.contextBuilder.GetModelContextWindow,
		func() context.Context { return app.ctx },
	)

	return app
}

// emit sends events to the Wails frontend.
func (a *App) emit(event string, data interface{}) {
	if a.ctx == nil {
		return
	}
	wailsRuntime.EventsEmit(a.ctx, event, data)
}

// ------- Lifecycle -------

// startup is called by Wails at application startup.
func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	a.terminalMgr = terminal.NewManager(a.emit, ctx.Done())
	a.lspMgr.SetContext(ctx)
	a.fileWatcher.SetContext(ctx)
	go a.mcpMgr.StartAll(context.Background())

	// Check if this is a first run (no providers configured)
	providers := a.providerMgr.GetProviders()
	hasConfig := false
	for _, p := range providers {
		if p.Enabled {
			hasConfig = true
			break
		}
	}
	if !hasConfig {
		time.AfterFunc(2*time.Second, func() {
			a.emit("app:first-run", map[string]interface{}{
				"needsSetup": true,
				"message":    "欢迎使用 StarCore！请先配置 AI 提供商以启用智能编程功能。",
			})
		})
	}
}

// IsProviderConfigured checks if any AI provider has been set up.
func (a *App) IsProviderConfigured() bool {
	for _, p := range a.providerMgr.GetProviders() {
		if p.Enabled {
			return true
		}
	}
	return false
}

// SetProjectPath starts file watching on the given path.
func (a *App) SetProjectPath(path string) {
	a.fileWatcher.Stop()
	a.fileWatcher = watcher.New(path)
	a.fileWatcher.SetContext(a.ctx)
	a.fileWatcher.Start(2 * time.Second)
}

// ------- Window controls -------

// MinimizeWindow minimizes the application window.
func (a *App) MinimizeWindow() {
	wailsRuntime.WindowMinimise(a.ctx)
}

// MaximizeWindow toggles the application window maximize state.
func (a *App) MaximizeWindow() {
	wailsRuntime.WindowToggleMaximise(a.ctx)
}

// CloseWindow closes the application.
func (a *App) CloseWindow() {
	wailsRuntime.Quit(a.ctx)
}

// Greet returns a greeting message.
func (a *App) Greet(name string) string {
	return fmt.Sprintf("Hello %s, It's show time!", name)
}

// ------- Terminal (delegates to terminal.Manager) -------

// NewTerminal creates a new terminal session.
func (a *App) NewTerminal(cwd string) (string, error) {
	return a.terminalMgr.New(cwd)
}

// ConnectTerminal connects the frontend to a buffered terminal session.
func (a *App) ConnectTerminal(id string) error {
	return a.terminalMgr.Connect(id)
}

// StartTerminal creates a new terminal (convenience wrapper).
func (a *App) StartTerminal(cwd string) error {
	_, err := a.terminalMgr.New(cwd)
	return err
}

// TerminalWrite writes data to a terminal's PTY.
func (a *App) TerminalWrite(id string, data string) error {
	return a.terminalMgr.Write(id, data)
}

// TerminalResize resizes a terminal's PTY dimensions.
func (a *App) TerminalResize(id string, cols int, rows int) error {
	return a.terminalMgr.Resize(id, cols, rows)
}

// KillTerminal closes and removes a terminal session.
func (a *App) KillTerminal(id string) error {
	return a.terminalMgr.Kill(id)
}

// ListTerminals returns all active terminal sessions.
func (a *App) ListTerminals() []map[string]interface{} {
	return a.terminalMgr.List()
}

// ------- File operations (delegates to files.Service) -------

// OpenFolder opens a native directory picker dialog.
func (a *App) OpenFolder() (string, error) {
	result, err := wailsRuntime.OpenDirectoryDialog(a.ctx, wailsRuntime.OpenDialogOptions{
		Title: "Select Folder",
	})
	if err != nil {
		return "", err
	}
	return result, nil
}

// ListDir lists files and directories in the given path.
func (a *App) ListDir(path string) ([]FileInfo, error) {
	return a.fileService.ListDir(path)
}

// ReadFile reads a file and returns its content.
func (a *App) ReadFile(path string) (string, error) {
	return a.fileService.ReadFile(path)
}

// WriteFile writes content to a file.
func (a *App) WriteFile(path string, content string) error {
	return a.fileService.WriteFile(path, content)
}

// CreateFile creates an empty file.
func (a *App) CreateFile(path string) error {
	return a.fileService.CreateFile(path)
}

// DeleteFile deletes a file.
func (a *App) DeleteFile(path string) error {
	return a.fileService.DeleteFile(path)
}

// RenameFile renames a file or directory.
func (a *App) RenameFile(oldPath, newPath string) error {
	return a.fileService.RenameFile(oldPath, newPath)
}

// CreateDir creates a new directory.
func (a *App) CreateDir(path string) error {
	return a.fileService.CreateDir(path)
}

// ExecuteCommand runs a shell command and returns its output.
func (a *App) ExecuteCommand(command string) (string, error) {
	return a.fileService.ExecuteCommand(command)
}

// SearchFiles searches for a query in files.
func (a *App) SearchFiles(query string, options SearchOptions) ([]SearchResult, error) {
	return a.fileService.SearchFiles(query, options)
}

// ReplaceInFiles replaces all occurrences of query with replacement in files.
func (a *App) ReplaceInFiles(query string, replacement string, options SearchOptions) error {
	return a.fileService.ReplaceInFiles(query, replacement, options)
}

// ApplyDiff applies a diff to a file.
func (a *App) ApplyDiff(req ApplyDiffRequest) error {
	return a.fileService.ApplyDiff(req.FilePath, req.Hunks)
}

// ComputeDiff computes a diff between the file on disk and new content.
func (a *App) ComputeDiff(filePath string, newContent string) ([]DiffHunk, error) {
	return a.fileService.ComputeDiff(filePath, newContent)
}

// ------- Git (delegates to git.Service) -------

// GitStatus returns the working tree status in a git repo.
func (a *App) GitStatus(projectPath string) ([]GitStatusEntry, error) {
	return a.gitService.Status(projectPath)
}

// GitBranch returns the current branch name.
func (a *App) GitBranch(projectPath string) (string, error) {
	return a.gitService.Branch(projectPath)
}

// GitLog returns recent commit history.
func (a *App) GitLog(projectPath string, count int) ([]GitLogEntry, error) {
	return a.gitService.Log(projectPath, count)
}

// GitCommit creates a commit with the given message.
func (a *App) GitCommit(projectPath string, message string) error {
	return a.gitService.Commit(projectPath, message)
}

// GitStage stages a file for commit.
func (a *App) GitStage(projectPath string, filePath string) error {
	return a.gitService.Stage(projectPath, filePath)
}

// GitUnstage unstages a file.
func (a *App) GitUnstage(projectPath string, filePath string) error {
	return a.gitService.Unstage(projectPath, filePath)
}

// GitDiff returns the working tree diff.
func (a *App) GitDiff(projectPath string, filePath string) (string, error) {
	return a.gitService.Diff(projectPath, filePath)
}

// GitStatusAndBranch returns combined status and branch info.
func (a *App) GitStatusAndBranch(projectPath string) (map[string]interface{}, error) {
	return a.gitService.StatusAndBranch(projectPath)
}

// GitPull pulls from the remote.
func (a *App) GitPull(projectPath string) (string, error) {
	return a.gitService.Pull(projectPath)
}

// GitPush pushes to the remote.
func (a *App) GitPush(projectPath string) (string, error) {
	return a.gitService.Push(projectPath)
}

// GitFetch fetches from the remote.
func (a *App) GitFetch(projectPath string) (string, error) {
	return a.gitService.Fetch(projectPath)
}

// GitCheckout switches to a branch.
func (a *App) GitCheckout(projectPath string, branch string) (string, error) {
	return a.gitService.Checkout(projectPath, branch)
}

// GitCreateBranch creates a new branch.
func (a *App) GitCreateBranch(projectPath string, branch string) (string, error) {
	return a.gitService.CreateBranch(projectPath, branch)
}

// GitMerge merges a branch into the current branch.
func (a *App) GitMerge(projectPath string, branch string) (string, error) {
	return a.gitService.Merge(projectPath, branch)
}

// GitStash manages git stash operations.
func (a *App) GitStash(projectPath string, action string) (string, error) {
	return a.gitService.Stash(projectPath, action)
}

// ------- AI (delegates to ai.Service) -------

// AIChatStream initiates a streaming AI chat with agent support.
func (a *App) AIChatStream(req provider.ChatRequest) error {
	return a.aiService.ChatStream(req)
}

// AIChat performs a non-streaming AI chat.
func (a *App) AIChat(req provider.ChatRequest) (string, error) {
	return a.aiService.Chat(req)
}

// AICompletion performs a code completion request.
func (a *App) AICompletion(providerID string, req provider.CompletionRequest) (string, error) {
	return a.aiService.Completion(providerID, req)
}

// StopGenerating cancels the current agent run.
func (a *App) StopGenerating() {
	a.aiService.Stop()
}

// ------- Provider pass-through -------

// GetProviders returns all registered AI providers.
func (a *App) GetProviders() []provider.ProviderInfo {
	return a.providerMgr.GetProviders()
}

// GetModels returns available models for a provider.
func (a *App) GetModels(providerID string) ([]provider.Model, error) {
	return a.providerMgr.GetModels(providerID)
}

// GetAgents returns all registered agents.
func (a *App) GetAgents() []agent.AgentDef {
	return a.agentReg.List()
}

// GetAgentConfig returns the configuration for a specific agent.
func (a *App) GetAgentConfig(agentID string) (agent.AgentDef, error) {
	ag, ok := a.agentReg.Get(agentID)
	if !ok {
		return agent.AgentDef{}, fmt.Errorf("agent not found: %s", agentID)
	}
	return ag, nil
}

// SetProviderConfig updates the configuration for a provider.
func (a *App) SetProviderConfig(providerID string, config provider.ProviderConfig) error {
	return a.providerMgr.SetProviderConfig(providerID, config)
}

// TestProvider tests the connection to a provider.
func (a *App) TestProvider(providerID string) error {
	return a.providerMgr.TestConnection(providerID)
}

// ------- Skill -------

// GetSkills returns all registered skills.
func (a *App) GetSkills() []skill.SkillDef {
	return a.skillReg.List()
}

// SaveSkill saves a skill as a SKILL.md file in the skills directory.
func (a *App) SaveSkill(s skill.SkillDef) error {
	dir := skill.GetSkillsDir(configDir())
	skillDir := filepath.Join(dir, s.ID)
	os.MkdirAll(skillDir, 0755)
	content := skill.BuildSkillMarkdown(s)
	return ioutil.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte(content), 0644)
}

// DeleteSkill removes a skill from the filesystem.
func (a *App) DeleteSkill(skillID string) error {
	dir := skill.GetSkillsDir(configDir())
	return os.RemoveAll(filepath.Join(dir, skillID))
}

// InstallSkillFromURL fetches a SKILL.md from a URL and installs it.
func (a *App) InstallSkillFromURL(url string) error {
	resp, err := http.Get(url)
	if err != nil {
		return fmt.Errorf("下载失败: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("下载失败: HTTP %d", resp.StatusCode)
	}
	data, err := ioutil.ReadAll(io.LimitReader(resp.Body, 50000))
	if err != nil {
		return fmt.Errorf("读取失败: %w", err)
	}
	content := string(data)
	// Parse frontmatter to get skill ID
	fm, body := parseSkillFrontmatter(content)
	id := fm["id"]
	if id == "" {
		// Extract ID from the first heading
		for _, line := range strings.Split(body, "\n") {
			line = strings.TrimSpace(line)
			if strings.HasPrefix(line, "# ") {
				id = strings.ToLower(strings.ReplaceAll(strings.TrimPrefix(line, "# "), " ", "-"))
				break
			}
		}
	}
	if id == "" {
		return fmt.Errorf("无法从文件中提取 Skill ID")
	}
	dir := skill.GetSkillsDir(configDir())
	skillDir := filepath.Join(dir, id)
	os.MkdirAll(skillDir, 0755)
	return ioutil.WriteFile(filepath.Join(skillDir, "SKILL.md"), data, 0644)
}

func parseSkillFrontmatter(content string) (map[string]string, string) {
	fm := make(map[string]string)
	lines := strings.Split(content, "\n")
	if len(lines) == 0 || strings.TrimSpace(lines[0]) != "---" {
		return fm, content
	}
	endIdx := -1
	for i := 1; i < len(lines); i++ {
		if strings.TrimSpace(lines[i]) == "---" {
			endIdx = i
			break
		}
		parts := strings.SplitN(strings.TrimSpace(lines[i]), ":", 2)
		if len(parts) == 2 {
			fm[strings.TrimSpace(parts[0])] = strings.TrimSpace(parts[1])
		}
	}
	if endIdx < 0 {
		return fm, content
	}
	return fm, strings.Join(lines[endIdx+1:], "\n")
}

// ExecuteSkill executes a skill by ID.
func (a *App) ExecuteSkill(skillID string, sctx skill.SkillContext, providerID string, model string) error {
	s, ok := a.skillReg.Get(skillID)
	if !ok {
		return fmt.Errorf("skill not found: %s", skillID)
	}

	eventCh, err := a.skillExec.Execute(a.ctx, skillID, sctx, providerID, model)
	if err != nil {
		wailsRuntime.EventsEmit(a.ctx, "skill:stream:error", err.Error())
		return err
	}

	go func() {
		wailsRuntime.EventsEmit(a.ctx, "skill:stream:start", s.ID)
		for event := range eventCh {
			switch event.Type {
			case "data":
				wailsRuntime.EventsEmit(a.ctx, "skill:stream:data", event.Content)
			case "done":
				wailsRuntime.EventsEmit(a.ctx, "skill:stream:done", s.ID)
			case "error":
				wailsRuntime.EventsEmit(a.ctx, "skill:stream:error", event.Content)
			case "thinking":
				wailsRuntime.EventsEmit(a.ctx, "skill:stream:thinking", event.Content)
			}
		}
	}()

	return nil
}

// ------- Memory / Conversation -------

// GetConversations returns recent conversations for a project.
func (a *App) GetConversations(projectPath string) ([]memory.Conversation, error) {
	if a.memoryStore == nil {
		return nil, nil
	}
	return a.memoryStore.GetConversations(projectPath, 50, 0)
}

// SaveConversation saves a conversation.
func (a *App) SaveConversation(conv memory.Conversation) error {
	if a.memoryStore == nil {
		return nil
	}
	return a.memoryStore.SaveConversation(&conv)
}

// DeleteConversation deletes a conversation.
func (a *App) DeleteConversation(id string) error {
	if a.memoryStore == nil {
		return nil
	}
	return a.memoryStore.DeleteConversation(id)
}

// GetMessages returns messages for a conversation.
func (a *App) GetMessages(conversationID string) ([]memory.Message, error) {
	if a.memoryStore == nil {
		return nil, nil
	}
	return a.memoryStore.GetMessages(conversationID)
}

// SaveMessage saves a message.
func (a *App) SaveMessage(msg memory.Message) error {
	if a.memoryStore == nil {
		return nil
	}
	return a.memoryStore.SaveMessage(&msg)
}

// GetKnowledge returns knowledge entries for a project.
func (a *App) GetKnowledge(projectPath string) ([]memory.Knowledge, error) {
	if a.memoryStore == nil {
		return nil, nil
	}
	return a.memoryStore.GetKnowledge(projectPath)
}

// SaveKnowledge saves a knowledge entry.
func (a *App) SaveKnowledge(entry memory.Knowledge) error {
	if a.memoryStore == nil {
		return nil
	}
	return a.memoryStore.SaveKnowledge(&entry)
}

// DeleteKnowledge deletes a knowledge entry.
func (a *App) DeleteKnowledge(id string) error {
	if a.memoryStore == nil {
		return nil
	}
	return a.memoryStore.DeleteKnowledge(id)
}

// ------- Tools -------

// GetTools returns all registered tools.
func (a *App) GetTools() []agent.ToolDef {
	return a.toolExec.ListToolDefs()
}

// ExecuteToolCall executes a tool call.
func (a *App) ExecuteToolCall(call agent.ToolCall) (*agent.ToolResult, error) {
	return a.toolExec.Execute(a.ctx, call)
}

// SetToolAutoApprove sets auto-approval for a tool.
func (a *App) SetToolAutoApprove(toolID string, approve bool) {
	a.toolExec.SetAutoApprove(toolID, approve)
}

// ------- Token usage -------

// GetTokenUsage returns token usage statistics.
func (a *App) GetTokenUsage(period string) (*memory.TokenUsageStats, error) {
	if a.memoryStore == nil {
		return nil, nil
	}
	return a.memoryStore.GetTokenUsage("", period)
}

// SaveTokenUsageEntry saves a token usage entry.
func (a *App) SaveTokenUsageEntry(entry memory.TokenUsageEntry) error {
	if a.memoryStore == nil {
		return nil
	}
	return a.memoryStore.SaveTokenUsage(&entry)
}

// ------- MCP -------

// GetMCPServers returns all MCP server configurations.
func (a *App) GetMCPServers() []mcp.MCPServerConfig {
	return a.mcpMgr.GetServers()
}

// AddMCPServer adds an MCP server.
func (a *App) AddMCPServer(config mcp.MCPServerConfig) error {
	return a.mcpMgr.AddServer(config)
}

// RemoveMCPServer removes an MCP server.
func (a *App) RemoveMCPServer(id string) error {
	return a.mcpMgr.RemoveServer(id)
}

// StartMCPServer starts an MCP server.
func (a *App) StartMCPServer(id string) error {
	return a.mcpMgr.StartServer(a.ctx, id)
}

// StopMCPServer stops an MCP server.
func (a *App) StopMCPServer(id string) error {
	return a.mcpMgr.StopServer(id)
}

// ------- LSP -------

// LSPDidOpen notifies LSP that a file was opened.
func (a *App) LSPDidOpen(filePath string, text string) error {
	return a.lspMgr.DidOpen(filePath, text)
}

// LSPDidChange notifies LSP that a file was changed.
func (a *App) LSPDidChange(filePath string, text string) error {
	return a.lspMgr.DidChange(filePath, text)
}

// LSPCloseFile notifies LSP that a file was closed.
func (a *App) LSPCloseFile(filePath string) {
	a.lspMgr.CloseFile(filePath)
}

// LSPCompletions returns LSP completion items.
func (a *App) LSPCompletions(filePath string, line int, col int) ([]lsp.FrontendCompletion, error) {
	return a.lspMgr.GetCompletions(filePath, line, col)
}

// LSPHover returns hover information at a position.
func (a *App) LSPHover(filePath string, line int, col int) (*lsp.Hover, error) {
	return a.lspMgr.GetHover(filePath, line, col)
}

// LSPDefinition returns definition locations for a symbol.
func (a *App) LSPDefinition(filePath string, line int, col int) ([]lsp.Location, error) {
	return a.lspMgr.GetDefinition(filePath, line, col)
}

// LSPShutdown shuts down all LSP servers.
func (a *App) LSPShutdown() {
	a.lspMgr.Shutdown()
}

// ReadFileWithLSP reads a file and notifies LSP that it's opened.
func (a *App) ReadFileWithLSP(path string) (string, error) {
	data, err := ioutil.ReadFile(path)
	if err != nil {
		return "", err
	}
	text := string(data)
	a.lspMgr.DidOpen(path, text)
	return text, nil
}

// WriteFileWithLSP writes a file and notifies LSP of the change.
func (a *App) WriteFileWithLSP(path string, content string) error {
	err := ioutil.WriteFile(path, []byte(content), 0644)
	if err == nil {
		a.lspMgr.DidChange(path, content)
	}
	return err
}

// ------- Project analysis (delegates to context.Builder) -------

// AnalyzeProject analyzes a project directory using AI.
func (a *App) AnalyzeProject(projectPath string) (string, error) {
	return a.contextBuilder.AnalyzeProject(projectPath)
}

// GetProjectAnalysis retrieves a cached project analysis.
func (a *App) GetProjectAnalysis(projectPath string) (string, error) {
	return a.contextBuilder.GetProjectAnalysis(projectPath)
}

// ------- Custom models -------

// SaveCustomModels persists custom model configurations.
func (a *App) SaveCustomModels(models []CustomModelEntry) error {
	configPath := filepath.Join(configDir(), "custom_models.json")
	data, err := json.MarshalIndent(models, "", "  ")
	if err != nil {
		return err
	}
	return ioutil.WriteFile(configPath, data, 0644)
}

// LoadCustomModels loads custom model configurations.
func (a *App) LoadCustomModels() ([]CustomModelEntry, error) {
	configPath := filepath.Join(configDir(), "custom_models.json")
	data, err := ioutil.ReadFile(configPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var models []CustomModelEntry
	if err := json.Unmarshal(data, &models); err != nil {
		return nil, err
	}
	for i := range models {
		if models[i].MaxTokens <= 0 {
			models[i].MaxTokens = provider.EstimateContextWindow(models[i].ModelID)
		}
	}
	return models, nil
}
