package codescan

import (
	"strings"
	"testing"
)

func TestSQLInjection(t *testing.T) {
	re := NewRuleEngine()
	code := `query := "SELECT * FROM users WHERE id=" + req.Param("id")`
	findings := re.ScanFile(code, "test.go", "go")
	if len(findings) == 0 {
		t.Error("should detect SQL injection")
	}
	if findings[0].CWE != "CWE-89" {
		t.Errorf("CWE = %s, want CWE-89", findings[0].CWE)
	}
}

func TestCommandInjection(t *testing.T) {
	re := NewRuleEngine()
	code := `cmd := exec.Command("sh", "-c", userInput)`
	findings := re.ScanFile(code, "test.go", "go")
	if len(findings) == 0 {
		t.Error("should detect command injection")
	}
}

func TestXSS(t *testing.T) {
	re := NewRuleEngine()
	code := `element.innerHTML = userInput;`
	findings := re.ScanFile(code, "test.js", "javascript")
	if len(findings) == 0 {
		t.Error("should detect XSS")
	}
}

func TestHardcodedSecret(t *testing.T) {
	re := NewRuleEngine()
	code := `const apiKey = "sk-abcdefghijklmnopqrstuvwx"`
	findings := re.ScanFile(code, "test.js", "javascript")
	if len(findings) == 0 {
		t.Error("should detect hardcoded secret")
	}
}

func TestHardcodedCredentials(t *testing.T) {
	re := NewRuleEngine()
	code := `const password = "supersecret123"`
	findings := re.ScanFile(code, "test.js", "javascript")
	if len(findings) == 0 {
		t.Error("should detect hardcoded credentials")
	}
}

func TestWeakHash(t *testing.T) {
	re := NewRuleEngine()
	code := `hash := md5.Sum(data)`
	findings := re.ScanFile(code, "test.go", "go")
	if len(findings) == 0 {
		t.Error("should detect weak hash MD5")
	}
}

func TestTLSInsecure(t *testing.T) {
	re := NewRuleEngine()
	code := `tls.Config{InsecureSkipVerify: true}`
	findings := re.ScanFile(code, "test.go", "go")
	if len(findings) == 0 {
		t.Error("should detect insecure TLS")
	}
}

func TestPathTraversal(t *testing.T) {
	re := NewRuleEngine()
	code := `data, _ := os.Open(req.Param("file") + "/data")`
	findings := re.ScanFile(code, "test.go", "go")
	if len(findings) == 0 {
		t.Error("should detect path traversal")
	}
}

func TestCORSAllowAll(t *testing.T) {
	re := NewRuleEngine()
	code := `w.Header().Set("Access-Control-Allow-Origin", "*")`
	findings := re.ScanFile(code, "test.go", "go")
	if len(findings) == 0 {
		t.Error("should detect CORS allow all")
	}
}

func TestSSRF(t *testing.T) {
	re := NewRuleEngine()
	code := `resp, _ := http.Get(req.Query("url"))`
	findings := re.ScanFile(code, "test.go", "go")
	if len(findings) == 0 {
		t.Error("should detect SSRF")
	}
}

func TestSafeCode(t *testing.T) {
	re := NewRuleEngine()
	code := `func add(a, b int) int { return a + b }`
	findings := re.ScanFile(code, "test.go", "go")
	if len(findings) > 0 {
		t.Errorf("safe code should not trigger findings, got %d", len(findings))
	}
}

func TestScanResult_HealthScore(t *testing.T) {
	re := NewRuleEngine()
	code := `password := "secret123"` + "\n" + `hash := md5.Sum(data)`
	result := re.ScanFileWithResult(code, "test.go", "go")
	if result.HealthScore >= 100 {
		t.Error("health score should be < 100 with findings")
	}
	if result.Total == 0 {
		t.Error("should have findings")
	}
}

func TestRuleCount(t *testing.T) {
	re := NewRuleEngine()
	rules := re.ListRules()
	if len(rules) < 20 {
		t.Errorf("expected at least 20 rules, got %d", len(rules))
	}
}

func TestListRulesByCategory(t *testing.T) {
	re := NewRuleEngine()
	injection := re.ListRulesByCategory("injection")
	if len(injection) == 0 {
		t.Error("should have injection rules")
	}
}

func TestStats(t *testing.T) {
	re := NewRuleEngine()
	stats := re.Stats()
	if stats["total"] < 20 {
		t.Errorf("expected at least 20 total rules, got %d", stats["total"])
	}
}

func TestScanFiles(t *testing.T) {
	re := NewRuleEngine()
	files := map[string]string{
		"main.go": `query := "SELECT * FROM t WHERE id=" + req.Param("id")`,
		"safe.go": `func add(a, b int) int { return a + b }`,
	}
	findings := re.ScanFiles(files, "go")
	if len(findings) == 0 {
		t.Error("should find issues across files")
	}
}

func TestScanString(t *testing.T) {
	re := NewRuleEngine()
	code := strings.Join([]string{
		`password := "hardcoded123"`,
		`hash := md5.Sum(data)`,
		`w.Header().Set("Access-Control-Allow-Origin", "*")`,
	}, "\n")
	findings := re.ScanString(code, "test.go")
	if len(findings) < 2 {
		t.Errorf("expected at least 2 findings, got %d", len(findings))
	}
}
