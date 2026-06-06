package tools

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"StarCore/internal/agent"
)

// GlobTool finds files matching a glob pattern.
type GlobTool struct{}

func NewGlobTool() *GlobTool { return &GlobTool{} }

func (t *GlobTool) ID() string             { return "glob_files" }
func (t *GlobTool) Name() string           { return "Glob Files" }
func (t *GlobTool) RequiresApproval() bool { return false }

func (t *GlobTool) Description() string {
	return "Find files matching a glob pattern (e.g. \"src/**/*.go\", \"*.json\"). " +
		"Returns matching file paths sorted alphabetically. " +
		"Automatically ignores node_modules, .git, vendor, and other common dirs."
}

func (t *GlobTool) Parameters() agent.ToolParameters {
	return agent.ToolParameters{
		Type: "object",
		Properties: map[string]agent.ToolParamProp{
			"pattern": {Type: "string", Description: "Glob pattern to match (e.g. \"**/*.go\", \"src/**/*.tsx\")"},
		},
		Required: []string{"pattern"},
	}
}

var alwaysIgnore = map[string]bool{
	"node_modules": true, ".git": true, ".svn": true, ".hg": true,
	"vendor": true, "__pycache__": true, ".next": true, "dist": true,
	"build": true, "target": true, ".cache": true, ".turbo": true,
	"coverage": true, ".nyc_output": true,
}

func (t *GlobTool) Execute(ctx context.Context, args map[string]any) (string, error) {
	_ = ctx
	pattern, _ := args["pattern"].(string)
	if pattern == "" {
		return "", fmt.Errorf("pattern is required")
	}

	var matches []string
	err := filepath.Walk(".", func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if info.IsDir() {
			name := info.Name()
			if strings.HasPrefix(name, ".") && name != "." || alwaysIgnore[name] {
				return filepath.SkipDir
			}
			return nil
		}
		matched, _ := filepath.Match(pattern, filepath.Base(path))
		if !matched {
			matched, _ = filepath.Match(pattern, path)
		}
		if matched {
			matches = append(matches, path)
		}
		return nil
	})
	if err != nil {
		return "", err
	}

	sort.Strings(matches)

	const maxResults = 200
	if len(matches) > maxResults {
		total := len(matches)
		matches = matches[:maxResults]
		matches = append(matches, fmt.Sprintf("... and %d more matches", total-maxResults))
	}

	if len(matches) == 0 {
		return fmt.Sprintf("No files matched pattern: %s", pattern), nil
	}

	return strings.Join(matches, "\n"), nil
}
