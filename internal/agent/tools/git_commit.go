package tools

import (
	"context"
	"fmt"
	"strings"

	"StarCore/internal/agent"
)

type GitCommitTool struct{}

func NewGitCommitTool() *GitCommitTool { return &GitCommitTool{} }

func (t *GitCommitTool) ID() string             { return "git_commit" }
func (t *GitCommitTool) Name() string           { return "Git Commit" }
func (t *GitCommitTool) RequiresApproval() bool { return true }

func (t *GitCommitTool) Description() string {
	return "暂存并提交代码变更。默认暂存所有文件，可用 files 参数指定特定文件。"
}

func (t *GitCommitTool) Parameters() agent.ToolParameters {
	return agent.ToolParameters{
		Type: "object",
		Properties: map[string]agent.ToolParamProp{
			"path":    {Type: "string", Description: "Repository path (default: current directory)"},
			"message": {Type: "string", Description: "Commit message (conventional commits format preferred)"},
			"files":   {Type: "string", Description: "Specific files to stage (space-separated, default: all tracked+new). Use 'git add' syntax."},
		},
		Required: []string{"message"},
	}
}

func (t *GitCommitTool) Execute(ctx context.Context, args map[string]any) (string, error) {
	path, _ := args["path"].(string)
	path = strings.TrimSpace(path)
	if path == "" {
		path = "."
	}
	message, ok := args["message"].(string)
	message = strings.TrimSpace(message)
	if !ok || message == "" {
		return "", fmt.Errorf("commit message is required")
	}

	// Stage files
	files, _ := args["files"].(string)
	if files != "" {
		for _, f := range strings.Fields(files) {
			out, err := runGitCmd(ctx, path, "git", "add", f)
			if err != nil {
				return "", fmt.Errorf("stage %s: %s", f, out)
			}
		}
	} else {
		if out, err := runGitCmd(ctx, path, "git", "add", "-A"); err != nil {
			return "", fmt.Errorf("stage all: %s", out)
		}
	}

	// Commit
	output, err := runGitCmd(ctx, path, "git", "commit", "-m", message)
	if err != nil {
		return "", fmt.Errorf("commit failed: %s", output)
	}
	return "Committed: " + strings.TrimSpace(output), nil
}

type GitPullTool struct{}

const defaultRemote = "origin"

func NewGitPullTool() *GitPullTool { return &GitPullTool{} }

func (t *GitPullTool) ID() string             { return "git_pull" }
func (t *GitPullTool) Name() string           { return "Git Pull" }
func (t *GitPullTool) RequiresApproval() bool { return true }

func (t *GitPullTool) Description() string {
	return "Pull latest changes from remote repository"
}

func (t *GitPullTool) Parameters() agent.ToolParameters {
	return agent.ToolParameters{
		Type: "object",
		Properties: map[string]agent.ToolParamProp{
			"path":   {Type: "string", Description: "Repository path (default: current directory)"},
			"remote": {Type: "string", Description: "Remote name (default: origin)"},
			"branch": {Type: "string", Description: "Branch name (default: current branch)"},
			"rebase": {Type: "boolean", Description: "Use rebase instead of merge (default: false)"},
		},
		Required: []string{},
	}
}

func (t *GitPullTool) Execute(ctx context.Context, args map[string]any) (string, error) {
	path, _ := args["path"].(string)
	path = strings.TrimSpace(path)
	if path == "" {
		path = "."
	}
	remote, _ := args["remote"].(string)
	remote = strings.TrimSpace(remote)
	if remote == "" {
		remote = defaultRemote
	}
	branch, _ := args["branch"].(string)
	branch = strings.TrimSpace(branch)

	var cmdArgs []string
	if branch != "" {
		cmdArgs = []string{"pull", remote, branch}
	} else {
		cmdArgs = []string{"pull"}
	}

	rebase, _ := args["rebase"].(bool)
	if rebase {
		cmdArgs = append([]string{"pull", "--rebase"}, cmdArgs[2:]...)
	}

	output, err := runGitCmd(ctx, path, "git", cmdArgs...)
	if err != nil {
		return "", fmt.Errorf("pull failed: %s", output)
	}
	return strings.TrimSpace(output), nil
}

type GitPushTool struct{}

func NewGitPushTool() *GitPushTool { return &GitPushTool{} }

func (t *GitPushTool) ID() string             { return "git_push" }
func (t *GitPushTool) Name() string           { return "Git Push" }
func (t *GitPushTool) RequiresApproval() bool { return true }

func (t *GitPushTool) Description() string {
	return "Push committed changes to remote repository"
}

func (t *GitPushTool) Parameters() agent.ToolParameters {
	return agent.ToolParameters{
		Type: "object",
		Properties: map[string]agent.ToolParamProp{
			"path":   {Type: "string", Description: "Repository path (default: current directory)"},
			"remote": {Type: "string", Description: "Remote name (default: origin)"},
			"branch": {Type: "string", Description: "Branch name (default: current branch)"},
			"tags":   {Type: "boolean", Description: "Also push tags (default: false)"},
		},
		Required: []string{},
	}
}

func (t *GitPushTool) Execute(ctx context.Context, args map[string]any) (string, error) {
	path, _ := args["path"].(string)
	path = strings.TrimSpace(path)
	if path == "" {
		path = "."
	}
	remote, _ := args["remote"].(string)
	remote = strings.TrimSpace(remote)
	if remote == "" {
		remote = defaultRemote
	}
	branch, _ := args["branch"].(string)
	branch = strings.TrimSpace(branch)

	cmdArgs := []string{"push"}
	tags, _ := args["tags"].(bool)
	if tags {
		cmdArgs = append(cmdArgs, "--tags")
	}
	if branch != "" {
		cmdArgs = append(cmdArgs, remote, branch)
	}

	output, err := runGitCmd(ctx, path, "git", cmdArgs...)
	if err != nil {
		return "", fmt.Errorf("push failed: %s", output)
	}
	return strings.TrimSpace(output), nil
}
