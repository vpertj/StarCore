package agent

import "sync"

type Registry struct {
	agents map[string]AgentDef
	mu     sync.RWMutex
}

func NewRegistry() *Registry {
	return &Registry{
		agents: make(map[string]AgentDef),
	}
}

func (r *Registry) Register(agent AgentDef) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.agents[agent.ID] = agent
}

func (r *Registry) Get(id string) (AgentDef, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	a, ok := r.agents[id]
	return a, ok
}

func (r *Registry) List() []AgentDef {
	r.mu.RLock()
	defer r.mu.RUnlock()
	result := make([]AgentDef, 0, len(r.agents))
	for _, a := range r.agents {
		result = append(result, a)
	}
	return result
}

func (r *Registry) ListByCategory(category string) []AgentDef {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var result []AgentDef
	for _, a := range r.agents {
		if a.Category == category {
			result = append(result, a)
		}
	}
	return result
}
