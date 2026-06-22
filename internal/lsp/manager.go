package lsp

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"

	wailsRuntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

type ServerInfo struct {
	LanguageID string
	Command    string
	Args       []string
	Extensions []string
	Custom     bool
	client     *Client
}

type FrontendServerInfo struct {
	LanguageID string   `json:"languageId"`
	Command    string   `json:"command"`
	Args       []string `json:"args"`
	Extensions []string `json:"extensions"`
	Custom     bool     `json:"custom"`
	Running    bool     `json:"running"`
}

type LanguagePackage struct {
	ID           string   `json:"id"`
	Name         string   `json:"name"`
	LanguageID   string   `json:"languageId"`
	Command      string   `json:"command"`
	Args         []string `json:"args"`
	Extensions   []string `json:"extensions"`
	InstallCmd   string   `json:"installCmd"`
	DownloadURL  string   `json:"downloadUrl"`
	DownloadFile string   `json:"downloadFile"`
	Description  string   `json:"description"`
	Category     string   `json:"category"`
	HasHighlight bool     `json:"hasHighlight"`
}

var PresetLanguagePackages = []LanguagePackage{
	{
		ID: "rust", Name: "Rust", LanguageID: "rust",
		Command: "rust-analyzer", Args: []string{},
		Extensions:   []string{".rs"},
		DownloadURL:  "https://github.com/rust-lang/rust-analyzer/releases/latest/download/rust-analyzer-x86_64-pc-windows-msvc.gz",
		DownloadFile: "rust-analyzer.exe",
		Description:  "Rust 语言服务器 (rust-analyzer)",
		Category:     "language",
		HasHighlight: true,
	},
	{
		ID: "vue", Name: "Vue", LanguageID: "vue",
		Command: "vue-language-server", Args: []string{"--stdio"},
		Extensions:   []string{".vue"},
		InstallCmd:   "npm install -g @vue/language-server",
		Description:  "Vue 3 语言服务器 (Volar)，需 npm",
		Category:     "framework",
		HasHighlight: true,
	},
	{
		ID: "java", Name: "Java", LanguageID: "java",
		Command: "jdtls", Args: []string{},
		Extensions:   []string{".java"},
		DownloadURL:  "https://download.eclipse.org/jdtls/milestones/latest/jdt-language-server-latest.tar.gz",
		DownloadFile: "jdtls",
		Description:  "Java 语言服务器 (Eclipse JDT.LS)",
		Category:     "language",
		HasHighlight: true,
	},
	{
		ID: "csharp", Name: "C#", LanguageID: "csharp",
		Command: "omnisharp", Args: []string{"--stdio"},
		Extensions:   []string{".cs"},
		DownloadURL:  "https://github.com/OmniSharp/omnisharp-roslyn/releases/latest/download/omnisharp-win-x64-net6.0.zip",
		DownloadFile: "omnisharp.exe",
		Description:  "C# 语言服务器 (OmniSharp)",
		Category:     "language",
		HasHighlight: true,
	},
	{
		ID: "ruby", Name: "Ruby", LanguageID: "ruby",
		Command: "solargraph", Args: []string{"stdio"},
		Extensions:   []string{".rb", ".erb"},
		InstallCmd:   "gem install solargraph",
		Description:  "Ruby 语言服务器 (Solargraph)，需 gem",
		Category:     "language",
		HasHighlight: false,
	},
	{
		ID: "dockerfile", Name: "Dockerfile", LanguageID: "dockerfile",
		Command: "docker-langserver", Args: []string{"--stdio"},
		Extensions:   []string{".dockerfile"},
		InstallCmd:   "npm install -g dockerfile-language-server-nodejs",
		Description:  "Dockerfile 语言服务器，需 npm",
		Category:     "tool",
		HasHighlight: false,
	},
	{
		ID: "bash", Name: "Bash", LanguageID: "bash",
		Command: "bash-language-server", Args: []string{"start"},
		Extensions:   []string{".sh", ".bash"},
		InstallCmd:   "npm install -g bash-language-server",
		Description:  "Bash 语言服务器，需 npm",
		Category:     "language",
		HasHighlight: false,
	},
	{
		ID: "lua", Name: "Lua", LanguageID: "lua",
		Command: "lua-language-server", Args: []string{},
		Extensions:   []string{".lua"},
		DownloadURL:  "https://github.com/LuaLS/lua-language-server/releases/latest/download/lua-language-server-3.13.5-win32-x64.zip",
		DownloadFile: "lua-language-server.exe",
		Description:  "Lua 语言服务器",
		Category:     "language",
		HasHighlight: false,
	},
	{
		ID: "terraform", Name: "Terraform", LanguageID: "terraform",
		Command: "terraform-ls", Args: []string{},
		Extensions:   []string{".tf", ".tfvars"},
		DownloadURL:  "https://github.com/hashicorp/terraform-ls/releases/latest/download/terraform-ls_0.34.1_windows_amd64.zip",
		DownloadFile: "terraform-ls.exe",
		Description:  "Terraform 语言服务器",
		Category:     "tool",
		HasHighlight: false,
	},
}

func GetPresetLanguagePackages() []LanguagePackage {
	return PresetLanguagePackages
}

func FindLocalBin(command string) string {
	d, err := os.UserConfigDir()
	if err != nil {
		return ""
	}
	binDir := filepath.Join(d, "StarCore", "bin")

	for _, name := range []string{command, command + ".exe"} {
		p := filepath.Join(binDir, name)
		if info, err := os.Stat(p); err == nil && !info.IsDir() {
			return p
		}
	}
	return ""
}

type Manager struct {
	ctx      context.Context
	rootPath string
	servers  map[string]*ServerInfo // keyed by languageID
	mu       sync.Mutex
	docs     map[string]*docState // keyed by filepath
	docsMu   sync.Mutex
}

type docState struct {
	URI     string
	LangID  string
	Version int
	Text    string
}

func NewManager() *Manager {
	return &Manager{
		servers: make(map[string]*ServerInfo),
		docs:    make(map[string]*docState),
	}
}

func (m *Manager) SetContext(ctx context.Context) {
	m.ctx = ctx
}

func (m *Manager) SetRootPath(rootPath string) {
	m.rootPath = rootPath
}

func (m *Manager) RegisterServer(langID, command string, args []string, extensions []string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.servers[langID] = &ServerInfo{
		LanguageID: langID,
		Command:    command,
		Args:       args,
		Extensions: extensions,
	}
}

func (m *Manager) RegisterCustomServer(langID, command string, args []string, extensions []string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.servers[langID] = &ServerInfo{
		LanguageID: langID,
		Command:    command,
		Args:       args,
		Extensions: extensions,
		Custom:     true,
	}
}

func (m *Manager) UnregisterServer(langID string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	info, ok := m.servers[langID]
	if ok && info.Custom {
		if info.client != nil {
			info.client.Notify("shutdown", nil)
			info.client.Close()
		}
		delete(m.servers, langID)
	}
}

func (m *Manager) ListServers() []FrontendServerInfo {
	m.mu.Lock()
	defer m.mu.Unlock()
	result := make([]FrontendServerInfo, 0, len(m.servers))
	for _, info := range m.servers {
		result = append(result, FrontendServerInfo{
			LanguageID: info.LanguageID,
			Command:    info.Command,
			Args:       info.Args,
			Extensions: info.Extensions,
			Custom:     info.Custom,
			Running:    info.client != nil,
		})
	}
	return result
}

func (m *Manager) detectLanguage(filePath string) string {
	ext := strings.ToLower(filepath.Ext(filePath))
	if ext == "" {
		return ""
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	for _, info := range m.servers {
		for _, registered := range info.Extensions {
			if strings.ToLower(registered) == ext {
				return info.LanguageID
			}
		}
	}

	return ""
}

func (m *Manager) getOrStartServer(filePath string) (*ServerInfo, error) {
	langID := m.detectLanguage(filePath)
	if langID == "" {
		return nil, fmt.Errorf("no language server for: %s", filePath)
	}

	m.mu.Lock()
	info, ok := m.servers[langID]
	if !ok {
		m.mu.Unlock()
		return nil, fmt.Errorf("no server registered for: %s", langID)
	}

	if info.client != nil {
		m.mu.Unlock()
		return info, nil
	}
	m.mu.Unlock()

	cmd := info.Command
	if localBin := FindLocalBin(info.Command); localBin != "" {
		cmd = localBin
	}

	client, err := NewClient(cmd, info.Args...)
	if err != nil {
		return nil, fmt.Errorf("start %s server: %w", langID, err)
	}

	// Handle diagnostics notification
	client.OnNotification("textDocument/publishDiagnostics", func(params json.RawMessage) {
		var diagParams PublishDiagnosticsParams
		if err := json.Unmarshal(params, &diagParams); err != nil {
			return
		}
		filePath := URItoFilePath(diagParams.URI)

		diags := make([]FrontendDiagnostic, 0, len(diagParams.Diagnostics))
		for _, d := range diagParams.Diagnostics {
			diags = append(diags, FrontendDiagnostic{
				FilePath: filePath,
				Line:     d.Range.Start.Line,
				Col:      d.Range.Start.Character,
				Message:  d.Message,
				Severity: SeverityString(d.Severity),
				Source:   d.Source,
			})
		}

		if m.ctx != nil {
			wailsRuntime.EventsEmit(m.ctx, "lsp:diagnostics", diags)
		}
	})

	// Initialize
	rootPath := m.rootPath
	if rootPath == "" {
		rootPath = "."
	}
	rootURI := DocumentURI(rootPath)
	initParams := InitializeParams{
		ProcessID: 0,
		RootURI:   rootURI,
		Capabilities: ClientCapabilities{
			TextDocument: TextDocumentClientCapabilities{
				Completion: CompletionClientCapabilities{
					CompletionItem: CompletionItemCapabilities{SnippetSupport: false},
				},
				Hover: HoverClientCapabilities{
					ContentFormat: []string{"markdown", "plaintext"},
				},
			},
		},
	}

	var initResult InitializeResult
	if err := client.Call("initialize", initParams, &initResult); err != nil {
		client.Close()
		return nil, fmt.Errorf("initialize %s: %w", langID, err)
	}

	client.Notify("initialized", struct{}{})

	log.Printf("LSP: %s server started (completion=%v, hover=%v, definition=%v)",
		langID,
		initResult.Capabilities.CompletionProvider != nil,
		initResult.Capabilities.HoverProvider,
		initResult.Capabilities.DefinitionProvider,
	)

	info.client = client
	return info, nil
}

// DidOpen notifies the language server that a file has been opened
func (m *Manager) DidOpen(filePath, text string) error {
	info, err := m.getOrStartServer(filePath)
	if err != nil {
		return err
	}

	uri := DocumentURI(filePath)
	langID := m.detectLanguage(filePath)

	m.docsMu.Lock()
	doc := &docState{URI: uri, LangID: langID, Version: 1, Text: text}
	m.docs[filePath] = doc
	m.docsMu.Unlock()

	return info.client.Notify("textDocument/didOpen", DidOpenParams{
		TextDocument: TextDocumentItem{
			URI:        uri,
			LanguageID: langID,
			Version:    1,
			Text:       text,
		},
	})
}

// DidChange notifies the language server that a file has changed
func (m *Manager) DidChange(filePath, text string) error {
	info, err := m.getOrStartServer(filePath)
	if err != nil {
		return err
	}

	m.docsMu.Lock()
	doc, ok := m.docs[filePath]
	if !ok {
		m.docsMu.Unlock()
		return m.DidOpen(filePath, text)
	}
	doc.Version++
	doc.Text = text
	version := doc.Version
	m.docsMu.Unlock()

	return info.client.Notify("textDocument/didChange", DidChangeParams{
		TextDocument: VersionedTextDocumentIdentifier{
			URI:     DocumentURI(filePath),
			Version: version,
		},
		ContentChanges: []TextDocumentContentChangeEvent{
			{Text: text},
		},
	})
}

// GetCompletions requests code completions from the language server
func (m *Manager) GetCompletions(filePath string, line, col int) ([]FrontendCompletion, error) {
	info, err := m.getOrStartServer(filePath)
	if err != nil {
		return nil, err
	}

	var result CompletionList
	err = info.client.Call("textDocument/completion", CompletionParams{
		TextDocument: TextDocumentIdentifier{URI: DocumentURI(filePath)},
		Position:     Position{Line: line, Character: col},
	}, &result)

	if err != nil {
		return nil, err
	}

	completions := make([]FrontendCompletion, 0, len(result.Items))
	for _, item := range result.Items {
		insertText := item.InsertText
		if insertText == "" {
			insertText = item.Label
		}
		if item.TextEdit != nil {
			insertText = item.TextEdit.NewText
		}
		completions = append(completions, FrontendCompletion{
			Label:      item.Label,
			InsertText: insertText,
			Kind:       item.Kind,
			Detail:     item.Detail,
		})
	}
	return completions, nil
}

// GetHover returns hover information at a position
func (m *Manager) GetHover(filePath string, line, col int) (*Hover, error) {
	info, err := m.getOrStartServer(filePath)
	if err != nil {
		return nil, err
	}

	var result Hover
	err = info.client.Call("textDocument/hover", HoverParams{
		TextDocument: TextDocumentIdentifier{URI: DocumentURI(filePath)},
		Position:     Position{Line: line, Character: col},
	}, &result)
	if err != nil {
		return nil, err
	}
	return &result, nil
}

// GetDefinition returns the definition location
func (m *Manager) GetDefinition(filePath string, line, col int) ([]Location, error) {
	info, err := m.getOrStartServer(filePath)
	if err != nil {
		return nil, err
	}

	var result interface{}
	err = info.client.Call("textDocument/definition", DefinitionParams{
		TextDocument: TextDocumentIdentifier{URI: DocumentURI(filePath)},
		Position:     Position{Line: line, Character: col},
	}, &result)
	if err != nil {
		return nil, err
	}

	// Result can be a single Location or []Location
	var locations []Location
	if locs, ok := result.([]interface{}); ok {
		for _, l := range locs {
			b, _ := json.Marshal(l)
			var loc Location
			json.Unmarshal(b, &loc)
			locations = append(locations, loc)
		}
	} else {
		b, _ := json.Marshal(result)
		var loc Location
		if err := json.Unmarshal(b, &loc); err == nil {
			locations = append(locations, loc)
		}
	}
	return locations, nil
}

// GetReferences returns all references to a symbol
func (m *Manager) GetReferences(filePath string, line, col int, includeDecl bool) ([]FrontendLocation, error) {
	info, err := m.getOrStartServer(filePath)
	if err != nil {
		return nil, err
	}

	var result interface{}
	err = info.client.Call("textDocument/references", ReferenceParams{
		TextDocument: TextDocumentIdentifier{URI: DocumentURI(filePath)},
		Position:     Position{Line: line, Character: col},
		Context:      ReferenceContext{IncludeDeclaration: includeDecl},
	}, &result)
	if err != nil {
		return nil, err
	}

	var locations []Location
	if locs, ok := result.([]interface{}); ok {
		for _, l := range locs {
			b, _ := json.Marshal(l)
			var loc Location
			json.Unmarshal(b, &loc)
			locations = append(locations, loc)
		}
	} else {
		b, _ := json.Marshal(result)
		var loc Location
		if err := json.Unmarshal(b, &loc); err == nil {
			locations = append(locations, loc)
		}
	}

	frontend := make([]FrontendLocation, 0, len(locations))
	for _, loc := range locations {
		frontend = append(frontend, FrontendLocation{
			FilePath: strings.TrimPrefix(loc.URI, "file:///"),
			Line:     loc.Range.Start.Line,
			Col:      loc.Range.Start.Character,
			EndLine:  loc.Range.End.Line,
			EndCol:   loc.Range.End.Character,
		})
	}
	return frontend, nil
}

// GetCodeActions returns code actions for a range
func (m *Manager) GetCodeActions(filePath string, startLine, startCol, endLine, endCol int) ([]FrontendCodeAction, error) {
	info, err := m.getOrStartServer(filePath)
	if err != nil {
		return nil, err
	}

	var result interface{}
	err = info.client.Call("textDocument/codeAction", CodeActionParams{
		TextDocument: TextDocumentIdentifier{URI: DocumentURI(filePath)},
		Range: Range{
			Start: Position{Line: startLine, Character: startCol},
			End:   Position{Line: endLine, Character: endCol},
		},
		Context: CodeActionContext{Diagnostics: []Diagnostic{}},
	}, &result)
	if err != nil {
		return nil, err
	}

	var actions []CodeAction
	if items, ok := result.([]interface{}); ok {
		for _, item := range items {
			b, _ := json.Marshal(item)
			var action CodeAction
			if err := json.Unmarshal(b, &action); err == nil {
				actions = append(actions, action)
			}
		}
	}

	frontend := make([]FrontendCodeAction, 0, len(actions))
	for _, a := range actions {
		fa := FrontendCodeAction{
			Title: a.Title,
			Kind:  a.Kind,
		}
		if a.Edit != nil && a.Edit.Changes != nil {
			fa.Edit = make(map[string][]FrontendTextEdit)
			for uri, edits := range a.Edit.Changes {
				fp := strings.TrimPrefix(uri, "file:///")
				fe := make([]FrontendTextEdit, 0, len(edits))
				for _, e := range edits {
					fe = append(fe, FrontendTextEdit{
						NewText:   e.NewText,
						StartLine: e.Range.Start.Line,
						StartCol:  e.Range.Start.Character,
						EndLine:   e.Range.End.Line,
						EndCol:    e.Range.End.Character,
					})
				}
				fa.Edit[fp] = fe
			}
		}
		frontend = append(frontend, fa)
	}
	return frontend, nil
}

// Shutdown stops all language servers
func (m *Manager) Shutdown() {
	m.mu.Lock()
	defer m.mu.Unlock()
	for langID, info := range m.servers {
		if info.client != nil {
			info.client.Notify("shutdown", nil)
			info.client.Close()
			log.Printf("LSP: %s server shut down", langID)
		}
	}
}

// CloseFile notifies the language server that a file was closed
func (m *Manager) CloseFile(filePath string) {
	m.docsMu.Lock()
	delete(m.docs, filePath)
	m.docsMu.Unlock()

	langID := m.detectLanguage(filePath)
	m.mu.Lock()
	info, ok := m.servers[langID]
	m.mu.Unlock()

	if ok && info.client != nil {
		info.client.Notify("textDocument/didClose", DidCloseParams{
			TextDocument: TextDocumentIdentifier{URI: DocumentURI(filePath)},
		})
	}
}

type RenameResult struct {
	Changes map[string][]TextEdit `json:"changes"`
}

type SignatureHelp struct {
	Signatures      []SignatureInfo `json:"signatures"`
	ActiveSignature int             `json:"activeSignature"`
	ActiveParameter int             `json:"activeParameter"`
}

type SignatureInfo struct {
	Label         string          `json:"label"`
	Documentation string          `json:"documentation,omitempty"`
	Parameters    []ParameterInfo `json:"parameters,omitempty"`
}

type ParameterInfo struct {
	Label         string `json:"label"`
	Documentation string `json:"documentation,omitempty"`
}

type WorkspaceSymbol struct {
	Name          string   `json:"name"`
	Kind          int      `json:"kind"`
	ContainerName string   `json:"containerName,omitempty"`
	Location      Location `json:"location"`
}

func (m *Manager) Rename(filePath string, line, col int, newName string) (*RenameResult, error) {
	srv, err := m.getOrStartServer(filePath)
	if err != nil {
		return nil, err
	}
	if srv.client == nil {
		return nil, fmt.Errorf("LSP server not running")
	}

	var result RenameResult
	err = srv.client.Call("textDocument/rename", map[string]any{
		"textDocument": TextDocumentIdentifier{URI: DocumentURI(filePath)},
		"position":     Position{Line: line, Character: col},
		"newName":      newName,
	}, &result)
	if err != nil {
		return nil, fmt.Errorf("rename: %w", err)
	}
	return &result, nil
}

func (m *Manager) Formatting(filePath string) ([]TextEdit, error) {
	srv, err := m.getOrStartServer(filePath)
	if err != nil {
		return nil, err
	}
	if srv.client == nil {
		return nil, fmt.Errorf("LSP server not running")
	}

	var edits []TextEdit
	err = srv.client.Call("textDocument/formatting", map[string]any{
		"textDocument": TextDocumentIdentifier{URI: DocumentURI(filePath)},
		"options": map[string]any{
			"tabSize":      4,
			"insertSpaces": true,
		},
	}, &edits)
	if err != nil {
		return nil, fmt.Errorf("formatting: %w", err)
	}
	return edits, nil
}

func (m *Manager) RangeFormatting(filePath string, startLine, startCol, endLine, endCol int) ([]TextEdit, error) {
	srv, err := m.getOrStartServer(filePath)
	if err != nil {
		return nil, err
	}
	if srv.client == nil {
		return nil, fmt.Errorf("LSP server not running")
	}

	var edits []TextEdit
	err = srv.client.Call("textDocument/rangeFormatting", map[string]any{
		"textDocument": TextDocumentIdentifier{URI: DocumentURI(filePath)},
		"range": Range{
			Start: Position{Line: startLine, Character: startCol},
			End:   Position{Line: endLine, Character: endCol},
		},
		"options": map[string]any{
			"tabSize":      4,
			"insertSpaces": true,
		},
	}, &edits)
	if err != nil {
		return nil, fmt.Errorf("rangeFormatting: %w", err)
	}
	return edits, nil
}

func (m *Manager) SignatureHelp(filePath string, line, col int) (*SignatureHelp, error) {
	srv, err := m.getOrStartServer(filePath)
	if err != nil {
		return nil, err
	}
	if srv.client == nil {
		return nil, fmt.Errorf("LSP server not running")
	}

	var result SignatureHelp
	err = srv.client.Call("textDocument/signatureHelp", map[string]any{
		"textDocument": TextDocumentIdentifier{URI: DocumentURI(filePath)},
		"position":     Position{Line: line, Character: col},
	}, &result)
	if err != nil {
		return nil, fmt.Errorf("signatureHelp: %w", err)
	}
	return &result, nil
}

func (m *Manager) WorkspaceSymbols(query string) ([]WorkspaceSymbol, error) {
	m.mu.Lock()
	var client *Client
	for _, info := range m.servers {
		if info.client != nil {
			client = info.client
			break
		}
	}
	m.mu.Unlock()

	if client == nil {
		return nil, fmt.Errorf("no language server running")
	}

	var symbols []WorkspaceSymbol
	err := client.Call("workspace/symbol", map[string]any{
		"query": query,
	}, &symbols)
	if err != nil {
		return nil, fmt.Errorf("workspaceSymbol: %w", err)
	}
	return symbols, nil
}

type CodeLens struct {
	Range   Range    `json:"range"`
	Command *Command `json:"command,omitempty"`
	Data    any      `json:"data,omitempty"`
}

type Command struct {
	Title     string `json:"title"`
	Command   string `json:"command"`
	Arguments []any  `json:"arguments,omitempty"`
}

func (m *Manager) GetCodeLens(filePath string) ([]CodeLens, error) {
	srv, err := m.getOrStartServer(filePath)
	if err != nil {
		return nil, err
	}
	if srv.client == nil {
		return nil, fmt.Errorf("LSP server not running")
	}

	var lenses []CodeLens
	err = srv.client.Call("textDocument/codeLens", map[string]any{
		"textDocument": TextDocumentIdentifier{URI: DocumentURI(filePath)},
	}, &lenses)
	if err != nil {
		return nil, fmt.Errorf("codeLens: %w", err)
	}
	return lenses, nil
}

type InlayHint struct {
	Position     Position `json:"position"`
	Label        any      `json:"label"`
	Kind         int      `json:"kind,omitempty"`
	Tooltip      any      `json:"tooltip,omitempty"`
	PaddingLeft  bool     `json:"paddingLeft,omitempty"`
	PaddingRight bool     `json:"paddingRight,omitempty"`
}

func (m *Manager) GetInlayHints(filePath string, startLine, startCol, endLine, endCol int) ([]InlayHint, error) {
	srv, err := m.getOrStartServer(filePath)
	if err != nil {
		return nil, err
	}
	if srv.client == nil {
		return nil, fmt.Errorf("LSP server not running")
	}

	var hints []InlayHint
	err = srv.client.Call("textDocument/inlayHint", map[string]any{
		"textDocument": TextDocumentIdentifier{URI: DocumentURI(filePath)},
		"range": map[string]any{
			"start": map[string]any{"line": startLine, "character": startCol},
			"end":   map[string]any{"line": endLine, "character": endCol},
		},
	}, &hints)
	if err != nil {
		return nil, fmt.Errorf("inlayHint: %w", err)
	}
	return hints, nil
}

type DocumentSymbol struct {
	Name           string           `json:"name"`
	Kind           int              `json:"kind"`
	Range          Range            `json:"range"`
	SelectionRange Range            `json:"selectionRange"`
	Children       []DocumentSymbol `json:"children,omitempty"`
}

func (m *Manager) GetDocumentSymbols(filePath string) ([]DocumentSymbol, error) {
	srv, err := m.getOrStartServer(filePath)
	if err != nil {
		return nil, err
	}
	if srv.client == nil {
		return nil, fmt.Errorf("LSP server not running")
	}

	var symbols []DocumentSymbol
	err = srv.client.Call("textDocument/documentSymbol", map[string]any{
		"textDocument": TextDocumentIdentifier{URI: DocumentURI(filePath)},
	}, &symbols)
	if err != nil {
		return nil, fmt.Errorf("documentSymbol: %w", err)
	}
	return symbols, nil
}

type FoldingRange struct {
	StartLine int    `json:"startLine"`
	EndLine   int    `json:"endLine"`
	Kind      string `json:"kind,omitempty"`
}

func (m *Manager) GetFoldingRanges(filePath string) ([]FoldingRange, error) {
	srv, err := m.getOrStartServer(filePath)
	if err != nil {
		return nil, err
	}
	if srv.client == nil {
		return nil, fmt.Errorf("LSP server not running")
	}

	var ranges []FoldingRange
	err = srv.client.Call("textDocument/foldingRange", map[string]any{
		"textDocument": TextDocumentIdentifier{URI: DocumentURI(filePath)},
	}, &ranges)
	if err != nil {
		return nil, fmt.Errorf("foldingRange: %w", err)
	}
	return ranges, nil
}
