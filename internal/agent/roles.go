package agent

import "strings"

// --- Agent Roles & Permission Matrix ---
//
// Each agent has a Role that determines which tools it can access.
// The permission matrix is hard-coded (not LLM-decided) for security:
//
//   Architect  → read-only      (analysis, planning, understanding)
//   Coder      → read-write     (implementation, editing, execution)
//   Reviewer   → review-only    (audit, suggestions, no modifications)
//
// Tools are categorized by their impact level. A role can access
// tools at or below its permission level.

// AgentRole defines the type of agent.
type AgentRole string

const (
	RoleArchitect AgentRole = "architect" // read-only: analysis, planning
	RoleCoder     AgentRole = "coder"     // read-write: full implementation
	RoleReviewer  AgentRole = "reviewer"  // review-only: audit, suggestions
	RoleSystem    AgentRole = "system"    // internal: no restrictions (default)
)

// ToolCategory classifies a tool by its impact level.
type ToolCategory int

const (
	CategoryRead    ToolCategory = iota // read-only, no side effects
	CategoryWrite                       // modifies files/state
	CategoryExecute                     // runs commands
)

// RoleConfig defines what a role can do.
type RoleConfig struct {
	Role        AgentRole
	Name        string
	Description string
	Icon        string
	Categories  []ToolCategory // allowed categories
	CanModify   bool           // can write/edit/delete files
	CanExecute  bool           // can run arbitrary commands
	SystemHint  string         // appended to system prompt
}

// roleConfigs is the hard-coded permission matrix.
var roleConfigs = map[AgentRole]RoleConfig{
	RoleArchitect: {
		Role:        RoleArchitect,
		Name:        "架构师",
		Description: "专注于分析、设计和规划。只读访问，不做修改。",
		Icon:        "🔍",
		Categories:  []ToolCategory{CategoryRead},
		CanModify:   false,
		CanExecute:  false,
		SystemHint:  "你是一个架构师角色。专注于分析代码结构、设计解决方案、制定实现计划。你不能修改或创建文件，只能读取和分析。输出清晰的架构建议和实现路线图。",
	},
	RoleCoder: {
		Role:        RoleCoder,
		Name:        "程序员",
		Description: "专注于实现和编码。完整的读写和执行权限。",
		Icon:        "💻",
		Categories:  []ToolCategory{CategoryRead, CategoryWrite, CategoryExecute},
		CanModify:   true,
		CanExecute:  true,
		SystemHint:  "你是一个程序员角色。专注于实现功能、修复 bug、编写高质量代码。你可以读写文件、执行命令。每次修改后验证结果。",
	},
	RoleReviewer: {
		Role:        RoleReviewer,
		Name:        "审查员",
		Description: "专注于代码审查和质量审计。只读 + 审查意见。",
		Icon:        "🔎",
		Categories:  []ToolCategory{CategoryRead},
		CanModify:   false,
		CanExecute:  false,
		SystemHint:  "你是一个代码审查员角色。专注于发现代码问题、安全漏洞、性能隐患。你不能修改文件，只能读取和分析。输出具体的问题位置、严重程度评分、修复建议。",
	},
	RoleSystem: {
		Role:        RoleSystem,
		Name:        "系统",
		Description: "内部系统角色，无权限限制。",
		Icon:        "⚙️",
		Categories:  []ToolCategory{CategoryRead, CategoryWrite, CategoryExecute},
		CanModify:   true,
		CanExecute:  true,
		SystemHint:  "",
	},
}

// toolCategories maps tool IDs to their category.
// This is the hard-coded permission mapping.
var toolCategories = map[string]ToolCategory{
	// Read-only tools
	"read_file":       CategoryRead,
	"glob_files":      CategoryRead,
	"search_files":    CategoryRead,
	"list_directory":  CategoryRead,
	"get_diagnostics": CategoryRead,
	"get_git_diff":    CategoryRead,
	"get_git_status":  CategoryRead,
	"lsp_definition":  CategoryRead,
	"lsp_references":  CategoryRead,
	"lsp_symbols":     CategoryRead,
	"web_fetch":       CategoryRead,
	"todo_read":       CategoryRead,
	"syntax_check":    CategoryRead,
	"todo_write":      CategoryRead, // todo is agent-scoped, not file modification

	// Write tools (modify files)
	"write_file":       CategoryWrite,
	"edit_file":        CategoryWrite,
	"multi_edit":       CategoryWrite,
	"create_directory": CategoryWrite,
	"delete_file":      CategoryWrite,
	"move_file":        CategoryWrite,
	"git_commit":       CategoryWrite,
	"git_push":         CategoryWrite,
	"git_pull":         CategoryWrite,
	"skill":            CategoryWrite, // skills can have side effects

	// Execute tools
	"execute_command": CategoryExecute,
	"http_request":    CategoryExecute,
	"sub_agent":       CategoryExecute,
}

// GetRoleConfig returns the config for a role.
func GetRoleConfig(role AgentRole) (RoleConfig, bool) {
	cfg, ok := roleConfigs[role]
	return cfg, ok
}

// GetToolCategory returns the category of a tool.
func GetToolCategory(toolID string) ToolCategory {
	if cat, ok := toolCategories[toolID]; ok {
		return cat
	}
	// Unknown tools default to Execute (most restrictive for non-Coder)
	return CategoryExecute
}

// CanRoleUseTool checks if a role can use a specific tool.
func CanRoleUseTool(role AgentRole, toolID string) bool {
	cfg, ok := roleConfigs[role]
	if !ok {
		return false
	}
	cat := GetToolCategory(toolID)
	for _, allowed := range cfg.Categories {
		if allowed == cat {
			return true
		}
	}
	return false
}

// ToolsForRole returns the list of tool IDs accessible to a role.
func ToolsForRole(role AgentRole) []string {
	cfg, ok := roleConfigs[role]
	if !ok {
		return nil
	}

	var tools []string
	for toolID := range toolCategories {
		cat := GetToolCategory(toolID)
		for _, allowed := range cfg.Categories {
			if allowed == cat {
				tools = append(tools, toolID)
				break
			}
		}
	}
	return tools
}

// AllRoles returns all registered roles.
func AllRoles() []AgentRole {
	roles := make([]AgentRole, 0, len(roleConfigs))
	for r := range roleConfigs {
		roles = append(roles, r)
	}
	return roles
}

// IsValidRole checks if a role string is valid.
func IsValidRole(roleStr string) bool {
	_, ok := roleConfigs[AgentRole(strings.ToLower(roleStr))]
	return ok
}

// ParseRole parses a role string (case-insensitive).
func ParseRole(roleStr string) (AgentRole, bool) {
	role := AgentRole(strings.ToLower(roleStr))
	_, ok := roleConfigs[role]
	return role, ok
}

// --- AgentDef extensions ---

// IsRoleEnabled checks if an agent definition supports a role.
// If the agent has no Roles specified, all roles are available.
func (a AgentDef) IsRoleEnabled(role AgentRole) bool {
	if len(a.Roles) == 0 {
		return true
	}
	for _, r := range a.Roles {
		if r == role {
			return true
		}
	}
	return false
}

// GetSystemHint returns the system hint for a role.
func (a AgentDef) GetSystemHint(role AgentRole) string {
	cfg, ok := roleConfigs[role]
	if !ok {
		return ""
	}
	return cfg.SystemHint
}

// HasTool checks if an agent has access to a specific tool.
func (a AgentDef) HasTool(toolID string) bool {
	// If agent specifies explicit tools, check that list
	if len(a.Tools) > 0 {
		for _, t := range a.Tools {
			if t == toolID {
				return true
			}
		}
		return false
	}
	// No explicit tool list → role determines access
	if len(a.Roles) > 0 {
		for _, role := range a.Roles {
			if CanRoleUseTool(role, toolID) {
				return true
			}
		}
		return false
	}
	// No tools and no roles → unrestricted
	return true
}
