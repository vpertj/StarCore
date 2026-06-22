package verify

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

type CheckType string

const (
	CheckLint      CheckType = "lint"
	CheckBuild     CheckType = "build"
	CheckTest      CheckType = "test"
	CheckVet       CheckType = "vet"
	CheckTypeCheck CheckType = "typecheck"
)

type CheckResult struct {
	Type     CheckType    `json:"type"`
	Passed   bool         `json:"passed"`
	Output   string       `json:"output"`
	Errors   []Diagnostic `json:"errors,omitempty"`
	Duration string       `json:"duration"`
	Command  string       `json:"command"`
}

type Diagnostic struct {
	File     string `json:"file"`
	Line     int    `json:"line"`
	Column   int    `json:"column"`
	Severity string `json:"severity"`
	Message  string `json:"message"`
}

type VerificationResult struct {
	AllPassed bool          `json:"allPassed"`
	Checks    []CheckResult `json:"checks"`
	Summary   string        `json:"summary"`
}

type TestSuiteResult struct {
	Name      string           `json:"name"`
	Total     int              `json:"total"`
	Passed    int              `json:"passed"`
	Failed    int              `json:"failed"`
	Skipped   int              `json:"skipped"`
	Duration  string           `json:"duration"`
	TestCases []TestCaseResult `json:"testCases"`
}

type TestCaseResult struct {
	Name     string `json:"name"`
	Status   string `json:"status"`
	Duration string `json:"duration"`
	Output   string `json:"output,omitempty"`
	File     string `json:"file,omitempty"`
	Line     int    `json:"line,omitempty"`
}

type Service struct {
	projectDir string
}

func NewService(projectDir string) *Service {
	return &Service{projectDir: projectDir}
}

func (s *Service) SetProjectDir(dir string) {
	s.projectDir = dir
}

func (s *Service) Verify(ctx context.Context, filePaths []string) *VerificationResult {
	result := &VerificationResult{AllPassed: true}

	lang := s.detectProjectLanguage()
	checks := s.getChecksForLanguage(lang, filePaths)

	for _, check := range checks {
		cr := s.runCheck(ctx, check)
		result.Checks = append(result.Checks, cr)
		if !cr.Passed {
			result.AllPassed = false
		}
	}

	result.Summary = s.buildSummary(result)
	return result
}

func (s *Service) VerifyFile(ctx context.Context, filePath string) *VerificationResult {
	return s.Verify(ctx, []string{filePath})
}

func (s *Service) QuickVerify(ctx context.Context, filePath string) *CheckResult {
	lang := s.detectLanguageFromFile(filePath)
	var check CheckConfig
	switch lang {
	case "go":
		check = CheckConfig{Type: CheckVet, Command: "go", Args: []string{"vet", "./..."}, WorkDir: s.projectDir}
	case "typescript", "javascript":
		check = CheckConfig{Type: CheckTypeCheck, Command: "npx", Args: []string{"tsc", "--noEmit"}, WorkDir: s.projectDir}
	case "python":
		check = CheckConfig{Type: CheckLint, Command: "python", Args: []string{"-m", "py_compile", filePath}, WorkDir: s.projectDir}
	default:
		return &CheckResult{Type: CheckLint, Passed: true, Output: "no quick check available"}
	}
	result := s.runCheck(ctx, check)
	return &result
}

type CheckConfig struct {
	Type    CheckType
	Command string
	Args    []string
	WorkDir string
	Timeout time.Duration
}

func (s *Service) getChecksForLanguage(lang string, filePaths []string) []CheckConfig {
	var checks []CheckConfig

	switch lang {
	case "go":
		checks = append(checks,
			CheckConfig{Type: CheckBuild, Command: "go", Args: []string{"build", "./..."}, WorkDir: s.projectDir, Timeout: 60 * time.Second},
			CheckConfig{Type: CheckVet, Command: "go", Args: []string{"vet", "./..."}, WorkDir: s.projectDir, Timeout: 30 * time.Second},
			CheckConfig{Type: CheckTest, Command: "go", Args: []string{"test", "./...", "-count=1"}, WorkDir: s.projectDir, Timeout: 120 * time.Second},
		)
	case "typescript", "javascript":
		checks = append(checks,
			CheckConfig{Type: CheckBuild, Command: "npm", Args: []string{"run", "build"}, WorkDir: s.projectDir, Timeout: 60 * time.Second},
		)
		if lang == "typescript" {
			checks = append(checks,
				CheckConfig{Type: CheckTypeCheck, Command: "npx", Args: []string{"tsc", "--noEmit"}, WorkDir: s.projectDir, Timeout: 30 * time.Second},
			)
		}
		checks = append(checks,
			CheckConfig{Type: CheckLint, Command: "npx", Args: []string{"eslint", ".", "--max-warnings", "0"}, WorkDir: s.projectDir, Timeout: 30 * time.Second},
		)
	case "python":
		checks = append(checks,
			CheckConfig{Type: CheckLint, Command: "python", Args: []string{"-m", "py_compile"}, WorkDir: s.projectDir, Timeout: 30 * time.Second},
			CheckConfig{Type: CheckTest, Command: "python", Args: []string{"-m", "pytest"}, WorkDir: s.projectDir, Timeout: 120 * time.Second},
		)
	case "rust":
		checks = append(checks,
			CheckConfig{Type: CheckBuild, Command: "cargo", Args: []string{"check"}, WorkDir: s.projectDir, Timeout: 120 * time.Second},
			CheckConfig{Type: CheckTest, Command: "cargo", Args: []string{"test"}, WorkDir: s.projectDir, Timeout: 120 * time.Second},
		)
	default:
		if len(filePaths) > 0 {
			checks = append(checks,
				CheckConfig{Type: CheckLint, Command: "echo", Args: []string{"no checks configured"}, WorkDir: s.projectDir},
			)
		}
	}

	return checks
}

func (s *Service) runCheck(ctx context.Context, check CheckConfig) CheckResult {
	timeout := check.Timeout
	if timeout == 0 {
		timeout = 30 * time.Second
	}

	checkCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	cmd := exec.CommandContext(checkCtx, check.Command, check.Args...)
	if check.WorkDir != "" {
		cmd.Dir = check.WorkDir
	}

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	start := time.Now()
	err := cmd.Run()
	duration := time.Since(start)

	result := CheckResult{
		Type:     check.Type,
		Command:  check.Command + " " + strings.Join(check.Args, " "),
		Duration: duration.Round(time.Millisecond).String(),
	}

	output := stdout.String()
	if stderr.Len() > 0 {
		if output != "" {
			output += "\n"
		}
		output += stderr.String()
	}
	result.Output = output
	if len(result.Output) > 4000 {
		result.Output = result.Output[:4000] + "\n... [truncated]"
	}

	if err == nil {
		result.Passed = true
	} else {
		result.Passed = false
		result.Errors = s.parseDiagnostics(output, check.Type)
	}

	return result
}

func (s *Service) parseDiagnostics(output string, checkType CheckType) []Diagnostic {
	var diags []Diagnostic

	patterns := []*regexp.Regexp{
		regexp.MustCompile(`^(.+?):(\d+):(\d+):\s*(.+)$`),
		regexp.MustCompile(`^(.+?):(\d+):\s*(.+)$`),
		regexp.MustCompile(`^(.+?)\((\d+),(\d+)\):\s*(.+)$`),
	}

	lines := strings.Split(output, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		for _, pat := range patterns {
			m := pat.FindStringSubmatch(line)
			if m == nil {
				continue
			}

			diag := Diagnostic{}
			switch len(m) {
			case 5:
				diag.File = m[1]
				fmt.Sscanf(m[2], "%d", &diag.Line)
				fmt.Sscanf(m[3], "%d", &diag.Column)
				diag.Message = m[4]
			case 4:
				diag.File = m[1]
				fmt.Sscanf(m[2], "%d", &diag.Line)
				diag.Message = m[3]
			}

			if strings.Contains(diag.Message, "error") {
				diag.Severity = "error"
			} else if strings.Contains(diag.Message, "warning") {
				diag.Severity = "warning"
			} else {
				diag.Severity = "info"
			}

			if diag.File != "" && diag.Message != "" {
				diags = append(diags, diag)
				if len(diags) >= 20 {
					return diags
				}
			}
			break
		}
	}

	return diags
}

func (s *Service) buildSummary(result *VerificationResult) string {
	if result.AllPassed {
		return "All checks passed"
	}

	var errors, warnings int
	var failedChecks []string
	for _, check := range result.Checks {
		if !check.Passed {
			failedChecks = append(failedChecks, string(check.Type))
			for _, diag := range check.Errors {
				if diag.Severity == "error" {
					errors++
				} else if diag.Severity == "warning" {
					warnings++
				}
			}
		}
	}

	summary := fmt.Sprintf("Failed: %s", strings.Join(failedChecks, ", "))
	if errors > 0 || warnings > 0 {
		summary += fmt.Sprintf(" (%d errors, %d warnings)", errors, warnings)
	}
	return summary
}

func (s *Service) RunTests(ctx context.Context, testPath string) []TestSuiteResult {
	if s.projectDir == "" {
		return nil
	}

	lang := s.detectProjectLanguage()
	var results []TestSuiteResult

	switch lang {
	case "go":
		results = s.runGoTests(ctx, testPath)
	case "typescript", "javascript":
		results = s.runJSTests(ctx, testPath)
	case "python":
		results = s.runPythonTests(ctx, testPath)
	case "rust":
		results = s.runRustTests(ctx, testPath)
	default:
		results = s.runGoTests(ctx, testPath)
	}

	return results
}

func (s *Service) runGoTests(ctx context.Context, testPath string) []TestSuiteResult {
	args := []string{"test", "-json", "-count=1"}
	if testPath != "" {
		args = append(args, testPath)
	} else {
		args = append(args, "./...")
	}

	testCtx, cancel := context.WithTimeout(ctx, 120*time.Second)
	defer cancel()

	cmd := exec.CommandContext(testCtx, "go", args...)
	cmd.Dir = s.projectDir
	var stdout bytes.Buffer
	cmd.Stdout = &stdout
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	start := time.Now()
	cmd.Run()
	totalDuration := time.Since(start).Round(time.Millisecond).String()

	return s.parseGoTestJSON(stdout.String(), stderr.String(), totalDuration)
}

func (s *Service) parseGoTestJSON(jsonOutput, stderrStr, totalDuration string) []TestSuiteResult {
	suiteMap := make(map[string]*TestSuiteResult)
	var suiteOrder []string

	type goTestEvent struct {
		Time    string  `json:"Time"`
		Action  string  `json:"Action"`
		Package string  `json:"Package"`
		Test    string  `json:"Test"`
		Output  string  `json:"Output"`
		Elapsed float64 `json:"Elapsed"`
	}

	lines := strings.Split(jsonOutput, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		var ev goTestEvent
		if json.Unmarshal([]byte(line), &ev) != nil {
			continue
		}
		if ev.Test == "" {
			continue
		}

		pkg := ev.Package
		if pkg == "" {
			pkg = "default"
		}
		if _, ok := suiteMap[pkg]; !ok {
			suiteMap[pkg] = &TestSuiteResult{Name: pkg}
			suiteOrder = append(suiteOrder, pkg)
		}
		suite := suiteMap[pkg]

		switch ev.Action {
		case "pass":
			suite.TestCases = append(suite.TestCases, TestCaseResult{
				Name:     ev.Test,
				Status:   "passed",
				Duration: fmt.Sprintf("%.0fms", ev.Elapsed*1000),
			})
			suite.Passed++
			suite.Total++
		case "fail":
			suite.TestCases = append(suite.TestCases, TestCaseResult{
				Name:     ev.Test,
				Status:   "failed",
				Duration: fmt.Sprintf("%.0fms", ev.Elapsed*1000),
				Output:   strings.TrimSpace(ev.Output),
			})
			suite.Failed++
			suite.Total++
		case "skip":
			suite.TestCases = append(suite.TestCases, TestCaseResult{
				Name:   ev.Test,
				Status: "skipped",
			})
			suite.Skipped++
			suite.Total++
		case "output":
			if len(suite.TestCases) > 0 {
				last := &suite.TestCases[len(suite.TestCases)-1]
				if last.Status == "failed" {
					if last.Output == "" {
						last.Output = strings.TrimSpace(ev.Output)
					} else {
						last.Output += "\n" + strings.TrimSpace(ev.Output)
					}
					if len(last.Output) > 2000 {
						last.Output = last.Output[:2000] + "\n... [truncated]"
					}
				}
			}
		}
	}

	if len(suiteMap) == 0 && stderrStr != "" {
		return []TestSuiteResult{{
			Name:     "go test",
			Total:    1,
			Failed:   1,
			Duration: totalDuration,
			TestCases: []TestCaseResult{{
				Name:   "test execution",
				Status: "failed",
				Output: truncate(stderrStr, 2000),
			}},
		}}
	}

	var results []TestSuiteResult
	for _, pkg := range suiteOrder {
		suite := suiteMap[pkg]
		suite.Duration = totalDuration
		results = append(results, *suite)
	}
	return results
}

func (s *Service) runJSTests(ctx context.Context, testPath string) []TestSuiteResult {
	testCtx, cancel := context.WithTimeout(ctx, 120*time.Second)
	defer cancel()

	cmd := exec.CommandContext(testCtx, "npx", "vitest", "run", "--reporter=json")
	cmd.Dir = s.projectDir
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	start := time.Now()
	cmd.Run()
	duration := time.Since(start).Round(time.Millisecond).String()

	output := stdout.String()
	if output == "" {
		output = stderr.String()
	}

	return []TestSuiteResult{{
		Name:     "vitest",
		Duration: duration,
		TestCases: []TestCaseResult{{
			Name:   "test run",
			Status: condStr(cmd.ProcessState.Success(), "passed", "failed"),
			Output: truncate(output, 2000),
		}},
		Total:  1,
		Passed: condInt(cmd.ProcessState.Success(), 1, 0),
		Failed: condInt(cmd.ProcessState.Success(), 0, 1),
	}}
}

func (s *Service) runPythonTests(ctx context.Context, testPath string) []TestSuiteResult {
	testCtx, cancel := context.WithTimeout(ctx, 120*time.Second)
	defer cancel()

	args := []string{"-m", "pytest", "-v"}
	if testPath != "" {
		args = append(args, testPath)
	}
	cmd := exec.CommandContext(testCtx, "python", args...)
	cmd.Dir = s.projectDir
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	start := time.Now()
	cmd.Run()
	duration := time.Since(start).Round(time.Millisecond).String()

	output := stdout.String() + stderr.String()
	return []TestSuiteResult{{
		Name:     "pytest",
		Duration: duration,
		TestCases: []TestCaseResult{{
			Name:   "test run",
			Status: condStr(cmd.ProcessState.Success(), "passed", "failed"),
			Output: truncate(output, 2000),
		}},
		Total:  1,
		Passed: condInt(cmd.ProcessState.Success(), 1, 0),
		Failed: condInt(cmd.ProcessState.Success(), 0, 1),
	}}
}

func (s *Service) runRustTests(ctx context.Context, testPath string) []TestSuiteResult {
	testCtx, cancel := context.WithTimeout(ctx, 120*time.Second)
	defer cancel()

	cmd := exec.CommandContext(testCtx, "cargo", "test")
	cmd.Dir = s.projectDir
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	start := time.Now()
	cmd.Run()
	duration := time.Since(start).Round(time.Millisecond).String()

	output := stdout.String() + stderr.String()
	return []TestSuiteResult{{
		Name:     "cargo test",
		Duration: duration,
		TestCases: []TestCaseResult{{
			Name:   "test run",
			Status: condStr(cmd.ProcessState.Success(), "passed", "failed"),
			Output: truncate(output, 2000),
		}},
		Total:  1,
		Passed: condInt(cmd.ProcessState.Success(), 1, 0),
		Failed: condInt(cmd.ProcessState.Success(), 0, 1),
	}}
}

func truncate(s string, maxLen int) string {
	if len(s) > maxLen {
		return s[:maxLen] + "\n... [truncated]"
	}
	return s
}

func condStr(cond bool, a, b string) string {
	if cond {
		return a
	}
	return b
}

func condInt(cond bool, a, b int) int {
	if cond {
		return a
	}
	return b
}

func (s *Service) detectProjectLanguage() string {
	if s.projectDir == "" {
		return "unknown"
	}

	indicators := map[string]string{
		"go.mod":         "go",
		"package.json":   "javascript",
		"tsconfig.json":  "typescript",
		"Cargo.toml":     "rust",
		"pyproject.toml": "python",
		"setup.py":       "python",
		"pom.xml":        "java",
		"build.gradle":   "java",
	}

	for file, lang := range indicators {
		if _, err := exec.LookPath(filepath.Join(s.projectDir, file)); err == nil {
			if lang == "javascript" {
				if _, err2 := exec.LookPath(filepath.Join(s.projectDir, "tsconfig.json")); err2 == nil {
					return "typescript"
				}
			}
			return lang
		}
	}

	return "unknown"
}

func (s *Service) detectLanguageFromFile(filePath string) string {
	ext := strings.ToLower(filepath.Ext(filePath))
	switch ext {
	case ".go":
		return "go"
	case ".ts", ".tsx":
		return "typescript"
	case ".js", ".jsx":
		return "javascript"
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
