package tools

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"StarCore/internal/agent"
)

type SearchFilesTool struct{}

func NewSearchFilesTool() *SearchFilesTool { return &SearchFilesTool{} }

func (t *SearchFilesTool) ID() string          { return "search_files" }
func (t *SearchFilesTool) Name() string        { return "Search Files" }
func (t *SearchFilesTool) RequiresApproval() bool { return false }

func (t *SearchFilesTool) Description() string {
	return "Search for a pattern in files. Supports regex, case sensitivity, and file pattern filters."
}

func (t *SearchFilesTool) Parameters() agent.ToolParameters {
	return agent.ToolParameters{
		Type: "object",
		Properties: map[string]agent.ToolParamProp{
			"query":           {Type: "string", Description: "Search query pattern"},
			"path":            {Type: "string", Description: "Root directory to search in (optional, defaults to .)"},
			"include_pattern": {Type: "string", Description: "File name glob pattern to include (optional)"},
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

	rootPath := "."
	if v, ok := args["path"].(string); ok && v != "" {
		rootPath = v
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

	type searchHit struct {
		FilePath string `json:"filePath"`
		Line     int    `json:"line"`
		Content  string `json:"content"`
	}

	var hits []searchHit

	err := filepath.Walk(rootPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if info.IsDir() {
			return nil
		}
		if includePattern != "" {
			matched, _ := filepath.Match(includePattern, info.Name())
			if !matched {
				return nil
			}
		}
		content, err := ioutil.ReadFile(path)
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
