package tools

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	"StarCore/internal/agent"
)

type SearchFilesTool struct{}

func NewSearchFilesTool() *SearchFilesTool { return &SearchFilesTool{} }

func (t *SearchFilesTool) ID() string             { return "search_files" }
func (t *SearchFilesTool) Name() string           { return "Search Files" }
func (t *SearchFilesTool) RequiresApproval() bool { return false }

func (t *SearchFilesTool) Description() string {
	return "Search for a pattern in files. Supports regex, case sensitivity, and file pattern filters. Uses ripgrep (rg) when available for fast search."
}

func (t *SearchFilesTool) Parameters() agent.ToolParameters {
	return agent.ToolParameters{
		Type: "object",
		Properties: map[string]agent.ToolParamProp{
			"query":           {Type: "string", Description: "Search query pattern"},
			"path":            {Type: "string", Description: "Root directory to search in (optional, defaults to .)"},
			"include_pattern": {Type: "string", Description: "File name glob pattern to include (optional, e.g. '*.go')"},
			"case_sensitive":  {Type: "boolean", Description: "Case sensitive search (optional, default false)"},
			"use_regex":       {Type: "boolean", Description: "Treat query as regex (optional, default false)"},
		},
		Required: []string{"query"},
	}
}

func (t *SearchFilesTool) Execute(ctx context.Context, args map[string]any) (string, error) {
	query, ok := args["query"].(string)
	if !ok || query == "" {
		return "", fmt.Errorf("search_files requires a 'query' parameter - the text to search for. Example: {\"query\": \"func main\"}")
	}
	query = strings.TrimSpace(query)

	rootPath := "."
	if v, ok := args["path"].(string); ok && v != "" {
		rootPath = strings.TrimSpace(v)
	}

	var includePattern string
	if v, ok := args["include_pattern"].(string); ok {
		includePattern = v
	}

	caseSensitive := false
	if v, ok := args["case_sensitive"].(bool); ok {
		caseSensitive = v
	}

	useRegex := false
	if v, ok := args["use_regex"].(bool); ok {
		useRegex = v
	}

	// Try ripgrep first, fallback to Go implementation
	if result, err := searchWithRipgrep(ctx, query, rootPath, includePattern, caseSensitive, useRegex); err == nil {
		return result, nil
	}

	return searchWithGo(ctx, query, rootPath, includePattern, caseSensitive, useRegex)
}

// searchWithRipgrep uses the rg command for fast search.
func searchWithRipgrep(ctx context.Context, query, rootPath, includePattern string, caseSensitive, useRegex bool) (string, error) {
	rgPath, err := exec.LookPath("rg")
	if err != nil {
		return "", fmt.Errorf("rg not found")
	}

	args := []string{
		"--line-number",
		"--max-count=200",
		"--max-filesize=1M",
	}

	if !caseSensitive {
		args = append(args, "-i")
	}

	if !useRegex {
		args = append(args, "--fixed-strings")
	}

	if includePattern != "" {
		args = append(args, "--glob", includePattern)
	}

	// Skip common non-code directories
	args = append(args, "--glob", "!{.git,node_modules,vendor,__pycache__,.svn,.hg,dist,build,target,.next,.cache}")

	args = append(args, query, rootPath)

	cmd := exec.CommandContext(ctx, rgPath, args...)

	output, err := cmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			// rg returns exit code 1 when no matches found
			if len(exitErr.Stderr) == 0 || strings.Contains(string(exitErr.Stderr), "no matches") {
				return "No results found", nil
			}
		}
		return "", err
	}

	result := strings.TrimRight(string(output), "\n\r")
	if result == "" {
		return "No results found", nil
	}

	lines := strings.Split(result, "\n")
	if len(lines) > 200 {
		result = strings.Join(lines[:200], "\n") + fmt.Sprintf("\n... and %d more results (use a more specific query to narrow down)", len(lines)-200)
	}

	return result, nil
}

// searchWithGo is a pure Go fallback when rg is not available.
func searchWithGo(ctx context.Context, query, rootPath, includePattern string, caseSensitive, useRegex bool) (string, error) {
	_ = ctx
	type searchHit struct {
		FilePath string
		Line     int
		Content  string
	}

	var hits []searchHit

	err := filepath.Walk(rootPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if info.IsDir() {
			name := info.Name()
			if (strings.HasPrefix(name, ".") && name != ".") || alwaysIgnore[name] {
				return filepath.SkipDir
			}
			return nil
		}
		if info.Size() > 1<<20 {
			return nil
		}
		if includePattern != "" {
			matched, _ := filepath.Match(includePattern, info.Name())
			if !matched {
				return nil
			}
		}
		content, err := os.ReadFile(path)
		if err != nil {
			return nil
		}
		lines := strings.Split(string(content), "\n")
		for i, line := range lines {
			if searchMatches(line, query, caseSensitive, useRegex) {
				hits = append(hits, searchHit{
					FilePath: path,
					Line:     i + 1,
					Content:  strings.TrimSpace(line),
				})
				if len(hits) >= 200 {
					return filepath.SkipAll
				}
			}
		}
		return nil
	})
	if err != nil {
		return "", err
	}

	if len(hits) == 0 {
		return "No results found", nil
	}

	var sb strings.Builder
	for _, h := range hits {
		sb.WriteString(fmt.Sprintf("%s:%d: %s\n", h.FilePath, h.Line, h.Content))
	}
	return sb.String(), nil
}

func searchMatches(line, query string, caseSensitive, useRegex bool) bool {
	if useRegex {
		flags := "(?m)"
		if !caseSensitive {
			flags += "i"
		}
		re, err := regexp.Compile(flags + query)
		if err != nil {
			return false
		}
		return re.MatchString(line)
	}
	if caseSensitive {
		return strings.Contains(line, query)
	}
	return strings.Contains(strings.ToLower(line), strings.ToLower(query))
}
