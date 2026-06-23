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
		"Supports ** for recursive directory matching. " +
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
	pattern = strings.TrimSpace(pattern)
	if pattern == "" {
		return "", fmt.Errorf("pattern is required")
	}

	// Normalize path separators
	pattern = filepath.ToSlash(pattern)

	var matches []string
	err := filepath.Walk(".", func(path string, info os.FileInfo, err error) error {
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

		// Normalize path for matching
		slashPath := filepath.ToSlash(path)

		if matchGlob(pattern, slashPath) {
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

// matchGlob matches a path against a glob pattern that supports **.
func matchGlob(pattern, path string) bool {
	// If no **, use simple matching
	if !strings.Contains(pattern, "**") {
		// Try matching against basename
		matched, _ := filepath.Match(pattern, filepath.Base(path))
		if matched {
			return true
		}
		// Try matching against full path
		matched, _ = filepath.Match(pattern, path)
		return matched
	}

	// Handle ** patterns by splitting into parts
	return matchDoublestar(pattern, path)
}

// matchDoublestar handles patterns with ** for recursive matching.
func matchDoublestar(pattern, path string) bool {
	// Split pattern on **
	parts := strings.Split(pattern, "**")

	if len(parts) == 2 {
		prefix := strings.TrimSuffix(parts[0], "/")
		suffix := strings.TrimPrefix(parts[1], "/")

		// Match prefix against the beginning of the path
		if prefix != "" {
			prefixParts := strings.Split(prefix, "/")
			pathParts := strings.Split(path, "/")

			if len(pathParts) < len(prefixParts) {
				return false
			}

			for i, pp := range prefixParts {
				matched, _ := filepath.Match(pp, pathParts[i])
				if !matched {
					return false
				}
			}

			// The rest of the path after prefix
			rest := strings.Join(pathParts[len(prefixParts):], "/")
			return matchSuffix(suffix, rest)
		}

		// No prefix, just match suffix anywhere
		return matchSuffix(suffix, path)
	}

	// Multiple ** segments — match each segment in order
	return matchMultiDoublestar(parts, path)
}

// matchSuffix matches a suffix pattern against the remaining path.
func matchSuffix(suffix, path string) bool {
	if suffix == "" {
		return true
	}

	// Try to match the suffix as a glob against the path
	matched, _ := filepath.Match(suffix, filepath.Base(path))
	if matched {
		return true
	}

	// Try matching against the full remaining path
	matched, _ = filepath.Match(suffix, path)
	if matched {
		return true
	}

	// Try matching suffix parts against path parts
	suffixParts := strings.Split(suffix, "/")
	pathParts := strings.Split(path, "/")

	if len(suffixParts) > len(pathParts) {
		return false
	}

	// Try aligning suffix at the end
	start := len(pathParts) - len(suffixParts)
	for i, sp := range suffixParts {
		matched, _ := filepath.Match(sp, pathParts[start+i])
		if !matched {
			return false
		}
	}
	return true
}

// matchMultiDoublestar handles patterns with multiple ** segments.
func matchMultiDoublestar(parts []string, path string) bool {
	pathParts := strings.Split(path, "/")

	// Try to match each part in order
	return matchParts(parts, 0, pathParts, 0)
}

// matchParts recursively matches pattern parts against path parts.
func matchParts(patternParts []string, pi int, pathParts []string, pp int) bool {
	if pi == len(patternParts) {
		return pp == len(pathParts)
	}

	part := patternParts[pi]
	if part == "" {
		// Empty segment (from **/** or leading/trailing **)
		return matchParts(patternParts, pi+1, pathParts, pp)
	}

	if pi == len(patternParts)-1 {
		// Last pattern part — match against remaining path
		remaining := strings.Join(pathParts[pp:], "/")
		matched, _ := filepath.Match(part, remaining)
		if matched {
			return true
		}
		// Try matching against each remaining path component
		for i := pp; i < len(pathParts); i++ {
			matched, _ := filepath.Match(part, pathParts[i])
			if matched {
				return matchParts(patternParts, pi+1, pathParts, i+1)
			}
		}
		return false
	}

	// Not the last part — try consuming path parts
	for i := pp; i < len(pathParts); i++ {
		matched, _ := filepath.Match(part, pathParts[i])
		if matched {
			if matchParts(patternParts, pi+1, pathParts, i+1) {
				return true
			}
		}
	}
	return false
}
