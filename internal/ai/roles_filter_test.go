package ai

import (
	"testing"

	"StarCore/internal/agent"
)

func TestFilterToolsByRole_Coder(t *testing.T) {
	s := &Service{}
	allTools := []string{"read_file", "write_file", "edit_file", "execute_command", "search_files", "sub_agent"}
	agentDef := agent.AgentDef{
		ID:    "coder-agent",
		Roles: []agent.AgentRole{agent.RoleCoder},
	}

	filtered := s.filterToolsByRole(allTools, agentDef)
	// Coder should have all tools
	if len(filtered) != len(allTools) {
		t.Errorf("Coder should keep all tools, got %d of %d: %v", len(filtered), len(allTools), filtered)
	}
}

func TestFilterToolsByRole_Architect(t *testing.T) {
	s := &Service{}
	allTools := []string{"read_file", "write_file", "edit_file", "execute_command", "search_files", "sub_agent"}
	agentDef := agent.AgentDef{
		ID:    "architect-agent",
		Roles: []agent.AgentRole{agent.RoleArchitect},
	}

	filtered := s.filterToolsByRole(allTools, agentDef)
	// Architect should only have read tools
	for _, tool := range filtered {
		cat := agent.GetToolCategory(tool)
		if cat != agent.CategoryRead {
			t.Errorf("Architect should not have tool %s (category %v)", tool, cat)
		}
	}
	// Verify it actually filtered some out
	if len(filtered) >= len(allTools) {
		t.Error("Architect should have fewer tools than the full list")
	}
	// read_file and search_files should remain
	hasRead := false
	hasSearch := false
	for _, tool := range filtered {
		if tool == "read_file" {
			hasRead = true
		}
		if tool == "search_files" {
			hasSearch = true
		}
	}
	if !hasRead {
		t.Error("Architect should keep read_file")
	}
	if !hasSearch {
		t.Error("Architect should keep search_files")
	}
}

func TestFilterToolsByRole_Reviewer(t *testing.T) {
	s := &Service{}
	allTools := []string{"read_file", "write_file", "edit_file", "execute_command", "search_files", "glob_files"}
	agentDef := agent.AgentDef{
		ID:    "reviewer-agent",
		Roles: []agent.AgentRole{agent.RoleReviewer},
	}

	filtered := s.filterToolsByRole(allTools, agentDef)
	// Reviewer should only have read tools
	for _, tool := range filtered {
		cat := agent.GetToolCategory(tool)
		if cat != agent.CategoryRead {
			t.Errorf("Reviewer should not have tool %s (category %v)", tool, cat)
		}
	}
}

func TestFilterToolsByRole_NoRoles(t *testing.T) {
	s := &Service{}
	allTools := []string{"read_file", "write_file", "execute_command"}
	agentDef := agent.AgentDef{
		ID: "no-role-agent",
	}

	// No roles → return original list
	filtered := s.filterToolsByRole(allTools, agentDef)
	if len(filtered) != len(allTools) {
		t.Errorf("Agent with no roles should keep all tools, got %d of %d", len(filtered), len(allTools))
	}
}

func TestFilterToolsByRole_ExplicitTools(t *testing.T) {
	s := &Service{}
	// Agent with explicit tools gets those filtered by role
	explicitTools := []string{"read_file", "write_file", "search_files"}
	agentDef := agent.AgentDef{
		ID:    "explicit-agent",
		Tools: explicitTools,
		Roles: []agent.AgentRole{agent.RoleReviewer},
	}

	filtered := s.filterToolsByRole(explicitTools, agentDef)
	// Only read tools should remain
	for _, tool := range filtered {
		if tool == "write_file" {
			t.Error("Reviewer should not have write_file even in explicit tools")
		}
	}
}
