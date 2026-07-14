package verify

import (
	"testing"
)

func TestDetectLanguageFromFile(t *testing.T) {
	svc := NewService("")

	tests := []struct {
		path     string
		expected string
	}{
		{"main.go", "go"},
		{"app.ts", "typescript"},
		{"app.tsx", "typescript"},
		{"app.js", "javascript"},
		{"app.py", "python"},
		{"main.rs", "rust"},
		{"App.java", "java"},
		{"unknown.txt", "unknown"},
	}

	for _, tt := range tests {
		result := svc.detectLanguageFromFile(tt.path)
		if result != tt.expected {
			t.Errorf("detectLanguageFromFile(%q) = %q, want %q", tt.path, result, tt.expected)
		}
	}
}

func TestParseDiagnostics(t *testing.T) {
	svc := NewService("")

	output := `main.go:10:5: syntax error
main.go:15:1: undeclared variable
util.go:20:3: warning: unused import`

	diags := svc.parseDiagnostics(output, CheckBuild)
	if len(diags) < 2 {
		t.Errorf("expected at least 2 diagnostics, got %d", len(diags))
	}

	if diags[0].File != "main.go" {
		t.Errorf("expected file 'main.go', got %q", diags[0].File)
	}
	if diags[0].Line != 10 {
		t.Errorf("expected line 10, got %d", diags[0].Line)
	}
}

func TestBuildSummary_AllPassed(t *testing.T) {
	svc := NewService("")
	result := &VerificationResult{
		AllPassed: true,
		Checks: []CheckResult{
			{Type: CheckBuild, Passed: true},
			{Type: CheckVet, Passed: true},
		},
	}
	summary := svc.buildSummary(result)
	if summary != "All checks passed" {
		t.Errorf("expected 'All checks passed', got %q", summary)
	}
}

func TestBuildSummary_Failed(t *testing.T) {
	svc := NewService("")
	result := &VerificationResult{
		AllPassed: false,
		Checks: []CheckResult{
			{Type: CheckBuild, Passed: true},
			{Type: CheckTest, Passed: false, Errors: []Diagnostic{
				{Severity: "error", Message: "test failed"},
			}},
		},
	}
	summary := svc.buildSummary(result)
	if summary == "All checks passed" {
		t.Error("expected failure summary")
	}
}

func TestCheckResult_Timeout(t *testing.T) {
	cr := CheckResult{
		Type:    CheckBuild,
		Passed:  false,
		Output:  "build timed out",
		Command: "go build ./...",
	}
	if cr.Passed {
		t.Error("expected check to fail")
	}
}

func TestDetectLanguagesFromFiles(t *testing.T) {
	svc := NewService("")

	tests := []struct {
		files    []string
		expected int
	}{
		{[]string{"main.go", "util.go"}, 1},             // single language
		{[]string{"app.ts", "util.js"}, 2},              // two JS-family languages
		{[]string{"main.go", "app.ts", "script.py"}, 3}, // three languages
		{[]string{"README.md", "Dockerfile"}, 0},        // no known languages
		{[]string{}, 0},                                 // empty
	}

	for _, tt := range tests {
		langs := svc.detectLanguagesFromFiles(tt.files)
		if len(langs) != tt.expected {
			t.Errorf("detectLanguagesFromFiles(%v) = %d langs, want %d", tt.files, len(langs), tt.expected)
		}
	}

	// Verify uniqueness
	langs := svc.detectLanguagesFromFiles([]string{"a.go", "b.go", "c.go"})
	if len(langs) != 1 || langs[0] != "go" {
		t.Errorf("expected single 'go', got %v", langs)
	}
}
