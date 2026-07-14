package agent

import (
	"testing"
)

func TestGetRoleConfig(t *testing.T) {
	cfg, ok := GetRoleConfig(RoleCoder)
	if !ok {
		t.Fatal("RoleCoder should exist")
	}
	if cfg.Name != "程序员" {
		t.Errorf("expected role name 程序员, got %s", cfg.Name)
	}
	if !cfg.CanModify {
		t.Error("Coder should be able to modify files")
	}
	if !cfg.CanExecute {
		t.Error("Coder should be able to execute commands")
	}
}

func TestGetRoleConfig_Architect(t *testing.T) {
	cfg, ok := GetRoleConfig(RoleArchitect)
	if !ok {
		t.Fatal("RoleArchitect should exist")
	}
	if cfg.CanModify {
		t.Error("Architect should NOT be able to modify files")
	}
	if cfg.CanExecute {
		t.Error("Architect should NOT be able to execute commands")
	}
}

func TestGetRoleConfig_Reviewer(t *testing.T) {
	cfg, ok := GetRoleConfig(RoleReviewer)
	if !ok {
		t.Fatal("RoleReviewer should exist")
	}
	if cfg.CanModify {
		t.Error("Reviewer should NOT be able to modify files")
	}
	if cfg.CanExecute {
		t.Error("Reviewer should NOT be able to execute commands")
	}
}

func TestGetToolCategory(t *testing.T) {
	tests := []struct {
		toolID   string
		expected ToolCategory
	}{
		{"read_file", CategoryRead},
		{"glob_files", CategoryRead},
		{"search_files", CategoryRead},
		{"write_file", CategoryWrite},
		{"edit_file", CategoryWrite},
		{"delete_file", CategoryWrite},
		{"execute_command", CategoryExecute},
		{"http_request", CategoryExecute},
		{"unknown_tool", CategoryExecute}, // unknown = most restrictive
	}

	for _, tt := range tests {
		cat := GetToolCategory(tt.toolID)
		if cat != tt.expected {
			t.Errorf("GetToolCategory(%q) = %v, want %v", tt.toolID, cat, tt.expected)
		}
	}
}

func TestCanRoleUseTool(t *testing.T) {
	// Coder can use everything
	if !CanRoleUseTool(RoleCoder, "read_file") {
		t.Error("Coder should be able to read files")
	}
	if !CanRoleUseTool(RoleCoder, "write_file") {
		t.Error("Coder should be able to write files")
	}
	if !CanRoleUseTool(RoleCoder, "execute_command") {
		t.Error("Coder should be able to execute commands")
	}

	// Architect: read-only
	if !CanRoleUseTool(RoleArchitect, "read_file") {
		t.Error("Architect should be able to read files")
	}
	if CanRoleUseTool(RoleArchitect, "write_file") {
		t.Error("Architect should NOT be able to write files")
	}
	if CanRoleUseTool(RoleArchitect, "execute_command") {
		t.Error("Architect should NOT be able to execute commands")
	}

	// Reviewer: read-only
	if !CanRoleUseTool(RoleReviewer, "read_file") {
		t.Error("Reviewer should be able to read files")
	}
	if CanRoleUseTool(RoleReviewer, "write_file") {
		t.Error("Reviewer should NOT be able to write files")
	}
	if CanRoleUseTool(RoleReviewer, "execute_command") {
		t.Error("Reviewer should NOT be able to execute commands")
	}

	// System: everything
	if !CanRoleUseTool(RoleSystem, "read_file") {
		t.Error("System should be able to read files")
	}
	if !CanRoleUseTool(RoleSystem, "write_file") {
		t.Error("System should be able to write files")
	}
	if !CanRoleUseTool(RoleSystem, "execute_command") {
		t.Error("System should be able to execute commands")
	}
}

func TestToolsForRole(t *testing.T) {
	// Coder should have the most tools
	coderTools := ToolsForRole(RoleCoder)
	if len(coderTools) == 0 {
		t.Error("Coder should have tools")
	}

	// Architect should have fewer tools (read-only)
	architectTools := ToolsForRole(RoleArchitect)
	if len(architectTools) >= len(coderTools) {
		t.Errorf("Architect should have fewer tools than Coder: %d vs %d", len(architectTools), len(coderTools))
	}

	// Reviewer same as Architect (read-only)
	reviewerTools := ToolsForRole(RoleReviewer)
	if len(reviewerTools) != len(architectTools) {
		t.Errorf("Reviewer should have same tools as Architect: %d vs %d", len(reviewerTools), len(architectTools))
	}

	// Verify Architect can't write
	for _, toolID := range architectTools {
		if GetToolCategory(toolID) == CategoryWrite || GetToolCategory(toolID) == CategoryExecute {
			t.Errorf("Architect should not have write/exec tool: %s", toolID)
		}
	}
}

func TestAllRoles(t *testing.T) {
	roles := AllRoles()
	if len(roles) < 4 {
		t.Errorf("expected at least 4 roles, got %d", len(roles))
	}

	// Check all expected roles exist
	expected := map[AgentRole]bool{RoleArchitect: false, RoleCoder: false, RoleReviewer: false, RoleSystem: false}
	for _, r := range roles {
		if _, ok := expected[r]; ok {
			expected[r] = true
		}
	}
	for role, found := range expected {
		if !found {
			t.Errorf("expected role %s not found in AllRoles()", role)
		}
	}
}

func TestParseRole(t *testing.T) {
	role, ok := ParseRole("coder")
	if !ok || role != RoleCoder {
		t.Errorf("ParseRole('coder') = %v, %v; want RoleCoder, true", role, ok)
	}

	role, ok = ParseRole("ARCHITECT")
	if !ok || role != RoleArchitect {
		t.Errorf("ParseRole('ARCHITECT') = %v, %v; want RoleArchitect, true", role, ok)
	}

	_, ok = ParseRole("invalid-role")
	if ok {
		t.Error("ParseRole should return false for invalid role")
	}
}

func TestIsValidRole(t *testing.T) {
	if !IsValidRole("coder") {
		t.Error("'coder' should be valid")
	}
	if !IsValidRole("reviewer") {
		t.Error("'reviewer' should be valid")
	}
	if IsValidRole("superadmin") {
		t.Error("'superadmin' should not be valid")
	}
}

// --- AgentDef extensions ---

func TestAgentDef_IsRoleEnabled(t *testing.T) {
	// No Roles specified → all roles enabled
	a1 := AgentDef{ID: "test1"}
	if !a1.IsRoleEnabled(RoleCoder) {
		t.Error("should enable all roles when none specified")
	}

	// Specific roles → only those enabled
	a2 := AgentDef{ID: "test2", Roles: []AgentRole{RoleArchitect}}
	if !a2.IsRoleEnabled(RoleArchitect) {
		t.Error("Architect should be enabled")
	}
	if a2.IsRoleEnabled(RoleCoder) {
		t.Error("Coder should not be enabled")
	}
}

func TestAgentDef_GetSystemHint(t *testing.T) {
	a := AgentDef{ID: "test", Roles: []AgentRole{RoleArchitect}}
	// GetSystemHint returns the global system hint for any valid role
	hint := a.GetSystemHint(RoleArchitect)
	if hint == "" {
		t.Error("RoleArchitect should have a system hint")
	}

	// Invalid roles return empty hint
	hint = a.GetSystemHint("invalid-role")
	if hint != "" {
		t.Error("Should return empty hint for invalid role")
	}
}

func TestAgentDef_HasTool(t *testing.T) {
	// Explicit tools → check explicit list only
	a1 := AgentDef{
		ID:    "test1",
		Tools: []string{"read_file", "search_files"},
		Roles: []AgentRole{RoleCoder},
	}
	if !a1.HasTool("read_file") {
		t.Error("should have explicit tool")
	}
	if a1.HasTool("write_file") {
		t.Error("should not have tool not in explicit list")
	}

	// No explicit tools → use roles
	a2 := AgentDef{
		ID:    "test2",
		Roles: []AgentRole{RoleReviewer},
	}
	if !a2.HasTool("read_file") {
		t.Error("Reviewer should have read_file via role")
	}
	if a2.HasTool("write_file") {
		t.Error("Reviewer should NOT have write_file")
	}

	// No tools, no roles → unrestricted
	a3 := AgentDef{ID: "test3"}
	if !a3.HasTool("anything") {
		t.Error("unrestricted agent should have any tool")
	}
}

func TestAgentDef_HasTool_MultipleRoles(t *testing.T) {
	a := AgentDef{
		ID:    "multi-role",
		Roles: []AgentRole{RoleArchitect, RoleReviewer},
	}
	// Both roles are read-only, so write should still be blocked
	if a.HasTool("write_file") {
		t.Error("multi-role read-only should not have write_file")
	}
	if !a.HasTool("read_file") {
		t.Error("multi-role should have read_file")
	}
}

func TestRoleSystemHint_NotEmpty(t *testing.T) {
	for _, role := range []AgentRole{RoleArchitect, RoleCoder, RoleReviewer} {
		cfg, _ := GetRoleConfig(role)
		if cfg.SystemHint == "" {
			t.Errorf("role %s should have a non-empty system hint", role)
		}
	}
}
