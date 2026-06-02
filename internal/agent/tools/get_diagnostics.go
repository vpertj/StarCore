package tools

import (
	"context"
	"fmt"
	"os/exec"
	"runtime"
	"strings"

	"StarCore/internal/agent"
)

type GetDiagnosticsTool struct{}

func NewGetDiagnosticsTool() *GetDiagnosticsTool { return &GetDiagnosticsTool{} }

func (t *GetDiagnosticsTool) ID() string          { return "get_diagnostics" }
func (t *GetDiagnosticsTool) Name() string        { return "Get Diagnostics" }
func (t *GetDiagnosticsTool) RequiresApproval() bool { return false }

func (t *GetDiagnosticsTool) Description() string {
	return "Get current build/lint diagnostics for the project. Runs go vet for Go projects, or npm ls for Node.js projects."
}

func (t *GetDiagnosticsTool) Parameters() agent.ToolParameters {
	return agent.ToolParameters{
		Type: "object",
		Properties: map[string]agent.ToolParamProp{
			"project_path": {Type: "string", Description: "Project root path"},
		},
		Required: []string{"project_path"},
	}
}

func (t *GetDiagnosticsTool) Execute(ctx context.Context, args map[string]any) (string, error) {
	projectPath, _ := args["project_path"].(string)
	if projectPath == "" {
		return "[]", nil
	}

	var results []string

	goVetOut, err := runCmd(projectPath, "go", "vet", "./...")
	if err == nil && goVetOut != "" {
		results = append(results, "[Go vet]\n"+goVetOut)
	}

	npmOut, err := runCmd(projectPath, "npm", "run", "lint", "--", "--no-error-on-unmatched-pattern")
	if err == nil && npmOut != "" {
		results = append(results, "[npm lint]\n"+npmOut)
	}

	if len(results) == 0 {
		return "No diagnostics found. Project appears clean.", nil
	}

	return strings.Join(results, "\n\n"), nil
}

func runCmd(cwd string, name string, args ...string) (string, error) {
	var cmd *exec.Cmd
	if runtime.GOOS == "windows" {
		allArgs := append([]string{"/c", name}, args...)
		cmd = exec.Command("cmd", allArgs...)
	} else {
		cmd = exec.Command(name, args...)
	}
	cmd.Dir = cwd
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("command failed: %w", err)
	}
	result := strings.TrimSpace(string(out))
	if len(result) > 3000 {
		result = result[:3000] + "\n... [truncated]"
	}
	return result, nil
}
