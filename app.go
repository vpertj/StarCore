package main

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"io"
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
	"StarCore/internal/codescan"
	"StarCore/internal/completion"
	ictx 	"StarCore/internal/context"
	"StarCore/internal/debug"
	"StarCore/internal/extension"
	"StarCore/internal/files"
	iggit "StarCore/internal/git"
	"StarCore/internal/lsp"
	"StarCore/internal/mcp"
	"StarCore/internal/memory"
	"StarCore/internal/pipeline"
	"StarCore/internal/provider"
	"StarCore/internal/rag"
	"StarCore/internal/remote"
	"StarCore/internal/sandbox"
	"StarCore/internal/skill"
	skillBuiltins "StarCore/internal/skill/builtins"
	"StarCore/internal/terminal"
	"StarCore/internal/verify"
	"StarCore/internal/watcher"
	"StarCore/internal/workspace"

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
	WorkspaceRoot  = workspace.WorkspaceRoot
	Extension      = extension.Extension
	ExtCommand     = extension.CommandContribution
	RemoteConn     = remote.Connection
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
	_ = os.MkdirAll(d, 0755)
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

	fileWatchers   map[string]*watcher.Watcher
	sandboxConfigs map[string]*sandbox.Config
	activeProject  string
	openProjects   []string

	aiService      *ai.Service
	contextBuilder *ictx.Builder
	terminalMgr    *terminal.Manager
	fileService    *files.Service
	gitService     *iggit.Service
	pipelineExec   *pipeline.Executor
	completionSvc  *completion.Service
	ragSvc         *rag.Service
	verifySvc      *verify.Service
	codeScanEngine *codescan.RuleEngine
	workspaceMgr   *workspace.Manager
	extRegistry    *extension.Registry
	remoteMgr      *remote.Manager
	debugMgr       *debug.Manager
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

	agentTools.SkillToolRegistry = skillReg
	agentTools.SubAgentToolExec = toolExec
	agentTools.SubAgentProviderMgr = mgr

	// --- Memory store ---
	memStore, err := memory.NewStore(dataDir)
	if err != nil {
		// On Windows, CGO may be disabled; SQLite won't work but app continues
		log.Printf("Warning: memory store unavailable: %v", err)
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

	// --- Skill executor (after memStore) ---
	skillExec := skill.NewExecutor(skillReg, mgr, toolExec, memStore)
	agentTools.SkillToolExecutor = skillExec
	toolExec.Register(agentTools.NewSkillTool())
	toolExec.Register(agentTools.NewSubAgentTool())
	agentTools.SubAgentMemoryStore = memStore

	app.providerMgr = mgr
	app.agentReg = reg
	app.skillReg = skillReg
	app.skillExec = skillExec
	app.memoryStore = memStore
	app.toolExec = toolExec
	app.mcpMgr = mcpMgr
	app.lspMgr = lspMgr
	app.fileWatchers = make(map[string]*watcher.Watcher)
	app.sandboxConfigs = make(map[string]*sandbox.Config)
	app.workspaceMgr = workspace.NewManager()
	app.extRegistry = extension.NewRegistry(dataDir)
	app.remoteMgr = remote.NewManager()
	app.debugMgr = debug.NewManager(app.emit)

	app.loadCustomLSPServers()

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
		app.autoVerify,
	)

	app.pipelineExec = pipeline.NewExecutor(mgr, toolExec, reg, memStore, app.emit)
	app.completionSvc = completion.NewService(mgr)
	app.ragSvc = rag.NewService(mgr)
	app.verifySvc = verify.NewService("")
	app.codeScanEngine = codescan.NewRuleEngine()

	sandbox.InitAuditLogger(dataDir)

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
	for _, w := range a.fileWatchers {
		w.SetContext(ctx)
	}
	go a.mcpMgr.StartAll(context.Background())

	crashed, crashedAt := memory.CheckAndClearCrashMarker(configDir())
	if crashed {
		a.emit("app:crash_recovery", map[string]interface{}{
			"crashedAt": crashedAt,
			"message":   "StarCore 上次异常退出，已自动恢复。您最近的对话已保留。",
		})
	}

	if a.memoryStore != nil {
		if state, err := a.memoryStore.LoadSessionState(); err == nil && state != nil {
			a.emit("app:session_restore", map[string]interface{}{
				"activeConvId": state.ActiveConvID,
				"projectPath":  state.ProjectPath,
				"agentId":      state.AgentID,
				"mode":         state.Mode,
				"providerId":   state.ProviderID,
				"model":        state.Model,
				"crashed":      crashed,
			})
		}
	}

	go func() {
		for {
			time.Sleep(30 * time.Second)
			memory.SaveCrashMarker(configDir())
		}
	}()

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

// shutdown is called by Wails when the application is closing.
func (a *App) shutdown(ctx context.Context) {
	// Stop all debug sessions
	if a.debugMgr != nil {
		a.debugMgr.StopAll()
	}

	// Stop all LSP servers
	if a.lspMgr != nil {
		a.lspMgr.Shutdown()
	}

	// Stop all file watchers
	for _, w := range a.fileWatchers {
		w.Stop()
	}

	// Stop MCP servers
	if a.mcpMgr != nil {
		a.mcpMgr.StopAll()
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
	if path == "" {
		return
	}
	// Add to open projects if not already present
	found := false
	for _, p := range a.openProjects {
		if p == path {
			found = true
			break
		}
	}
	if !found {
		a.openProjects = append(a.openProjects, path)
	}

	// Create watcher for this project if not already exists
	if _, exists := a.fileWatchers[path]; !exists {
		w := watcher.New(path)
		w.SetContext(a.ctx)
		w.Start(2 * time.Second)
		a.fileWatchers[path] = w
	}

	// Set sandbox config for this project
	cfg := sandbox.DefaultConfig(path)
	a.sandboxConfigs[path] = cfg
	agentTools.SandboxConfig = cfg

	a.activeProject = path
	a.verifySvc.SetProjectDir(path)
	a.lspMgr.SetRootPath(path)
	a.workspaceMgr.AddRoot(path)
	a.workspaceMgr.SetActive(path)
}

// AddWorkspaceRoot adds an additional root to the workspace.
func (a *App) AddWorkspaceRoot(path string) {
	a.workspaceMgr.AddRoot(path)
}

// RemoveWorkspaceRoot removes a root from the workspace.
func (a *App) RemoveWorkspaceRoot(path string) {
	a.workspaceMgr.RemoveRoot(path)
}

// SetActiveWorkspaceRoot sets the active workspace root.
func (a *App) SetActiveWorkspaceRoot(path string) {
	a.workspaceMgr.SetActive(path)
}

// SwitchProject changes the active project without destroying others.
func (a *App) SwitchProject(path string) {
	a.activeProject = path
	if cfg, ok := a.sandboxConfigs[path]; ok {
		agentTools.SandboxConfig = cfg
	}
	a.verifySvc.SetProjectDir(path)
	a.lspMgr.SetRootPath(path)
	a.workspaceMgr.SetActive(path)
	a.emit("project:switched", map[string]interface{}{"path": path})
}

// GetOpenProjects returns all open project paths.
func (a *App) GetOpenProjects() []string {
	return a.openProjects
}

// CloseProject cleans up a specific project's resources.
func (a *App) CloseProject(path string) {
	if w, ok := a.fileWatchers[path]; ok {
		w.Stop()
		delete(a.fileWatchers, path)
	}
	a.terminalMgr.KillByProject(path)
	delete(a.sandboxConfigs, path)
	newProjects := make([]string, 0, len(a.openProjects)-1)
	for _, p := range a.openProjects {
		if p != path {
			newProjects = append(newProjects, p)
		}
	}
	a.openProjects = newProjects
	if a.activeProject == path && len(a.openProjects) > 0 {
		a.SwitchProject(a.openProjects[0])
	} else if len(a.openProjects) == 0 {
		a.activeProject = ""
	}
}

// GetWorkspaceRoots returns all workspace roots.
func (a *App) GetWorkspaceRoots() []workspace.WorkspaceRoot {
	return a.workspaceMgr.Roots()
}

// GetExtensions returns all registered extensions.
func (a *App) GetExtensions() []extension.Extension {
	return a.extRegistry.List()
}

// GetExtensionCommands returns all extension-contributed commands.
func (a *App) GetExtensionCommands() []extension.CommandContribution {
	return a.extRegistry.GetCommands()
}

// SetExtensionEnabled enables or disables an extension.
func (a *App) SetExtensionEnabled(id string, enabled bool) error {
	return a.extRegistry.SetEnabled(id, enabled)
}

// AddRemoteConnection adds a remote development connection.
func (a *App) AddRemoteConnection(conn remote.Connection) error {
	return a.remoteMgr.AddConnection(conn)
}

// RemoveRemoteConnection removes a remote connection.
func (a *App) RemoveRemoteConnection(id string) {
	a.remoteMgr.RemoveConnection(id)
}

// ListRemoteConnections returns all remote connections.
func (a *App) ListRemoteConnections() []remote.Connection {
	return a.remoteMgr.ListConnections()
}

// ConnectRemote establishes a remote connection.
func (a *App) ConnectRemote(id string) error {
	return a.remoteMgr.Connect(a.ctx, id)
}

// DisconnectRemote closes a remote connection.
func (a *App) DisconnectRemote(id string) {
	a.remoteMgr.Disconnect(id)
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

// ------- Terminal (delegates to terminal.Manager) -------

// NewTerminal creates a new terminal session.
func (a *App) NewTerminal(cwd string) (string, error) {
	return a.terminalMgr.New(cwd, a.activeProject)
}

// ConnectTerminal connects the frontend to a buffered terminal session.
func (a *App) ConnectTerminal(id string) error {
	return a.terminalMgr.Connect(id)
}

// StartTerminal creates a new terminal (convenience wrapper).
func (a *App) StartTerminal(cwd string) error {
	_, err := a.terminalMgr.New(cwd, a.activeProject)
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

// ListTerminalsByProject returns active terminal sessions for a specific project.
func (a *App) ListTerminalsByProject(projectPath string) []map[string]interface{} {
	return a.terminalMgr.ListByProject(projectPath)
}

// ------- Debug (delegates to debug.Manager) -------

// DebugStart starts a new debug session for the given Go program.
func (a *App) DebugStart(programPath string, args []string) (map[string]interface{}, error) {
	session, err := a.debugMgr.Start(programPath, args)
	if err != nil {
		return nil, err
	}
	return map[string]interface{}{
		"id":      session.ID,
		"program": session.ProgramPath,
		"port":    session.Port,
	}, nil
}

// DebugStop stops a debug session.
func (a *App) DebugStop(sessionID string) error {
	return a.debugMgr.Stop(sessionID)
}

// DebugList returns all active debug sessions.
func (a *App) DebugList() []map[string]interface{} {
	return a.debugMgr.List()
}

// DebugAddBreakpoint sets a breakpoint at the specified file and line.
func (a *App) DebugAddBreakpoint(sessionID string, file string, line int, condition string) (*debug.Breakpoint, error) {
	return a.debugMgr.AddBreakpoint(sessionID, file, line, condition)
}

// DebugAddBreakpointByFunc sets a breakpoint at the specified function.
func (a *App) DebugAddBreakpointByFunc(sessionID string, function string) (*debug.Breakpoint, error) {
	return a.debugMgr.AddBreakpointByFunc(sessionID, function)
}

// DebugRemoveBreakpoint removes a breakpoint by ID.
func (a *App) DebugRemoveBreakpoint(sessionID string, bpID int) error {
	return a.debugMgr.RemoveBreakpoint(sessionID, bpID)
}

// DebugListBreakpoints returns all breakpoints for a session.
func (a *App) DebugListBreakpoints(sessionID string) ([]debug.Breakpoint, error) {
	return a.debugMgr.ListBreakpoints(sessionID)
}

// DebugContinue resumes program execution.
func (a *App) DebugContinue(sessionID string) error {
	return a.debugMgr.Continue(sessionID)
}

// DebugStepOver steps over the current line.
func (a *App) DebugStepOver(sessionID string) error {
	return a.debugMgr.StepOver(sessionID)
}

// DebugStepIn steps into the current function.
func (a *App) DebugStepIn(sessionID string) error {
	return a.debugMgr.StepIn(sessionID)
}

// DebugStepOut steps out of the current function.
func (a *App) DebugStepOut(sessionID string) error {
	return a.debugMgr.StepOut(sessionID)
}

// DebugGetState returns the current debug state.
func (a *App) DebugGetState(sessionID string) (*debug.SessionState, error) {
	return a.debugMgr.GetState(sessionID)
}

// DebugGetVariable evaluates an expression and returns its value.
func (a *App) DebugGetVariable(sessionID string, frameID int, expr string) (*debug.Variable, error) {
	return a.debugMgr.GetVariable(sessionID, frameID, expr)
}

// DebugRestart restarts the debug session.
func (a *App) DebugRestart(sessionID string) error {
	return a.debugMgr.Restart(sessionID)
}

// DebugDetach detaches from the debug session without killing the program.
func (a *App) DebugDetach(sessionID string) error {
	return a.debugMgr.Detach(sessionID)
}

// DebugConsoleExecute executes a debug console command.
func (a *App) DebugConsoleExecute(sessionID string, expr string) (*debug.ConsoleResult, error) {
	return a.debugMgr.ConsoleExecute(sessionID, expr)
}

// DebugCheckDlv checks if dlv is installed and returns its version.
func (a *App) DebugCheckDlv() (bool, string) {
	return debug.CheckDlvInstalled()
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

// FormatFile formats a file using the appropriate formatter.
func (a *App) FormatFile(path string) (string, error) {
	return a.fileService.FormatFile(path)
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
	return a.gitService.FetchRemote(projectPath, "")
}

// GitBlame returns per-line blame information for a file.
func (a *App) GitBlame(projectPath string, filePath string) ([]iggit.BlameLine, error) {
	return a.gitService.Blame(projectPath, filePath)
}

// GitVisualDiff returns a unified diff for a file.
func (a *App) GitVisualDiff(projectPath string, filePath string) (string, error) {
	return a.gitService.VisualDiff(projectPath, filePath)
}

// GitDiffBetween returns diff between two refs for a file.
func (a *App) GitDiffBetween(projectPath string, from string, to string, filePath string) (string, error) {
	return a.gitService.DiffBetween(projectPath, from, to, filePath)
}

// GitRemoteList returns configured remotes.
func (a *App) GitRemoteList(projectPath string) (string, error) {
	return a.gitService.RemoteList(projectPath)
}

// GitLogFile returns commit history for a specific file.
func (a *App) GitLogFile(projectPath string, filePath string, count int) ([]iggit.LogEntry, error) {
	return a.gitService.LogFile(projectPath, filePath, count)
}

// GitConflictFiles returns files with merge conflicts.
func (a *App) GitConflictFiles(projectPath string) ([]string, error) {
	return a.gitService.ConflictFiles(projectPath)
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

// ContinueAgentLoop adds extra iterations to the running agent loop.
func (a *App) ContinueAgentLoop(extraLoops int) {
	a.aiService.ContinueLoop(extraLoops)
}

// RespondToAsk is called by the frontend when the user answers an ask_user prompt.
func (a *App) RespondToAsk(response agentTools.AskUserResponse) bool {
	return a.aiService.RespondToAsk(response)
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
	resolved := filepath.Join(dir, s.ID)
	if !strings.HasPrefix(filepath.Clean(resolved), filepath.Clean(dir)+string(filepath.Separator)) {
		return fmt.Errorf("invalid skill ID")
	}
	if err := os.MkdirAll(resolved, 0755); err != nil {
		return fmt.Errorf("创建技能目录失败: %w", err)
	}
	content := skill.BuildSkillMarkdown(s)
	return os.WriteFile(filepath.Join(resolved, "SKILL.md"), []byte(content), 0644)
}

// DeleteSkill removes a skill from the filesystem.
func (a *App) DeleteSkill(skillID string) error {
	dir := skill.GetSkillsDir(configDir())
	resolved := filepath.Join(dir, skillID)
	if !strings.HasPrefix(filepath.Clean(resolved), filepath.Clean(dir)+string(filepath.Separator)) {
		return fmt.Errorf("invalid skill ID")
	}
	return os.RemoveAll(resolved)
}

// InstallSkillFromURL fetches a SKILL.md from a URL and installs it.
func (a *App) InstallSkillFromURL(url string) error {
	if err := sandbox.ValidateURL(url); err != nil {
		return fmt.Errorf("URL验证失败: %w", err)
	}
	resp, err := http.Get(url)
	if err != nil {
		return fmt.Errorf("下载失败: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("下载失败: HTTP %d", resp.StatusCode)
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, 50000))
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
	if err := os.MkdirAll(skillDir, 0755); err != nil {
		return fmt.Errorf("create skill dir: %w", err)
	}
	return os.WriteFile(filepath.Join(skillDir, "SKILL.md"), data, 0644)
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

// DeleteMessage deletes a single message by ID.
func (a *App) DeleteMessage(id string) error {
	if a.memoryStore == nil {
		return nil
	}
	return a.memoryStore.DeleteMessage(id)
}

// DeleteConversationMessages deletes all messages for a conversation (localStorage cleanup helper).
func (a *App) DeleteConversationMessages(convID string) error {
	if a.memoryStore == nil {
		return nil
	}
	// Messages are cascade-deleted with DeleteConversation; this is a no-op for SQLite.
	// Exists for API symmetry with frontend cleanup of localStorage.
	return nil
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

// RespondToolApproval is called by the frontend when the user approves/rejects a tool call.
func (a *App) RespondToolApproval(callID string, approved bool) bool {
	return a.toolExec.RespondApproval(callID, approved)
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

// ClearTokenUsage deletes all token usage records (reset).
func (a *App) ClearTokenUsage() error {
	if a.memoryStore == nil {
		return nil
	}
	return a.memoryStore.ClearTokenUsage()
}

// ------- Session State -------

// SaveSessionState persists the active session state for crash recovery.
func (a *App) SaveSessionState(state memory.SessionState) error {
	if a.memoryStore == nil {
		return nil
	}
	return a.memoryStore.SaveSessionState(&state)
}

// LoadSessionState returns the last saved session state.
func (a *App) LoadSessionState() (*memory.SessionState, error) {
	if a.memoryStore == nil {
		return nil, nil
	}
	return a.memoryStore.LoadSessionState()
}

// GetRecentConversations returns recent conversations for a project.
func (a *App) GetRecentConversations(projectPath string, limit int) ([]memory.Conversation, error) {
	if a.memoryStore == nil {
		return nil, nil
	}
	return a.memoryStore.GetRecentConversations(projectPath, limit)
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

// LSPReferences returns all references to a symbol.
func (a *App) LSPReferences(filePath string, line int, col int, includeDecl bool) ([]lsp.FrontendLocation, error) {
	return a.lspMgr.GetReferences(filePath, line, col, includeDecl)
}

// LSPCodeActions returns code actions for a range.
func (a *App) LSPCodeActions(filePath string, startLine int, startCol int, endLine int, endCol int) ([]lsp.FrontendCodeAction, error) {
	return a.lspMgr.GetCodeActions(filePath, startLine, startCol, endLine, endCol)
}

// LSPShutdown shuts down all LSP servers.
func (a *App) LSPShutdown() {
	a.lspMgr.Shutdown()
}

func (a *App) LSPRename(filePath string, line int, col int, newName string) (*lsp.RenameResult, error) {
	return a.lspMgr.Rename(filePath, line, col, newName)
}

func (a *App) LSPFormatting(filePath string) ([]lsp.TextEdit, error) {
	return a.lspMgr.Formatting(filePath)
}

func (a *App) LSPSignatureHelp(filePath string, line int, col int) (*lsp.SignatureHelp, error) {
	return a.lspMgr.SignatureHelp(filePath, line, col)
}

func (a *App) LSPWorkspaceSymbols(query string) ([]lsp.WorkspaceSymbol, error) {
	return a.lspMgr.WorkspaceSymbols(query)
}

func (a *App) LSPCodeLens(filePath string) ([]lsp.CodeLens, error) {
	return a.lspMgr.GetCodeLens(filePath)
}

func (a *App) LSPInlayHints(filePath string, startLine, startCol, endLine, endCol int) ([]lsp.InlayHint, error) {
	return a.lspMgr.GetInlayHints(filePath, startLine, startCol, endLine, endCol)
}

func (a *App) LSPDocumentSymbols(filePath string) ([]lsp.DocumentSymbol, error) {
	return a.lspMgr.GetDocumentSymbols(filePath)
}

func (a *App) LSPFoldingRanges(filePath string) ([]lsp.FoldingRange, error) {
	return a.lspMgr.GetFoldingRanges(filePath)
}

// GetLSPServers returns all registered LSP server configurations.
func (a *App) GetLSPServers() []lsp.FrontendServerInfo {
	return a.lspMgr.ListServers()
}

// AddLSPServer adds a custom LSP server configuration.
func (a *App) AddLSPServer(langID string, command string, args []string, extensions []string) error {
	a.lspMgr.RegisterCustomServer(langID, command, args, extensions)
	return a.saveCustomLSPServers()
}

// RemoveLSPServer removes a custom LSP server configuration.
func (a *App) RemoveLSPServer(langID string) error {
	a.lspMgr.UnregisterServer(langID)
	return a.saveCustomLSPServers()
}

func (a *App) saveCustomLSPServers() error {
	servers := a.lspMgr.ListServers()
	custom := make([]lsp.FrontendServerInfo, 0)
	for _, s := range servers {
		if s.Custom {
			custom = append(custom, s)
		}
	}
	data, err := json.MarshalIndent(custom, "", "  ")
	if err != nil {
		return err
	}
	configPath := filepath.Join(configDir(), "lsp_servers.json")
	return os.WriteFile(configPath, data, 0644)
}

func (a *App) loadCustomLSPServers() {
	configPath := filepath.Join(configDir(), "lsp_servers.json")
	data, err := os.ReadFile(configPath)
	if err != nil {
		return
	}
	var servers []lsp.FrontendServerInfo
	if err := json.Unmarshal(data, &servers); err != nil {
		return
	}
	for _, s := range servers {
		a.lspMgr.RegisterCustomServer(s.LanguageID, s.Command, s.Args, s.Extensions)
	}
}

// GetLanguagePackages returns all preset language packages.
func (a *App) GetLanguagePackages() []lsp.LanguagePackage {
	return lsp.GetPresetLanguagePackages()
}

// InstallLanguagePackage downloads or installs a language server.
func (a *App) InstallLanguagePackage(pkgID string) (string, error) {
	for _, pkg := range lsp.PresetLanguagePackages {
		if pkg.ID != pkgID {
			continue
		}
		if pkg.DownloadURL != "" {
			binDir := filepath.Join(configDir(), "bin")
			if err := os.MkdirAll(binDir, 0755); err != nil {
				return "", fmt.Errorf("创建目录失败: %w", err)
			}
			targetPath := filepath.Join(binDir, pkg.DownloadFile)
			if _, err := os.Stat(targetPath); err == nil {
				return "已安装: " + targetPath, nil
			}
			tmpPath := targetPath + ".tmp"
			client := &http.Client{Timeout: 5 * time.Minute}
			resp, err := client.Get(pkg.DownloadURL)
			if err != nil {
				return "", fmt.Errorf("下载失败: %w", err)
			}
			defer resp.Body.Close()
			if resp.StatusCode != http.StatusOK {
				return "", fmt.Errorf("下载失败: HTTP %d", resp.StatusCode)
			}
			f, err := os.Create(tmpPath)
			if err != nil {
				return "", fmt.Errorf("创建文件失败: %w", err)
			}
			written, err := io.Copy(f, io.LimitReader(resp.Body, 100*1024*1024))
			if err != nil {
				f.Close()
				os.Remove(tmpPath)
				return "", fmt.Errorf("写入失败: %w", err)
			}
			f.Close()
			if written >= 100*1024*1024 {
				os.Remove(tmpPath)
				return "", fmt.Errorf("下载文件超过100MB限制")
			}
			if strings.HasSuffix(pkg.DownloadURL, ".gz") {
				if err := decompressGz(tmpPath, targetPath); err != nil {
					os.Remove(tmpPath)
					return "", fmt.Errorf("解压失败: %w", err)
				}
				os.Remove(tmpPath)
			} else {
				if err := os.Rename(tmpPath, targetPath); err != nil {
					os.Remove(tmpPath)
					return "", fmt.Errorf("重命名失败: %w", err)
				}
			}
			if err := os.Chmod(targetPath, 0755); err != nil {
				return "", fmt.Errorf("设置权限失败: %w", err)
			}
			return "安装完成: " + targetPath, nil
		}
		if pkg.InstallCmd != "" {
			output, err := a.fileService.ExecuteCommand(pkg.InstallCmd)
			if err != nil {
				return output, fmt.Errorf("安装失败: %w", err)
			}
			return output, nil
		}
		return "", fmt.Errorf("该语言服务器需手动安装: %s", pkg.Command)
	}
	return "", fmt.Errorf("未找到语言包: %s", pkgID)
}

func decompressGz(src, dst string) error {
	f, err := os.Open(src)
	if err != nil {
		return err
	}
	defer f.Close()
	gz, err := io.ReadAll(f)
	if err != nil {
		return err
	}
	r, err := gzip.NewReader(bytes.NewReader(gz))
	if err != nil {
		return err
	}
	defer r.Close()
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()
	_, err = io.Copy(out, r)
	return err
}

// CheckCommandExists checks if a command is available on the system PATH or in local bin dir.
func (a *App) CheckCommandExists(command string) bool {
	if lsp.FindLocalBin(command) != "" {
		return true
	}
	_, err := a.fileService.ExecuteCommand(fmt.Sprintf("where %s 2>nul || which %s 2>/dev/null", command, command))
	return err == nil
}

// SetProxy configures the HTTP proxy for all AI providers.
func (a *App) SetProxy(proxyURL string, noProxy string) {
	provider.SetGlobalProxy(proxyURL, noProxy)
}

// GetProxy returns the current HTTP proxy configuration.
func (a *App) GetProxy() (string, string) {
	return provider.GetGlobalProxy()
}

// ReadFileWithLSP reads a file and notifies LSP that it's opened.
func (a *App) ReadFileWithLSP(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	text := string(data)
	a.lspMgr.DidOpen(path, text)
	return text, nil
}

// WriteFileWithLSP writes a file and notifies LSP of the change.
func (a *App) WriteFileWithLSP(path string, content string) error {
	err := os.WriteFile(path, []byte(content), 0644)
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

// ------- Editor settings persistence (file-based, like VS Code's settings.json) -------

// SaveEditorSettings persists editor settings to the StarCore config directory.
// This is the authoritative store — localStorage is only a fast cache.
func (a *App) SaveEditorSettings(settingsJSON string) error {
	configPath := filepath.Join(configDir(), "editor_settings.json")
	return os.WriteFile(configPath, []byte(settingsJSON), 0644)
}

// LoadEditorSettings loads editor settings from the config directory.
// Returns empty string if the file doesn't exist (first run).
func (a *App) LoadEditorSettings() (string, error) {
	configPath := filepath.Join(configDir(), "editor_settings.json")
	data, err := os.ReadFile(configPath)
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}
		return "", err
	}
	return string(data), nil
}

// ------- Custom models -------

// SaveCustomModels persists custom model configurations.
func (a *App) SaveCustomModels(models []CustomModelEntry) error {
	configPath := filepath.Join(configDir(), "custom_models.json")
	data, err := json.MarshalIndent(models, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(configPath, data, 0644)
}

// LoadCustomModels loads custom model configurations.
func (a *App) LoadCustomModels() ([]CustomModelEntry, error) {
	configPath := filepath.Join(configDir(), "custom_models.json")
	data, err := os.ReadFile(configPath)
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

// ------- Pipeline (Multi-Agent Orchestration) -------

// PipelineInfo is a lightweight pipeline descriptor for the frontend.
type PipelineInfo struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	StageCount  int    `json:"stageCount"`
}

// ListPipelines returns all available pipeline definitions.
func (a *App) ListPipelines() []PipelineInfo {
	pipelines := pipeline.AllPipelines()
	result := make([]PipelineInfo, len(pipelines))
	for i, p := range pipelines {
		result[i] = PipelineInfo{
			ID:          p.ID,
			Name:        p.Name,
			Description: p.Description,
			StageCount:  len(p.Stages),
		}
	}
	return result
}

// RunPipelineRequest is the request payload for RunPipeline.
type RunPipelineRequest struct {
	PipelineID  string `json:"pipelineId"`
	UserInput   string `json:"userInput"`
	ProjectPath string `json:"projectPath"`
}

// RunPipeline starts a pipeline execution and returns the result.
func (a *App) RunPipeline(req RunPipelineRequest) (*pipeline.PipelineRun, error) {
	var targetPipeline pipeline.Pipeline
	found := false
	for _, p := range pipeline.AllPipelines() {
		if p.ID == req.PipelineID {
			targetPipeline = p
			found = true
			break
		}
	}
	if !found {
		return nil, fmt.Errorf("pipeline not found: %s", req.PipelineID)
	}
	return a.pipelineExec.Run(a.ctx, targetPipeline, req.UserInput, req.ProjectPath)
}

// StopPipeline cancels the running pipeline.
func (a *App) StopPipeline() {
	a.pipelineExec.Stop()
}

// ------- Code Completion (FIM) -------

// CodeCompleteRequest is the request payload for CodeComplete.
type CodeCompleteRequest struct {
	BeforeCursor string `json:"beforeCursor"`
	AfterCursor  string `json:"afterCursor"`
	FileName     string `json:"fileName"`
	Language     string `json:"language"`
	MaxTokens    int    `json:"maxTokens,omitempty"`
}

// CodeComplete performs line-level code completion.
func (a *App) CodeComplete(req CodeCompleteRequest) (*completion.Suggestion, error) {
	return a.completionSvc.Complete(a.ctx, completion.FIMRequest{
		BeforeCursor: req.BeforeCursor,
		AfterCursor:  req.AfterCursor,
		FileName:     req.FileName,
		Language:     req.Language,
		MaxTokens:    req.MaxTokens,
	})
}

// CodeCompleteMultiLine performs block-level code completion.
func (a *App) CodeCompleteMultiLine(req CodeCompleteRequest) (*completion.Suggestion, error) {
	return a.completionSvc.CompleteMultiLine(a.ctx, completion.FIMRequest{
		BeforeCursor: req.BeforeCursor,
		AfterCursor:  req.AfterCursor,
		FileName:     req.FileName,
		Language:     req.Language,
		MaxTokens:    req.MaxTokens,
	})
}

// ------- RAG (Semantic Search) -------

// IndexProject indexes the project for semantic search.
func (a *App) IndexProject(projectPath string) error {
	return a.ragSvc.IndexProject(a.ctx, projectPath)
}

// RAGSearch performs semantic search on the indexed project.
func (a *App) RAGSearch(projectPath string, query string, topK int) ([]rag.SearchResult, error) {
	return a.ragSvc.Search(a.ctx, projectPath, query, topK)
}

// RAGSearchHybrid performs hybrid (semantic + keyword) search.
func (a *App) RAGSearchHybrid(projectPath string, query string, topK int) ([]rag.SearchResult, error) {
	return a.ragSvc.SearchHybrid(a.ctx, projectPath, query, topK)
}

// RAGIndexStats returns indexing statistics.
func (a *App) RAGIndexStats(projectPath string) map[string]any {
	return a.ragSvc.GetIndexStats(projectPath)
}

// RAGIndexFileIncremental incrementally indexes a single changed file.
func (a *App) RAGIndexFileIncremental(projectPath, filePath string) error {
	return a.ragSvc.IndexFileIncremental(a.ctx, projectPath, filePath)
}

// RAGRemoveFileFromIndex removes a file from the RAG index.
func (a *App) RAGRemoveFileFromIndex(projectPath, filePath string) {
	a.ragSvc.RemoveFileFromIndex(projectPath, filePath)
}

// ------- Verification (Edit→Lint→Test→Fix Loop) -------

// VerifyProject runs all checks on the current project.
func (a *App) VerifyProject() *verify.VerificationResult {
	return a.verifySvc.Verify(a.ctx, nil)
}

// VerifyFile runs checks for a specific file.
func (a *App) VerifyFile(filePath string) *verify.VerificationResult {
	return a.verifySvc.VerifyFile(a.ctx, filePath)
}

// QuickVerifyFile runs a quick check on a single file.
func (a *App) QuickVerifyFile(filePath string) *verify.CheckResult {
	return a.verifySvc.QuickVerify(a.ctx, filePath)
}

// RunTests runs project tests and returns structured results.
func (a *App) RunTests(testPath string) []verify.TestSuiteResult {
	return a.verifySvc.RunTests(a.ctx, testPath)
}

// autoVerify is the VerifyFunc callback for the AI service.
// It runs quick verification on modified files and returns a summary.
func (a *App) autoVerify(ctx context.Context, filePaths []string) string {
	if a.verifySvc == nil || len(filePaths) == 0 {
		return ""
	}
	var sb strings.Builder
	for _, fp := range filePaths {
		cr := a.verifySvc.QuickVerify(ctx, fp)
		if cr == nil {
			continue
		}
		if cr.Passed {
			sb.WriteString(fmt.Sprintf("✓ %s: %s passed\n", fp, cr.Type))
		} else {
			sb.WriteString(fmt.Sprintf("✗ %s: %s FAILED\n", fp, cr.Type))
			if cr.Output != "" {
				maxLen := 1500
				output := cr.Output
				if len(output) > maxLen {
					output = output[:maxLen] + "\n... [truncated]"
				}
				sb.WriteString(output + "\n")
			}
		}
	}
	return sb.String()
}

// ------- Code Scan (OWASP/CWE Rules) -------

// CodeScanFile scans a single file for security issues.
func (a *App) CodeScanFile(filePath string, content string) *codescan.ScanResult {
	return a.codeScanEngine.ScanFileWithResult(content, filePath, "")
}

// CodeScanRules returns all available security rules.
func (a *App) CodeScanRules() []codescan.Rule {
	return a.codeScanEngine.ListRules()
}

// CodeScanStats returns rule statistics.
func (a *App) CodeScanStats() map[string]int {
	return a.codeScanEngine.Stats()
}
