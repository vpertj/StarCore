package tools

import (
	"context"
	"fmt"
	"os/exec"
	"runtime"
	"strings"

	"StarCore/internal/agent"
)

type GetGitDiffTool struct{}

func NewGetGitDiffTool() *GetGitDiffTool { return &GetGitDiffTool{} }

func (t *GetGitDiffTool) ID() string             { return "get_git_diff" }
func (t *GetGitDiffTool) Name() string           { return "Get Git Info" }
func (t *GetGitDiffTool) RequiresApproval() bool { return false }

func (t *GetGitDiffTool) Description() string {
	return "获取 git 信息：diff（变更差异）、status（文件状态）、log（最近提交）。"
}

func (t *GetGitDiffTool) Parameters() agent.ToolParameters {
	return agent.ToolParameters{
		Type: "object",
		Properties: map[string]agent.ToolParamProp{
			"path":   {Type: "string", Description: "Repository path"},
			"action": {Type: "string", Description: "One of: diff, status, log (default: status)"},
		},
		Required: []string{"path"},
	}
}

func (t *GetGitDiffTool) Execute(ctx context.Context, args map[string]any) (string, error) {
	path, _ := args["path"].(string)
	path = strings.TrimSpace(path)
	if path == "" {
		path = "."
	}
	action := "status"
	if v, ok := args["action"].(string); ok && v != "" {
		action = strings.TrimSpace(v)
	}

	switch action {
	case "diff":
		return runGitCmd(ctx, path, "git", "diff")
	case "status":
		return runGitCmd(ctx, path, "git", "status", "--short")
	case "log":
		return runGitCmd(ctx, path, "git", "log", "--oneline", "-20")
	default:
		return runGitCmd(ctx, path, "git", "status", "--short")
	}
}

func runGitCmd(ctx context.Context, cwd string, name string, args ...string) (string, error) {
	var cmd *exec.Cmd
	if runtime.GOOS == "windows" {
		allArgs := append([]string{"/c", name}, args...)
		cmd = exec.CommandContext(ctx, "cmd", allArgs...)
	} else {
		cmd = exec.CommandContext(ctx, name, args...)
	}
	cmd.Dir = cwd

	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("git command failed: %s", strings.TrimSpace(string(output)))
	}

	result := strings.TrimSpace(string(output))
	if result == "" {
		return "No output (clean working tree or no diff)", nil
	}
	if len(result) > 5000 {
		result = result[:5000] + "\n... [truncated]"
	}
	return result, nil
}
