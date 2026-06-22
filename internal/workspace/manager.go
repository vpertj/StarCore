package workspace

import (
	"os"
	"path/filepath"
	"sync"
)

type WorkspaceRoot struct {
	Path   string `json:"path"`
	Name   string `json:"name"`
	Active bool   `json:"active"`
}

type Manager struct {
	mu    sync.RWMutex
	roots []WorkspaceRoot
}

func NewManager() *Manager {
	return &Manager{}
}

func (m *Manager) AddRoot(path string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	abs, err := filepath.Abs(path)
	if err != nil {
		abs = path
	}

	for _, r := range m.roots {
		if r.Path == abs {
			return
		}
	}

	name := filepath.Base(abs)
	m.roots = append(m.roots, WorkspaceRoot{
		Path:   abs,
		Name:   name,
		Active: len(m.roots) == 0,
	})
}

func (m *Manager) RemoveRoot(path string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	abs, err := filepath.Abs(path)
	if err != nil {
		abs = path
	}

	for i, r := range m.roots {
		if r.Path == abs {
			m.roots = append(m.roots[:i], m.roots[i+1:]...)
			break
		}
	}

	if len(m.roots) > 0 {
		hasActive := false
		for i := range m.roots {
			if m.roots[i].Active {
				hasActive = true
				break
			}
		}
		if !hasActive {
			m.roots[0].Active = true
		}
	}
}

func (m *Manager) SetActive(path string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	abs, err := filepath.Abs(path)
	if err != nil {
		abs = path
	}

	for i := range m.roots {
		m.roots[i].Active = m.roots[i].Path == abs
	}
}

func (m *Manager) Roots() []WorkspaceRoot {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]WorkspaceRoot, len(m.roots))
	copy(result, m.roots)
	return result
}

func (m *Manager) ActiveRoot() *WorkspaceRoot {
	m.mu.RLock()
	defer m.mu.RUnlock()

	for i := range m.roots {
		if m.roots[i].Active {
			r := m.roots[i]
			return &r
		}
	}
	if len(m.roots) > 0 {
		r := m.roots[0]
		return &r
	}
	return nil
}

func (m *Manager) ActivePath() string {
	r := m.ActiveRoot()
	if r != nil {
		return r.Path
	}
	return ""
}

func (m *Manager) ResolvePath(relPath string) string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if filepath.IsAbs(relPath) {
		return relPath
	}

	for _, r := range m.roots {
		candidate := filepath.Join(r.Path, relPath)
		if _, err := os.Stat(candidate); err == nil {
			return candidate
		}
	}

	if len(m.roots) > 0 {
		return filepath.Join(m.roots[0].Path, relPath)
	}
	return relPath
}

func (m *Manager) FindRootForPath(filePath string) *WorkspaceRoot {
	m.mu.RLock()
	defer m.mu.RUnlock()

	abs, err := filepath.Abs(filePath)
	if err != nil {
		abs = filePath
	}

	var best *WorkspaceRoot
	bestLen := 0
	for i := range m.roots {
		rel, err := filepath.Rel(m.roots[i].Path, abs)
		if err != nil {
			continue
		}

		if len(rel) >= 2 && rel[:2] == ".." {
			continue
		}
		rootLen := len(m.roots[i].Path)
		if rootLen > bestLen {
			r := m.roots[i]
			best = &r
			bestLen = rootLen
		}
	}
	return best
}
