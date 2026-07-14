package context

import (
	"os"
	"path/filepath"
	"strings"
)

type ContextSuggestion struct {
	FilePath string
	Reason   string
	Score    float64
}

type ContextSuggester struct {
	projectPath string
}

func NewContextSuggester(projectPath string) *ContextSuggester {
	return &ContextSuggester{projectPath: projectPath}
}

func (s *ContextSuggester) Suggest(activeFile string, queryContext string, maxSuggestions int, existingFiles []string) []ContextSuggestion {
	if activeFile == "" || s.projectPath == "" {
		return nil
	}

	existing := make(map[string]bool)
	for _, f := range existingFiles {
		existing[f] = true
	}
	existing[activeFile] = true

	var suggestions []ContextSuggestion
	seen := make(map[string]bool)
	for f := range existing {
		seen[f] = true
	}

	sameDir := s.suggestSameDirectory(activeFile, seen)
	for _, s := range sameDir {
		if !seen[s.FilePath] {
			seen[s.FilePath] = true
			suggestions = append(suggestions, s)
		}
	}

	deps := s.suggestDependencies(activeFile, seen)
	for _, d := range deps {
		if !seen[d.FilePath] {
			seen[d.FilePath] = true
			suggestions = append(suggestions, d)
		}
	}

	if len(suggestions) > maxSuggestions {
		suggestions = suggestions[:maxSuggestions]
	}

	return suggestions
}

func (s *ContextSuggester) suggestSameDirectory(activeFile string, seen map[string]bool) []ContextSuggestion {
	dir := filepath.Dir(activeFile)
	ext := filepath.Ext(activeFile)
	if ext == "" {
		return nil
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}

	var suggestions []ContextSuggestion
	baseName := filepath.Base(activeFile)

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if name == baseName {
			continue
		}
		if strings.HasSuffix(name, "_test"+ext) != strings.HasSuffix(baseName, "_test"+ext) {
			continue
		}
		fileExt := filepath.Ext(name)
		if fileExt != ext {
			continue
		}
		fullPath := filepath.Join(dir, name)
		if seen[fullPath] {
			continue
		}
		suggestions = append(suggestions, ContextSuggestion{
			FilePath: fullPath,
			Reason:   "同目录相关文件",
			Score:    0.8,
		})
		if len(suggestions) >= 3 {
			break
		}
	}

	return suggestions
}

func (s *ContextSuggester) suggestDependencies(activeFile string, seen map[string]bool) []ContextSuggestion {
	structures := AnalyzeProjectStructure(s.projectPath, 50)
	if len(structures) == 0 {
		return nil
	}

	var activeImports []string
	for _, s := range structures {
		if s.FilePath == activeFile {
			activeImports = s.Imports
			break
		}
	}
	if len(activeImports) == 0 {
		return nil
	}

	fileMap := make(map[string]string)
	for _, s := range structures {
		fileMap[s.FilePath] = s.FilePath
		fileMap[filepath.Base(s.FilePath)] = s.FilePath
	}

	var suggestions []ContextSuggestion
	for _, imp := range activeImports {
		if fp, ok := fileMap[imp]; ok && !seen[fp] {
			suggestions = append(suggestions, ContextSuggestion{
				FilePath: fp,
				Reason:   "依赖文件",
				Score:    0.6,
			})
			if len(suggestions) >= 3 {
				break
			}
		}
	}

	return suggestions
}
