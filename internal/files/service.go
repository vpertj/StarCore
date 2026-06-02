package files

import (
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"

	"StarCore/internal/watcher"
)

// FileInfo represents a file or directory entry.
type FileInfo struct {
	Name  string `json:"name"`
	Path  string `json:"path"`
	IsDir bool   `json:"isDir"`
	Size  int64  `json:"size"`
	Mode  uint32 `json:"mode"`
}

// DiffHunk represents a single diff hunk.
type DiffHunk struct {
	OldStart int      `json:"oldStart"`
	OldCount int      `json:"oldCount"`
	NewStart int      `json:"newStart"`
	NewCount int      `json:"newCount"`
	OldLines []string `json:"oldLines"`
	NewLines []string `json:"newLines"`
}

// SearchResult represents a single search match.
type SearchResult struct {
	FilePath string `json:"filePath"`
	Line     int    `json:"line"`
	Content  string `json:"content"`
}

// SearchOptions configures a file search.
type SearchOptions struct {
	CaseSensitive  bool   `json:"caseSensitive"`
	WholeWord      bool   `json:"wholeWord"`
	UseRegex       bool   `json:"useRegex"`
	IncludePattern string `json:"includePattern"`
	ExcludePattern string `json:"excludePattern"`
}

// Service provides file operations, search, and diff functionality.
type Service struct{}

// NewService creates a new file service.
func NewService() *Service {
	return &Service{}
}

// ListDir lists files and directories in the given path.
func (s *Service) ListDir(path string) ([]FileInfo, error) {
	files, err := ioutil.ReadDir(path)
	if err != nil {
		return nil, err
	}
	result := make([]FileInfo, 0, len(files))
	for _, file := range files {
		if file.IsDir() && watcher.DefaultIgnoreDirs[file.Name()] {
			continue
		}
		if !file.IsDir() && strings.HasPrefix(file.Name(), ".") && file.Name() != ".gitignore" && file.Name() != ".env" {
			continue
		}
		result = append(result, FileInfo{
			Name:  file.Name(),
			Path:  filepath.Join(path, file.Name()),
			IsDir: file.IsDir(),
			Size:  file.Size(),
			Mode:  uint32(file.Mode()),
		})
	}
	return result, nil
}

// ReadFile reads a file and returns its content.
func (s *Service) ReadFile(path string) (string, error) {
	data, err := ioutil.ReadFile(path)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// WriteFile writes content to a file.
func (s *Service) WriteFile(path string, content string) error {
	return ioutil.WriteFile(path, []byte(content), 0644)
}

// CreateFile creates an empty file.
func (s *Service) CreateFile(path string) error {
	_, err := os.Create(path)
	return err
}

// DeleteFile deletes a file.
func (s *Service) DeleteFile(path string) error {
	return os.Remove(path)
}

// RenameFile renames a file or directory.
func (s *Service) RenameFile(oldPath, newPath string) error {
	return os.Rename(oldPath, newPath)
}

// CreateDir creates a new directory.
func (s *Service) CreateDir(path string) error {
	return os.Mkdir(path, 0755)
}

// ExecuteCommand runs a shell command and returns its output.
func (s *Service) ExecuteCommand(command string) (string, error) {
	var cmd *exec.Cmd
	if runtime.GOOS == "windows" {
		cmd = exec.Command("cmd", "/c", command)
	} else {
		cmd = exec.Command("sh", "-c", command)
	}
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("%s", string(output))
	}
	return strings.TrimSpace(string(output)), nil
}

// SearchFiles searches for a query in files.
func (s *Service) SearchFiles(query string, options SearchOptions) ([]SearchResult, error) {
	if rgPath, err := exec.LookPath("rg"); err == nil {
		return searchWithRipgrep(rgPath, query, options)
	}
	return searchWithWalk(query, options)
}

// ReplaceInFiles replaces all occurrences of query with replacement in files.
func (s *Service) ReplaceInFiles(query string, replacement string, options SearchOptions) error {
	projectPath := "."
	return filepath.Walk(projectPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if info.IsDir() {
			return nil
		}
		if options.IncludePattern != "" {
			matched, _ := filepath.Match(options.IncludePattern, info.Name())
			if !matched {
				return nil
			}
		}
		if options.ExcludePattern != "" {
			matched, _ := filepath.Match(options.ExcludePattern, info.Name())
			if matched {
				return nil
			}
		}
		content, err := ioutil.ReadFile(path)
		if err != nil {
			return nil
		}
		newContent := replaceInContent(string(content), query, replacement, options)
		if newContent != string(content) {
			return ioutil.WriteFile(path, []byte(newContent), info.Mode())
		}
		return nil
	})
}

// ApplyDiff applies a diff to a file.
func (s *Service) ApplyDiff(filePath string, hunks []DiffHunk) error {
	data, err := ioutil.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("failed to read file: %w", err)
	}
	lines := strings.Split(string(data), "\n")

	for i := len(hunks) - 1; i >= 0; i-- {
		hunk := hunks[i]
		startIdx := hunk.OldStart - 1
		if startIdx < 0 {
			startIdx = 0
		}
		endIdx := startIdx + hunk.OldCount
		if endIdx > len(lines) {
			endIdx = len(lines)
		}

		newLines := make([]string, len(lines[:startIdx]))
		copy(newLines, lines[:startIdx])
		newLines = append(newLines, hunk.NewLines...)
		newLines = append(newLines, lines[endIdx:]...)
		lines = newLines
	}

	newContent := strings.Join(lines, "\n")
	return ioutil.WriteFile(filePath, []byte(newContent), 0644)
}

// ComputeDiff computes a diff between disk content and new content.
func (s *Service) ComputeDiff(filePath string, newContent string) ([]DiffHunk, error) {
	data, err := ioutil.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	oldLines := strings.Split(string(data), "\n")
	newLines := strings.Split(newContent, "\n")

	return computeLineDiff(oldLines, newLines), nil
}

// ---- helper functions ----

func matchesQuery(line, query string, options SearchOptions) bool {
	if options.UseRegex {
		flags := "(?m)"
		if !options.CaseSensitive {
			flags += "i"
		}
		re, err := regexp.Compile(flags + query)
		if err != nil {
			return false
		}
		return re.MatchString(line)
	}
	if options.CaseSensitive {
		if options.WholeWord {
			return regexp.MustCompile(`\b`+regexp.QuoteMeta(query)+`\b`).MatchString(line)
		}
		return strings.Contains(line, query)
	}
	lineLower := strings.ToLower(line)
	queryLower := strings.ToLower(query)
	if options.WholeWord {
		return regexp.MustCompile(`\b`+regexp.QuoteMeta(queryLower)+`\b`).MatchString(lineLower)
	}
	return strings.Contains(lineLower, queryLower)
}

func replaceInContent(content, query, replacement string, options SearchOptions) string {
	if options.UseRegex {
		flags := "(?m)"
		if !options.CaseSensitive {
			flags += "i"
		}
		re, err := regexp.Compile(flags + query)
		if err != nil {
			return content
		}
		return re.ReplaceAllString(content, replacement)
	}
	if options.CaseSensitive {
		if options.WholeWord {
			re := regexp.MustCompile(`\b` + regexp.QuoteMeta(query) + `\b`)
			return re.ReplaceAllString(content, replacement)
		}
		return strings.ReplaceAll(content, query, replacement)
	}
	return strings.ReplaceAll(content, query, replacement)
}

func computeLineDiff(oldLines, newLines []string) []DiffHunk {
	var hunks []DiffHunk
	oi, ni := 0, 0

	for oi < len(oldLines) || ni < len(newLines) {
		for oi < len(oldLines) && ni < len(newLines) && oldLines[oi] == newLines[ni] {
			oi++
			ni++
		}

		if oi >= len(oldLines) && ni >= len(newLines) {
			break
		}

		diffStartOld := oi
		diffStartNew := ni

		matchOld := oi
		matchNew := ni
		found := false

		for mo := oi; mo < len(oldLines) && !found; mo++ {
			for mn := ni; mn < len(newLines); mn++ {
				if oldLines[mo] == newLines[mn] {
					matchOld = mo
					matchNew = mn
					found = true
					break
				}
			}
		}

		if !found {
			matchOld = len(oldLines)
			matchNew = len(newLines)
		}

		hunk := DiffHunk{
			OldStart: diffStartOld + 1,
			OldCount: matchOld - diffStartOld,
			NewStart: diffStartNew + 1,
			NewCount: matchNew - diffStartNew,
			OldLines: oldLines[diffStartOld:matchOld],
			NewLines: newLines[diffStartNew:matchNew],
		}
		hunks = append(hunks, hunk)

		oi = matchOld
		ni = matchNew
	}

	return hunks
}

func searchWithRipgrep(rgPath, query string, options SearchOptions) ([]SearchResult, error) {
	args := []string{"--no-heading", "--line-number", "--color", "never"}
	if !options.CaseSensitive {
		args = append(args, "-i")
	}
	if options.WholeWord {
		args = append(args, "-w")
	}
	if !options.UseRegex {
		args = append(args, "-F")
	}
	if options.IncludePattern != "" {
		args = append(args, "-g", options.IncludePattern)
	}
	args = append(args, "--", query, ".")

	cmd := exec.Command(rgPath, args...)
	output, err := cmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 1 {
			return nil, nil
		}
		return nil, fmt.Errorf("ripgrep: %w", err)
	}

	var results []SearchResult
	lines := strings.Split(string(output), "\n")
	for _, line := range lines {
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, ":", 3)
		if len(parts) >= 3 {
			lineNum := 0
			fmt.Sscanf(parts[1], "%d", &lineNum)
			results = append(results, SearchResult{
				FilePath: parts[0],
				Line:     lineNum,
				Content:  strings.TrimSpace(parts[2]),
			})
		}
	}
	return results, nil
}

func searchWithWalk(query string, options SearchOptions) ([]SearchResult, error) {
	var results []SearchResult
	err := filepath.Walk(".", func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if info.IsDir() {
			name := info.Name()
			if strings.HasPrefix(name, ".") || watcher.DefaultIgnoreDirs[name] {
				return filepath.SkipDir
			}
			return nil
		}
		if options.IncludePattern != "" {
			matched, _ := filepath.Match(options.IncludePattern, info.Name())
			if !matched {
				return nil
			}
		}
		if options.ExcludePattern != "" {
			matched, _ := filepath.Match(options.ExcludePattern, info.Name())
			if matched {
				return nil
			}
		}
		content, err := ioutil.ReadFile(path)
		if err != nil {
			return nil
		}
		lines := strings.Split(string(content), "\n")
		for i, line := range lines {
			if matchesQuery(line, query, options) {
				results = append(results, SearchResult{
					FilePath: path,
					Line:     i + 1,
					Content:  strings.TrimSpace(line),
				})
			}
		}
		return nil
	})
	return results, err
}
