package extension

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

type Extension struct {
	ID          string                 `json:"id"`
	Name        string                 `json:"name"`
	Version     string                 `json:"version"`
	Description string                 `json:"description"`
	Author      string                 `json:"author"`
	EntryPoint  string                 `json:"entryPoint"`
	Enabled     bool                   `json:"enabled"`
	Commands    []CommandContribution  `json:"commands,omitempty"`
	Menus       map[string][]string    `json:"menus,omitempty"`
	Config      map[string]interface{} `json:"config,omitempty"`
}

type CommandContribution struct {
	ID       string `json:"id"`
	Label    string `json:"label"`
	Shortcut string `json:"shortcut,omitempty"`
	Category string `json:"category,omitempty"`
}

type Registry struct {
	mu         sync.RWMutex
	extensions map[string]*Extension
	configDir  string
}

func NewRegistry(configDir string) *Registry {
	return &Registry{
		extensions: make(map[string]*Extension),
		configDir:  configDir,
	}
}

func (r *Registry) Register(ext Extension) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if ext.ID == "" {
		return fmt.Errorf("extension ID is required")
	}
	if _, exists := r.extensions[ext.ID]; exists {
		return fmt.Errorf("extension %s already registered", ext.ID)
	}

	ext.Enabled = true
	r.extensions[ext.ID] = &ext
	return nil
}

func (r *Registry) Unregister(id string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.extensions, id)
}

func (r *Registry) Get(id string) *Extension {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.extensions[id]
}

func (r *Registry) List() []Extension {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make([]Extension, 0, len(r.extensions))
	for _, ext := range r.extensions {
		result = append(result, *ext)
	}
	return result
}

func (r *Registry) ListEnabled() []Extension {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var result []Extension
	for _, ext := range r.extensions {
		if ext.Enabled {
			result = append(result, *ext)
		}
	}
	return result
}

func (r *Registry) SetEnabled(id string, enabled bool) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	ext, ok := r.extensions[id]
	if !ok {
		return fmt.Errorf("extension %s not found", id)
	}
	ext.Enabled = enabled
	return nil
}

func (r *Registry) GetCommands() []CommandContribution {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var cmds []CommandContribution
	for _, ext := range r.extensions {
		if ext.Enabled {
			cmds = append(cmds, ext.Commands...)
		}
	}
	return cmds
}

func (r *Registry) LoadFromDir(dir string) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		manifestPath := filepath.Join(dir, entry.Name(), "extension.json")
		data, err := os.ReadFile(manifestPath)
		if err != nil {
			continue
		}

		var ext Extension
		if json.Unmarshal(data, &ext) != nil {
			continue
		}
		if ext.ID == "" {
			ext.ID = entry.Name()
		}
		ext.EntryPoint = filepath.Join(dir, entry.Name(), ext.EntryPoint)

		r.mu.Lock()
		r.extensions[ext.ID] = &ext
		r.mu.Unlock()
	}

	return nil
}

func (r *Registry) SaveConfig() error {
	r.mu.RLock()
	defer r.mu.RUnlock()

	dir := filepath.Join(r.configDir, "extensions")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("create extensions dir: %w", err)
	}

	configPath := filepath.Join(dir, "config.json")
	list := r.List()
	data, err := json.MarshalIndent(list, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(configPath, data, 0644)
}

func (r *Registry) LoadConfig() error {
	configPath := filepath.Join(r.configDir, "extensions", "config.json")
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil
	}

	var list []Extension
	if json.Unmarshal(data, &list) != nil {
		return nil
	}

	r.mu.Lock()
	for i := range list {
		r.extensions[list[i].ID] = &list[i]
	}
	r.mu.Unlock()

	return nil
}
