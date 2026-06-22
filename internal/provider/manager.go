package provider

import (
	"context"
	"encoding/json"
	"log"
	"os"
	"path/filepath"
	"sync"
)

type Manager struct {
	providers  map[string]Provider
	configs    map[string]ProviderConfig
	configPath string
	mu         sync.RWMutex
	appCtx     func() context.Context
}

func NewManager(dataDir string, appCtx func() context.Context) *Manager {
	m := &Manager{
		providers: make(map[string]Provider),
		configs:   make(map[string]ProviderConfig),
		appCtx:    appCtx,
	}
	if dataDir != "" {
		m.configPath = filepath.Join(dataDir, "provider_configs.json")
	}
	return m
}

func (m *Manager) Register(p Provider) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.providers[p.ID()] = p
	m.configs[p.ID()] = p.GetConfig()
}

func (m *Manager) LoadPersistedConfigs() error {
	if m.configPath == "" {
		return nil
	}
	data, err := os.ReadFile(m.configPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	var saved []ProviderConfig
	if err := json.Unmarshal(data, &saved); err != nil {
		return err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, cfg := range saved {
		m.configs[cfg.ID] = cfg
		if p, ok := m.providers[cfg.ID]; ok {
			p.SetConfig(cfg)
		}
	}
	return nil
}

func (m *Manager) saveConfigs() error {
	if m.configPath == "" {
		return nil
	}
	var configs []ProviderConfig
	for id, cfg := range m.configs {
		if cfg.ID == "" {
			cfg.ID = id
		}
		if cfg.Name == "" {
			cfg.Name = id
		}
		// API key or endpoint configured means the provider is enabled
		if cfg.APIKey != "" || cfg.Endpoint != "" {
			cfg.Enabled = true
		}
		configs = append(configs, cfg)
	}
	log.Printf("Saved %d provider configs to %s", len(configs), m.configPath)
	data, err := json.MarshalIndent(configs, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(m.configPath, data, 0600)
}

// getOrCreateProvider returns an existing provider or creates a new one
// for the given ID. Must be called with the write lock held.
func (m *Manager) getOrCreateProvider(providerID string) Provider {
	if p, ok := m.providers[providerID]; ok {
		return p
	}
	cfg, hasCfg := m.configs[providerID]

	switch providerID {
	case "anthropic":
		ap := NewAnthropicProvider()
		if hasCfg {
			ap.SetConfig(cfg)
		}
		m.providers[providerID] = ap
		return ap
	case "ollama":
		op := NewOllamaProvider()
		if hasCfg {
			op.SetConfig(cfg)
		}
		m.providers[providerID] = op
		return op
	default:
		name := providerID
		if hasCfg && cfg.Name != "" {
			name = cfg.Name
		}
		oai := NewGenericOpenAIProvider(providerID, name)
		if hasCfg {
			oai.SetConfig(cfg)
		}
		m.providers[providerID] = oai
		return oai
	}
}

func (m *Manager) Get(providerID string) (Provider, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.getOrCreateProvider(providerID), nil
}

func (m *Manager) Chat(ctx context.Context, req ChatRequest) (*ChatResponse, error) {
	p, err := m.Get(req.ProviderID)
	if err != nil {
		return nil, err
	}
	return p.Chat(ctx, req)
}

func (m *Manager) ChatStream(ctx context.Context, req ChatRequest) (<-chan StreamEvent, error) {
	p, err := m.Get(req.ProviderID)
	if err != nil {
		return nil, err
	}
	return p.ChatStream(ctx, req)
}

func (m *Manager) Completion(ctx context.Context, providerID string, req CompletionRequest) (*CompletionResponse, error) {
	p, err := m.Get(providerID)
	if err != nil {
		return nil, err
	}
	return p.Completion(ctx, req)
}

func (m *Manager) GetProviders() []ProviderInfo {
	m.mu.Lock()
	defer m.mu.Unlock()

	seen := make(map[string]bool)
	result := make([]ProviderInfo, 0)

	// Registered providers (includes dynamically created ones)
	for id, p := range m.providers {
		cfg := p.GetConfig()
		info := ProviderInfo{
			ID:        id,
			Name:      p.Name(),
			Endpoint:  cfg.Endpoint,
			Enabled:   cfg.Enabled,
			IsDefault: cfg.IsDefault,
		}
		ctx := m.appCtx()
		if ctx == nil {
			ctx = context.Background()
		}
		models, err := p.ListModels(ctx)
		if err == nil {
			info.Models = models
		}
		seen[id] = true
		result = append(result, info)
	}

	// Also include configs for providers not yet instantiated
	for id, cfg := range m.configs {
		if !seen[id] {
			info := ProviderInfo{
				ID:        id,
				Name:      cfg.Name,
				Endpoint:  cfg.Endpoint,
				Enabled:   cfg.Enabled,
				IsDefault: cfg.IsDefault,
			}
			result = append(result, info)
		}
	}
	return result
}

func (m *Manager) GetModels(providerID string) ([]Model, error) {
	p, err := m.Get(providerID)
	if err != nil {
		return nil, err
	}
	return p.ListModels(m.appCtx())
}

func (m *Manager) TestConnection(providerID string) error {
	p, err := m.Get(providerID)
	if err != nil {
		return err
	}
	return p.Validate(m.appCtx())
}

func (m *Manager) SetProviderConfig(providerID string, config ProviderConfig) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	p := m.getOrCreateProvider(providerID)

	// Auto-enable provider when API key or endpoint is configured
	if config.APIKey != "" || config.Endpoint != "" {
		config.Enabled = true
	}
	config.ID = providerID
	p.SetConfig(config)
	m.configs[providerID] = config
	return m.saveConfigs()
}

func (m *Manager) GetDefaultProvider() Provider {
	m.mu.Lock()
	defer m.mu.Unlock()

	priorities := []func(ProviderConfig) bool{
		func(cfg ProviderConfig) bool { return cfg.IsDefault && cfg.Enabled },
		func(cfg ProviderConfig) bool { return cfg.Enabled },
		func(cfg ProviderConfig) bool { return cfg.APIKey != "" },
	}
	for _, check := range priorities {
		for id, cfg := range m.configs {
			if check(cfg) {
				return m.getOrCreateProvider(id)
			}
		}
	}
	return nil
}
