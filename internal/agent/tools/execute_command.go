package tools

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
	"unicode/utf8"

	"StarCore/internal/agent"
	"StarCore/internal/sandbox"
)

const maxCommandOutput = 8000

var SandboxConfig *sandbox.Config

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
			"command":     {Type: "string", Description: "Shell command to execute"},
			"cwd":         {Type: "string", Description: "Working directory (optional, defaults to project root)"},
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
		if LoopStateRef != nil {
			if paths := LoopStateRef.GetFilesTouched(); len(paths) > 0 {
				cwd = filepath.Dir(paths[0])
			}
		}
	}
	if cwd == "" {
		cwd = "."
	}

	if SandboxConfig != nil {
		if err := SandboxConfig.ValidateCommand(command, cwd); err != nil {
			return "", fmt.Errorf("sandbox: %w", err)
		}
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

	// Stream stdout and stderr simultaneously
	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		return "", fmt.Errorf("failed to create stdout pipe: %w", err)
	}
	stderrPipe, err := cmd.StderrPipe()
	if err != nil {
		return "", fmt.Errorf("failed to create stderr pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return "", fmt.Errorf("failed to start command: %w", err)
	}

	// Read both streams concurrently
	var stdoutBuf, stderrBuf bytes.Buffer
	doneCh := make(chan struct{}, 2)

	go func() {
		io.Copy(&stdoutBuf, stdoutPipe)
		doneCh <- struct{}{}
	}()
	go func() {
		io.Copy(&stderrBuf, stderrPipe)
		doneCh <- struct{}{}
	}()

	// Wait for both streams to finish
	<-doneCh
	<-doneCh

	// Wait for process to exit
	waitErr := cmd.Wait()

	// Combine output: stdout first, then stderr
	var combined strings.Builder
	if stdoutBuf.Len() > 0 {
		combined.Write(stdoutBuf.Bytes())
	}
	if stderrBuf.Len() > 0 {
		if combined.Len() > 0 {
			combined.WriteByte('\n')
		}
		combined.WriteString("[stderr]\n")
		combined.Write(stderrBuf.Bytes())
	}

	outStr := combined.String()

	// Sanitize
	outStr = strings.ReplaceAll(outStr, "\x00", "")
	if !utf8.ValidString(outStr) {
		outStr = string([]rune(outStr))
	}

	// Smart truncation: keep head + tail for long output
	if len(outStr) > maxCommandOutput {
		headSize := maxCommandOutput * 3 / 4
		tailSize := maxCommandOutput / 4
		outStr = outStr[:headSize] + fmt.Sprintf("\n... [%d chars omitted] ...\n", len(outStr)-headSize-tailSize) + outStr[len(outStr)-tailSize:]
	}

	if execCtx.Err() == context.DeadlineExceeded {
		return "", fmt.Errorf("command timed out after %ds. Partial output:\n%s", timeoutSec, outStr)
	}

	if waitErr != nil {
		if outStr == "" {
			return "", fmt.Errorf("exit code != 0: %v", waitErr)
		}
		return "", fmt.Errorf("exit code != 0. Output:\n%s", outStr)
	}

	outStr = strings.TrimSpace(outStr)
	if outStr == "" {
		outStr = "(no output)"
	}

	return outStr, nil
}
