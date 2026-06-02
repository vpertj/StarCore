package tools

import (
	"context"
	"fmt"
	"os/exec"
	"runtime"
	"strings"
	"time"
	"unicode/utf8"

	"StarCore/internal/agent"
)

const maxCommandOutput = 8000

type ExecuteCommandTool struct{}

func NewExecuteCommandTool() *ExecuteCommandTool { return &ExecuteCommandTool{} }

func (t *ExecuteCommandTool) ID() string             { return "execute_command" }
func (t *ExecuteCommandTool) Name() string           { return "Execute Command" }
func (t *ExecuteCommandTool) RequiresApproval() bool { return true }

func (t *ExecuteCommandTool) Description() string {
	return "Execute a shell command in the project directory and return its output. " +
		"Use for running tests, builds, git operations, or inspecting the environment. " +
		"Long outputs are automatically truncated. " +
		"Use cwd to specify a working directory (defaults to project root)."
}

func (t *ExecuteCommandTool) Parameters() agent.ToolParameters {
	return agent.ToolParameters{
		Type: "object",
		Properties: map[string]agent.ToolParamProp{
			"command":    {Type: "string", Description: "Shell command to execute"},
			"cwd":        {Type: "string", Description: "Working directory (optional, defaults to project root)"},
			"timeout_sec": {Type: "number", Description: "Command timeout in seconds (optional, default 30, max 120)"},
		},
		Required: []string{"command"},
	}
}

func (t *ExecuteCommandTool) Execute(ctx context.Context, args map[string]any) (string, error) {
	command, _ := args["command"].(string)
	if command == "" {
		return "", fmt.Errorf("command is required")
	}

	cwd, _ := args["cwd"].(string)
	if cwd == "" {
		cwd = "." // default to current directory (project root in dev mode)
	}

	timeoutSec := 30
	if v, ok := args["timeout_sec"].(float64); ok && v > 0 {
		timeoutSec = int(v)
		if timeoutSec > 120 {
			timeoutSec = 120
		}
	}

	execCtx, cancel := context.WithTimeout(ctx, time.Duration(timeoutSec)*time.Second)
	defer cancel()

	var cmd *exec.Cmd
	if runtime.GOOS == "windows" {
		cmd = exec.CommandContext(execCtx, "cmd", "/c", command)
	} else {
		cmd = exec.CommandContext(execCtx, "sh", "-c", command)
	}
	if cwd != "" {
		cmd.Dir = cwd
	}

	output, err := cmd.CombinedOutput()
	outStr := string(output)

	// Sanitize: replace null bytes and ensure valid UTF-8.
	outStr = strings.ReplaceAll(outStr, "\x00", "")
	if !utf8.ValidString(outStr) {
		outStr = string([]rune(outStr))
	}

	// Truncate long output.
	truncated := false
	if len(outStr) > maxCommandOutput {
		outStr = outStr[:maxCommandOutput]
		truncated = true
	}

	if execCtx.Err() == context.DeadlineExceeded {
		return "", fmt.Errorf("command timed out after %ds. Partial output:\n%s", timeoutSec, outStr)
	}

	if err != nil {
		return "", fmt.Errorf("exit code != 0. Output:\n%s", outStr)
	}

	outStr = strings.TrimSpace(outStr)
	if outStr == "" {
		outStr = "(no output)"
	}
	if truncated {
		outStr += fmt.Sprintf("\n... [output truncated at %d chars]", maxCommandOutput)
	}

	return outStr, nil
}
