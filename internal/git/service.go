package git

import (
	"fmt"
	"os/exec"
	"runtime"
	"strings"
)

const maxDiffChars = 8000

// StatusEntry describes the status of a single file in a git repo.
type StatusEntry struct {
	Path    string `json:"path"`
	Status  string `json:"status"`
	Staged  bool   `json:"staged"`
	Added   bool   `json:"added"`
	Deleted bool   `json:"deleted"`
	Renamed bool   `json:"renamed"`
}

// LogEntry describes a single git commit.
type LogEntry struct {
	Hash    string `json:"hash"`
	Message string `json:"message"`
	Author  string `json:"author"`
	Date    string `json:"date"`
}

// Service provides git operations via the git CLI.
type Service struct{}

// NewService creates a new git service.
func NewService() *Service {
	return &Service{}
}

// runGit executes a git command in the specified working directory.
func runGit(cwd string, args ...string) (string, error) {
	var cmd *exec.Cmd
	if runtime.GOOS == "windows" {
		allArgs := append([]string{"/c", "git"}, args...)
		cmd = exec.Command("cmd", allArgs...)
	} else {
		cmd = exec.Command("git", args...)
	}
	cmd.Dir = cwd
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("git %s: %s", strings.Join(args, " "), strings.TrimSpace(string(out)))
	}
	return strings.TrimSpace(string(out)), nil
}

// Status returns the working tree status.
func (s *Service) Status(projectPath string) ([]StatusEntry, error) {
	out, err := runGit(projectPath, "status", "--porcelain")
	if err != nil {
		if strings.Contains(err.Error(), "not a git repository") {
			return nil, nil
		}
		return nil, err
	}
	if out == "" {
		return nil, nil
	}
	lines := strings.Split(out, "\n")
	entries := make([]StatusEntry, 0, len(lines))
	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			continue
		}
		if len(line) < 3 {
			continue
		}
		entry := StatusEntry{}
		x := line[0:1]
		y := line[1:2]
		path := strings.TrimSpace(line[3:])
		entry.Path = path
		entry.Staged = x != " " && x != "?"
		entry.Added = x == "A" || y == "A"
		entry.Deleted = x == "D" || y == "D"
		entry.Renamed = x == "R" || y == "R"
		switch {
		case x == "?":
			entry.Status = "untracked"
		case x == "M" || y == "M":
			entry.Status = "modified"
		case x == "A" || y == "A":
			entry.Status = "added"
		case x == "D" || y == "D":
			entry.Status = "deleted"
		case x == "R" || y == "R":
			entry.Status = "renamed"
		default:
			entry.Status = "modified"
		}
		entries = append(entries, entry)
	}
	return entries, nil
}

// Branch returns the current branch name.
func (s *Service) Branch(projectPath string) (string, error) {
	out, err := runGit(projectPath, "branch", "--show-current")
	if err != nil {
		return "", nil
	}
	return out, nil
}

// Log returns recent commit history.
func (s *Service) Log(projectPath string, count int) ([]LogEntry, error) {
	if count < 1 || count > 100 {
		count = 20
	}
	format := "--format=%H|%s|%an|%ar"
	out, err := runGit(projectPath, "log", fmt.Sprintf("-%d", count), format)
	if err != nil {
		return nil, err
	}
	if out == "" {
		return nil, nil
	}
	lines := strings.Split(out, "\n")
	entries := make([]LogEntry, 0, len(lines))
	for _, line := range lines {
		parts := strings.SplitN(line, "|", 4)
		if len(parts) >= 2 {
			hash := parts[0]
			if len(hash) > 8 {
				hash = hash[:8]
			}
			entry := LogEntry{Hash: hash, Message: parts[1]}
			if len(parts) > 2 {
				entry.Author = parts[2]
			}
			if len(parts) > 3 {
				entry.Date = parts[3]
			}
			entries = append(entries, entry)
		}
	}
	return entries, nil
}

// Commit creates a commit with the given message.
func (s *Service) Commit(projectPath string, message string) error {
	_, err := runGit(projectPath, "commit", "-m", message)
	return err
}

// Stage stages a file for commit.
func (s *Service) Stage(projectPath string, filePath string) error {
	_, err := runGit(projectPath, "add", filePath)
	return err
}

// Unstage unstages a file.
func (s *Service) Unstage(projectPath string, filePath string) error {
	_, err := runGit(projectPath, "reset", "HEAD", "--", filePath)
	return err
}

// Diff returns the working tree diff.
func (s *Service) Diff(projectPath string, filePath string) (string, error) {
	args := []string{"diff"}
	if filePath != "" {
		args = append(args, "--", filePath)
	}
	out, err := runGit(projectPath, args...)
	if err != nil {
		return "", err
	}
	if out == "" {
		return "No changes", nil
	}
	if len(out) > 5000 {
		out = out[:maxDiffChars] + "\n... [truncated]"
	}
	return out, nil
}

// StatusAndBranch returns combined status and branch info.
func (s *Service) StatusAndBranch(projectPath string) (map[string]interface{}, error) {
	branch, _ := s.Branch(projectPath)
	status, err := s.Status(projectPath)
	if err != nil {
		return nil, err
	}
	return map[string]interface{}{
		"branch":   branch,
		"status":   status,
		"modified": len(status),
	}, nil
}

// Pull pulls from the remote.
func (s *Service) Pull(projectPath string) (string, error) {
	return runGit(projectPath, "pull")
}

// Push pushes to the remote.
func (s *Service) Push(projectPath string) (string, error) {
	return runGit(projectPath, "push")
}

// Fetch fetches from the remote.
func (s *Service) Fetch(projectPath string) (string, error) {
	return runGit(projectPath, "fetch")
}

// Checkout switches to a branch.
func (s *Service) Checkout(projectPath string, branch string) (string, error) {
	return runGit(projectPath, "checkout", branch)
}

// CreateBranch creates a new branch.
func (s *Service) CreateBranch(projectPath string, branch string) (string, error) {
	return runGit(projectPath, "branch", branch)
}

// Merge merges a branch into the current branch.
func (s *Service) Merge(projectPath string, branch string) (string, error) {
	return runGit(projectPath, "merge", branch)
}

// Stash manages git stash operations.
func (s *Service) Stash(projectPath string, action string) (string, error) {
	if action == "pop" || action == "apply" || action == "list" || action == "drop" {
		return runGit(projectPath, "stash", action)
	}
	return runGit(projectPath, "stash")
}

type BlameLine struct {
	Hash    string `json:"hash"`
	Author  string `json:"author"`
	Date    string `json:"date"`
	Line    int    `json:"line"`
	Content string `json:"content"`
}

func (s *Service) Blame(projectPath string, filePath string) ([]BlameLine, error) {
	out, err := runGit(projectPath, "blame", "--porcelain", filePath)
	if err != nil {
		return nil, err
	}
	return parseBlame(out), nil
}

func parseBlame(output string) []BlameLine {
	var lines []BlameLine
	currentHash := ""
	currentAuthor := ""
	currentDate := ""
	lineNum := 0

	for _, line := range strings.Split(output, "\n") {
		if len(line) == 0 {
			continue
		}
		if line[0] >= '0' && line[0] <= '9' || line[0] >= 'a' && line[0] <= 'f' {
			parts := strings.SplitN(line, " ", 4)
			if len(parts) >= 3 {
				currentHash = parts[0]
				fmt.Sscanf(parts[2], "%d", &lineNum)
			}
		} else if strings.HasPrefix(line, "author ") {
			currentAuthor = strings.TrimPrefix(line, "author ")
		} else if strings.HasPrefix(line, "author-time ") {
			ts := strings.TrimPrefix(line, "author-time ")
			if t, err := fmt.Sscanf(ts, "%d", new(int)); err == nil && t == 1 {
				currentDate = ts
			}
		} else if strings.HasPrefix(line, "\t") {
			lines = append(lines, BlameLine{
				Hash:    currentHash[:min(len(currentHash), 8)],
				Author:  currentAuthor,
				Date:    currentDate,
				Line:    lineNum,
				Content: strings.TrimPrefix(line, "\t"),
			})
		}
	}
	return lines
}

func (s *Service) VisualDiff(projectPath string, filePath string) (string, error) {
	return runGit(projectPath, "diff", "--unified=5", "--", filePath)
}

func (s *Service) DiffBetween(projectPath string, from string, to string, filePath string) (string, error) {
	return runGit(projectPath, "diff", "--unified=5", from, to, "--", filePath)
}

func (s *Service) RemoteList(projectPath string) (string, error) {
	return runGit(projectPath, "remote", "-v")
}

func (s *Service) FetchRemote(projectPath string, remote string) (string, error) {
	if remote == "" {
		remote = "origin"
	}
	return runGit(projectPath, "fetch", remote)
}

func (s *Service) LogFile(projectPath string, filePath string, count int) ([]LogEntry, error) {
	if count <= 0 {
		count = 20
	}
	out, err := runGit(projectPath, "log", fmt.Sprintf("-%d", count), "--format=%H|%s|%an|%ai", "--", filePath)
	if err != nil {
		return nil, err
	}
	return parseLogEntries(out), nil
}

func parseLogEntries(output string) []LogEntry {
	if output == "" {
		return nil
	}
	var entries []LogEntry
	for _, line := range strings.Split(output, "\n") {
		parts := strings.SplitN(line, "|", 4)
		if len(parts) < 2 {
			continue
		}
		hash := parts[0]
		if len(hash) > 8 {
			hash = hash[:8]
		}
		entry := LogEntry{Hash: hash}
		if len(parts) > 1 {
			entry.Message = parts[1]
		}
		if len(parts) > 2 {
			entry.Author = parts[2]
		}
		if len(parts) > 3 {
			entry.Date = parts[3]
		}
		entries = append(entries, entry)
	}
	return entries
}

func (s *Service) ConflictFiles(projectPath string) ([]string, error) {
	out, err := runGit(projectPath, "diff", "--name-only", "--diff-filter=U")
	if err != nil {
		return nil, err
	}
	if out == "" {
		return nil, nil
	}
	return strings.Split(strings.TrimSpace(out), "\n"), nil
}
