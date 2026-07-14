package agent

import (
	"testing"
)

func TestRegistry_FindByIntent(t *testing.T) {
	reg := NewRegistry()
	reg.Register(AgentDef{ID: "assistant", Capabilities: []IntentType{IntentCodeEdit, IntentChat}, Priority: 0})
	reg.Register(AgentDef{ID: "frontend", Capabilities: []IntentType{IntentCodeEdit, IntentRefactor}, Priority: 10})

	result := reg.FindByIntent(IntentCodeEdit)
	if result != "frontend" {
		t.Errorf("expected frontend, got %s", result)
	}

	result = reg.FindByIntent(IntentChat)
	if result != "assistant" {
		t.Errorf("expected assistant, got %s", result)
	}

	result = reg.FindByIntent(IntentGit)
	if result != "" {
		t.Errorf("expected empty, got %s", result)
	}
}

func TestRegistry_SuggestAgents(t *testing.T) {
	reg := NewRegistry()
	reg.Register(AgentDef{ID: "a", Capabilities: []IntentType{IntentCodeEdit}, Priority: 5})
	reg.Register(AgentDef{ID: "b", Capabilities: []IntentType{IntentCodeEdit}, Priority: 10})
	reg.Register(AgentDef{ID: "c", Capabilities: []IntentType{IntentDebug}, Priority: 10})

	result := reg.SuggestAgents(IntentCodeEdit, 2)
	if len(result) != 2 {
		t.Fatalf("expected 2, got %d", len(result))
	}
	if result[0].ID != "b" {
		t.Errorf("expected b first, got %s", result[0].ID)
	}
}

func TestRegistry_SuggestAgents_Empty(t *testing.T) {
	reg := NewRegistry()
	reg.Register(AgentDef{ID: "a", Capabilities: []IntentType{IntentCodeEdit}, Priority: 5})

	result := reg.SuggestAgents(IntentGit, 3)
	if len(result) != 0 {
		t.Errorf("expected 0, got %d", len(result))
	}
}
