package lsp

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
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
	client     *Client
}

type Manager struct {
	ctx       context.Context
	servers   map[string]*ServerInfo // keyed by languageID
	mu        sync.Mutex
	docs      map[string]*docState // keyed by filepath
	docsMu    sync.Mutex
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

func (m *Manager) detectLanguage(filePath string) string {
	ext := strings.ToLower(filepath.Ext(filePath))
	switch ext {
	case ".go":
		return "go"
	case ".js", ".jsx", ".mjs", ".cjs":
		return "javascript"
	case ".ts", ".tsx":
		return "typescript"
	case ".py", ".pyw":
		return "python"
	case ".rs":
		return "rust"
	case ".json":
		return "json"
	case ".html", ".htm":
		return "html"
	case ".css":
		return "css"
	case ".md":
		return "markdown"
	case ".yaml", ".yml":
		return "yaml"
	case ".sql":
		return "sql"
	default:
		return ""
	}
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

	// Start the server
	client, err := NewClient(info.Command, info.Args...)
	if err != nil {
		return nil, fmt.Errorf("start %s server: %w", langID, err)
	}

	// Handle diagnostics notification
	client.OnNotification("textDocument/publishDiagnostics", func(params json.RawMessage) {
		var diagParams PublishDiagnosticsParams
		if err := json.Unmarshal(params, &diagParams); err != nil {
			return
		}
		filePath := strings.TrimPrefix(diagParams.URI, "file:///")

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
	rootURI := DocumentURI(".")
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
