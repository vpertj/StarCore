package skill

import "sync"

type Registry struct {
	skills map[string]SkillDef
	mu     sync.RWMutex
}

func NewRegistry() *Registry {
	return &Registry{
		skills: make(map[string]SkillDef),
	}
}

func (r *Registry) Register(s SkillDef) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.skills[s.ID] = s
}

func (r *Registry) Get(id string) (SkillDef, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	s, ok := r.skills[id]
	return s, ok
}

func (r *Registry) List() []SkillDef {
	r.mu.RLock()
	defer r.mu.RUnlock()
	result := make([]SkillDef, 0, len(r.skills))
	for _, s := range r.skills {
		result = append(result, s)
	}
	return result
}

func (r *Registry) ListByCategory(category string) []SkillDef {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var result []SkillDef
	for _, s := range r.skills {
		if s.Category == category {
			result = append(result, s)
		}
	}
	return result
}

func (r *Registry) ListByTrigger(trigger string) []SkillDef {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var result []SkillDef
	for _, s := range r.skills {
		if s.Trigger == trigger {
			result = append(result, s)
		}
	}
	return result
}
