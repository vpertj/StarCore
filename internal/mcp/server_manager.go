package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sync"

	"StarCore/internal/agent"
)

type ServerManager struct {
	clients  map[string]*MCPClient
	configs  map[string]MCPServerConfig
	toolExec *agent.ToolExecutor
	mu       sync.RWMutex
	dataDir  string
}

func NewServerManager(toolExec *agent.ToolExecutor, dataDir string) *ServerManager {
	return &ServerManager{
		clients:  make(map[string]*MCPClient),
		configs:  make(map[string]MCPServerConfig),
		toolExec: toolExec,
		dataDir:  dataDir,
	}
}

func (m *ServerManager) LoadConfig() error {
	configPath := filepath.Join(m.dataDir, "mcp_servers.json")
	data, err := os.ReadFile(configPath)
	if err != nil {
		if os.IsNotExist(err) {
			// First run: seed built-in MCP templates
			m.seedBuiltinTemplates()
			return m.SaveConfig()
		}
		return err
	}

	var configs map[string]MCPServerConfig
	if err := json.Unmarshal(data, &configs); err != nil {
		return err
	}

	m.mu.Lock()
	for id, cfg := range configs {
		m.configs[id] = cfg
	}
	m.mu.Unlock()
	return nil
}

func (m *ServerManager) seedBuiltinTemplates() {
	templates := []MCPServerConfig{
		{
			ID:        "github",
			Name:      "GitHub (需 npx + GitHub Token)",
			Command:   "npx",
			Args:      []string{"-y", "@anthropic/mcp-server-github"},
			Transport: "stdio",
			Env:       map[string]string{"GITHUB_TOKEN": "your_token_here"},
			Enabled:   false,
		},
		{
			ID:        "filesystem",
			Name:      "文件系统访问 (需 npx)",
			Command:   "npx",
			Args:      []string{"-y", "@modelcontextprotocol/server-filesystem", "."},
			Transport: "stdio",
			Enabled:   false,
		},
		{
			ID:        "postgres",
			Name:      "PostgreSQL (需 npx + PG 连接)",
			Command:   "npx",
			Args:      []string{"-y", "@modelcontextprotocol/server-postgres", "postgresql://localhost:5432"},
			Transport: "stdio",
			Env:       map[string]string{"DATABASE_URL": ""},
			Enabled:   false,
		},
		{
			ID:        "git",
			Name:      "Git 操作 (需 uvx 或 pip install mcp-server-git)",
			Command:   "uvx",
			Args:      []string{"mcp-server-git"},
			Transport: "stdio",
			Enabled:   false,
		},
		{
			ID:        "brave-search",
			Name:      "Brave 网页搜索 (需 npx + API Key)",
			Command:   "npx",
			Args:      []string{"-y", "@anthropic/mcp-server-brave-search"},
			Transport: "stdio",
			Env:       map[string]string{"BRAVE_API_KEY": "your_key_here"},
			Enabled:   false,
		},
		{
			ID:        "sqlite",
			Name:      "SQLite 数据库 (需 uvx)",
			Command:   "uvx",
			Args:      []string{"mcp-server-sqlite", "--db-path", "database.db"},
			Transport: "stdio",
			Enabled:   false,
		},
		{
			ID:        "docker",
			Name:      "Docker 管理 (需 uvx)",
			Command:   "uvx",
			Args:      []string{"mcp-server-docker"},
			Transport: "stdio",
			Enabled:   false,
		},
	}
	for _, t := range templates {
		if _, exists := m.configs[t.ID]; !exists {
			m.configs[t.ID] = t
		}
	}
}

func (m *ServerManager) SaveConfig() error {
	configPath := filepath.Join(m.dataDir, "mcp_servers.json")
	if err := os.MkdirAll(m.dataDir, 0755); err != nil {
		return fmt.Errorf("create data dir: %w", err)
	}

	m.mu.RLock()
	data, err := json.MarshalIndent(m.configs, "", "  ")
	m.mu.RUnlock()
	if err != nil {
		return err
	}

	return os.WriteFile(configPath, data, 0644)
}

func (m *ServerManager) AddServer(config MCPServerConfig) error {
	m.mu.Lock()
	m.configs[config.ID] = config
	m.mu.Unlock()
	return m.SaveConfig()
}

func (m *ServerManager) RemoveServer(id string) error {
	m.mu.Lock()
	if client, ok := m.clients[id]; ok {
		client.Close()
		delete(m.clients, id)
	}
	delete(m.configs, id)
	m.mu.Unlock()
	return m.SaveConfig()
}

func (m *ServerManager) StartServer(ctx context.Context, id string) error {
	m.mu.RLock()
	cfg, ok := m.configs[id]
	m.mu.RUnlock()
	if !ok {
		return fmt.Errorf("server not found: %s", id)
	}
	if !cfg.Enabled {
		return fmt.Errorf("server is disabled: %s", id)
	}

	client, err := NewClient(cfg)
	if err != nil {
		return fmt.Errorf("failed to create client: %w", err)
	}

	if err := client.Initialize(ctx); err != nil {
		client.Close()
		return fmt.Errorf("failed to initialize: %w", err)
	}

	for _, tool := range client.GetTools() {
		mcpTool := &MCPToolAdapter{
			client:   client,
			toolInfo: tool,
		}
		m.toolExec.Register(mcpTool)
	}

	m.mu.Lock()
	m.clients[id] = client
	m.mu.Unlock()
	return nil
}

func (m *ServerManager) StopServer(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	client, ok := m.clients[id]
	if !ok {
		return nil
	}

	for _, tool := range client.GetTools() {
		m.toolExec.Unregister("mcp_" + tool.Name)
	}

	err := client.Close()
	delete(m.clients, id)
	return err
}

func (m *ServerManager) StartAll(ctx context.Context) {
	m.mu.RLock()
	configs := make([]MCPServerConfig, 0)
	for _, cfg := range m.configs {
		if cfg.Enabled {
			configs = append(configs, cfg)
		}
	}
	m.mu.RUnlock()

	for _, cfg := range configs {
		if err := m.StartServer(ctx, cfg.ID); err != nil {
			log.Printf("Warning: failed to start MCP server %s: %v", cfg.ID, err)
		}
	}
}

func (m *ServerManager) StopAll() {
	m.mu.Lock()
	defer m.mu.Unlock()
	for id, client := range m.clients {
		client.Close()
		delete(m.clients, id)
	}
}

func (m *ServerManager) GetServers() []MCPServerConfig {
	m.mu.Lock()
	m.seedBuiltinTemplates()
	result := make([]MCPServerConfig, 0, len(m.configs))
	for _, cfg := range m.configs {
		result = append(result, cfg)
	}
	m.mu.Unlock()
	return result
}

func (m *ServerManager) GetClient(id string) (*MCPClient, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	c, ok := m.clients[id]
	return c, ok
}

func (m *ServerManager) IsRunning(id string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	_, ok := m.clients[id]
	return ok
}

type MCPToolAdapter struct {
	client   *MCPClient
	toolInfo ToolInfo
}

func (a *MCPToolAdapter) ID() string          { return "mcp_" + a.toolInfo.Name }
func (a *MCPToolAdapter) Name() string        { return a.toolInfo.Name }
func (a *MCPToolAdapter) Description() string { return a.toolInfo.Description }
func (a *MCPToolAdapter) Parameters() agent.ToolParameters {
	return a.toolInfo.InputSchema
}
func (a *MCPToolAdapter) RequiresApproval() bool { return true }

func (a *MCPToolAdapter) Execute(ctx context.Context, args map[string]any) (string, error) {
	return a.client.CallTool(ctx, a.toolInfo.Name, args)
}
