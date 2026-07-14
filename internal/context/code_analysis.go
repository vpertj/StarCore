package context

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// CodeStructure holds extracted code elements from a file.
type CodeStructure struct {
	FilePath   string   `json:"filePath"`
	Language   string   `json:"language"`
	Imports    []string `json:"imports,omitempty"`
	Functions  []string `json:"functions,omitempty"`
	Types      []string `json:"types,omitempty"`
	Interfaces []string `json:"interfaces,omitempty"`
	Constants  []string `json:"constants,omitempty"`
	Lines      int      `json:"lines"`
}

// ExtractCodeStructure extracts code structure from a file using regex-based analysis.
func ExtractCodeStructure(filePath string) (*CodeStructure, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, err
	}

	content := string(data)
	ext := strings.ToLower(filepath.Ext(filePath))
	lang := extToLanguage(ext)

	structure := &CodeStructure{
		FilePath: filePath,
		Language: lang,
		Lines:    strings.Count(content, "\n") + 1,
	}

	switch lang {
	case "go":
		extractGoStructure(content, structure)
	case "javascript", "typescript":
		extractJSStructure(content, structure)
	case "python":
		extractPythonStructure(content, structure)
	}

	return structure, nil
}

func extToLanguage(ext string) string {
	switch ext {
	case ".go":
		return "go"
	case ".js", ".jsx", ".mjs":
		return "javascript"
	case ".ts", ".tsx":
		return "typescript"
	case ".py":
		return "python"
	case ".rs":
		return "rust"
	case ".java":
		return "java"
	default:
		return "unknown"
	}
}

// extractGoStructure extracts Go code elements.
func extractGoStructure(content string, s *CodeStructure) {
	// Imports
	importRe := regexp.MustCompile(`(?m)^\s*import\s*\(?(.*?)\)?\s*$`)
	matches := importRe.FindAllStringSubmatch(content, -1)
	for _, m := range matches {
		if len(m) > 1 {
			lines := strings.Split(m[1], "\n")
			for _, line := range lines {
				line = strings.TrimSpace(line)
				if line == "" || line == "import" || line == "(" || line == ")" {
					continue
				}
				// Extract package path from import line
				pkgRe := regexp.MustCompile(`"([^"]+)"`)
				if pkgMatch := pkgRe.FindStringSubmatch(line); len(pkgMatch) > 1 {
					s.Imports = append(s.Imports, pkgMatch[1])
				}
			}
		}
	}

	// Functions
	funcRe := regexp.MustCompile(`(?m)^func\s+(?:\([^)]+\)\s+)?(\w+)\s*\(`)
	for _, m := range funcRe.FindAllStringSubmatch(content, -1) {
		if len(m) > 1 && m[1] != "main" && m[1] != "init" {
			s.Functions = append(s.Functions, m[1])
		}
	}

	// Types
	typeRe := regexp.MustCompile(`(?m)^type\s+(\w+)\s+(struct|interface)`)
	for _, m := range typeRe.FindAllStringSubmatch(content, -1) {
		if len(m) > 2 {
			if m[2] == "struct" {
				s.Types = append(s.Types, m[1])
			} else {
				s.Interfaces = append(s.Interfaces, m[1])
			}
		}
	}

	// Constants
	constRe := regexp.MustCompile(`(?m)^const\s*\(?`)
	_ = constRe
	// Simple const detection
	simpleConstRe := regexp.MustCompile(`(?m)^const\s+(\w+)`)
	for _, m := range simpleConstRe.FindAllStringSubmatch(content, -1) {
		if len(m) > 1 {
			s.Constants = append(s.Constants, m[1])
		}
	}
}

// extractJSStructure extracts JavaScript/TypeScript code elements.
func extractJSStructure(content string, s *CodeStructure) {
	// Imports
	importRe := regexp.MustCompile(`(?m)^import\s+.*?from\s+['"]([^'"]+)['"]`)
	for _, m := range importRe.FindAllStringSubmatch(content, -1) {
		if len(m) > 1 {
			s.Imports = append(s.Imports, m[1])
		}
	}
	// Require imports
	requireRe := regexp.MustCompile(`(?:require|import)\s*\(\s*['"]([^'"]+)['"]\s*\)`)
	for _, m := range requireRe.FindAllStringSubmatch(content, -1) {
		if len(m) > 1 {
			s.Imports = append(s.Imports, m[1])
		}
	}

	// Functions
	funcRe := regexp.MustCompile(`(?m)^(?:export\s+)?(?:async\s+)?function\s+(\w+)`)
	for _, m := range funcRe.FindAllStringSubmatch(content, -1) {
		if len(m) > 1 {
			s.Functions = append(s.Functions, m[1])
		}
	}
	// Arrow functions assigned to const/let/var
	arrowRe := regexp.MustCompile(`(?m)^(?:export\s+)?(?:const|let|var)\s+(\w+)\s*=\s*(?:async\s+)?\(`)
	for _, m := range arrowRe.FindAllStringSubmatch(content, -1) {
		if len(m) > 1 {
			s.Functions = append(s.Functions, m[1])
		}
	}

	// Classes and types
	classRe := regexp.MustCompile(`(?m)^(?:export\s+)?(?:class|interface|type)\s+(\w+)`)
	for _, m := range classRe.FindAllStringSubmatch(content, -1) {
		if len(m) > 1 {
			s.Types = append(s.Types, m[1])
		}
	}
}

// extractPythonStructure extracts Python code elements.
func extractPythonStructure(content string, s *CodeStructure) {
	// Imports
	importRe := regexp.MustCompile(`(?m)^(?:from\s+(\S+)\s+)?import\s+(.+)`)
	for _, m := range importRe.FindAllStringSubmatch(content, -1) {
		if len(m) > 1 {
			if m[1] != "" {
				s.Imports = append(s.Imports, m[1])
			} else {
				s.Imports = append(s.Imports, strings.TrimSpace(m[2]))
			}
		}
	}

	// Functions
	funcRe := regexp.MustCompile(`(?m)^def\s+(\w+)\s*\(`)
	for _, m := range funcRe.FindAllStringSubmatch(content, -1) {
		if len(m) > 1 && m[1] != "__init__" {
			s.Functions = append(s.Functions, m[1])
		}
	}

	// Classes
	classRe := regexp.MustCompile(`(?m)^class\s+(\w+)`)
	for _, m := range classRe.FindAllStringSubmatch(content, -1) {
		if len(m) > 1 {
			s.Types = append(s.Types, m[1])
		}
	}
}

// AnalyzeProjectStructure performs deep code analysis on a project.
func AnalyzeProjectStructure(projectPath string, maxFiles int) []*CodeStructure {
	var structures []*CodeStructure
	fileCount := 0

	filepath.Walk(projectPath, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}
		if fileCount >= maxFiles {
			return filepath.SkipDir
		}

		name := info.Name()
		if strings.HasPrefix(name, ".") || name == "node_modules" || name == "vendor" || name == "__pycache__" {
			return nil
		}

		ext := strings.ToLower(filepath.Ext(path))
		if ext == ".go" || ext == ".js" || ext == ".ts" || ext == ".jsx" || ext == ".tsx" || ext == ".py" {
			s, err := ExtractCodeStructure(path)
			if err == nil {
				structures = append(structures, s)
				fileCount++
			}
		}
		return nil
	})

	return structures
}

// BuildDependencyGraph builds a simple dependency graph from code structures.
func BuildDependencyGraph(structures []*CodeStructure) map[string][]string {
	graph := make(map[string][]string)

	for _, s := range structures {
		if len(s.Imports) > 0 {
			graph[s.FilePath] = s.Imports
		}
	}

	return graph
}

// FindDependencyChain finds the dependency chain from one file to another.
func FindDependencyChain(graph map[string][]string, from, to string) []string {
	visited := make(map[string]bool)
	var path []string

	var dfs func(current string) bool
	dfs = func(current string) bool {
		if current == to {
			path = append(path, current)
			return true
		}
		if visited[current] {
			return false
		}
		visited[current] = true
		path = append(path, current)

		for _, dep := range graph[current] {
			if dfs(dep) {
				return true
			}
		}
		path = path[:len(path)-1]
		return false
	}

	dfs(from)
	return path
}

// GetCodeSummary returns a human-readable summary of code structures.
func GetCodeSummary(structures []*CodeStructure) string {
	if len(structures) == 0 {
		return "No code files analyzed."
	}

	var buf strings.Builder
	buf.WriteString(fmt.Sprintf("Project Code Analysis (%d files):\n\n", len(structures)))

	// Group by language
	byLang := make(map[string][]*CodeStructure)
	for _, s := range structures {
		byLang[s.Language] = append(byLang[s.Language], s)
	}

	for lang, files := range byLang {
		totalFuncs := 0
		totalTypes := 0
		for _, f := range files {
			totalFuncs += len(f.Functions)
			totalTypes += len(f.Types) + len(f.Interfaces)
		}
		buf.WriteString(fmt.Sprintf("%s: %d files, %d functions, %d types\n", lang, len(files), totalFuncs, totalTypes))
	}

	// Top-level exports (functions and types)
	var allFuncs []string
	var allTypes []string
	for _, s := range structures {
		allFuncs = append(allFuncs, s.Functions...)
		allTypes = append(allTypes, s.Types...)
		allTypes = append(allTypes, s.Interfaces...)
	}

	if len(allFuncs) > 0 {
		sort.Strings(allFuncs)
		uniqueFuncs := dedup(allFuncs)
		if len(uniqueFuncs) > 20 {
			uniqueFuncs = uniqueFuncs[:20]
			buf.WriteString(fmt.Sprintf("\nFunctions (top 20 of %d): %s\n", len(allFuncs), strings.Join(uniqueFuncs, ", ")))
		} else {
			buf.WriteString(fmt.Sprintf("\nFunctions: %s\n", strings.Join(uniqueFuncs, ", ")))
		}
	}

	if len(allTypes) > 0 {
		sort.Strings(allTypes)
		uniqueTypes := dedup(allTypes)
		if len(uniqueTypes) > 20 {
			uniqueTypes = uniqueTypes[:20]
			buf.WriteString(fmt.Sprintf("Types (top 20 of %d): %s\n", len(allTypes), strings.Join(uniqueTypes, ", ")))
		} else {
			buf.WriteString(fmt.Sprintf("Types: %s\n", strings.Join(uniqueTypes, ", ")))
		}
	}

	return buf.String()
}

func dedup(s []string) []string {
	seen := make(map[string]bool)
	var result []string
	for _, v := range s {
		if !seen[v] {
			seen[v] = true
			result = append(result, v)
		}
	}
	return result
}
