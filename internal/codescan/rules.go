package codescan

import (
	"fmt"
	"regexp"
	"strings"
)

func buildRules() []RuleDef {
	var rules []RuleDef

	rules = append(rules, injectionRules()...)
	rules = append(rules, xssRules()...)
	rules = append(rules, cryptoRules()...)
	rules = append(rules, pathTraversalRules()...)
	rules = append(rules, authRules()...)
	rules = append(rules, dataExposureRules()...)
	rules = append(rules, misconfigRules()...)
	rules = append(rules, codeQualityRules()...)

	return rules
}

func injectionRules() []RuleDef {
	return []RuleDef{
		{
			Rule: Rule{ID: "SC-001", Name: "SQL Injection", Category: "injection",
				OWASP: "A03:2021", CWE: "CWE-89", Severity: SeverityCritical,
				Description: "Potential SQL injection: string concatenation in SQL query",
				Languages:   []string{"go", "python", "java", "javascript", "typescript", "php", "ruby"}},
			Pattern: regexp.MustCompile(`(?i)(SELECT|INSERT|UPDATE|DELETE|DROP|ALTER|CREATE)\s+.*\+\s*.*(?:req|param|input|query|args|user|body|data)`),
		},
		{
			Rule: Rule{ID: "SC-002", Name: "SQL Injection (fmt.Sprintf)", Category: "injection",
				OWASP: "A03:2021", CWE: "CWE-89", Severity: SeverityCritical,
				Description: "Potential SQL injection: fmt.Sprintf used to build SQL query",
				Languages:   []string{"go"}},
			Pattern: regexp.MustCompile(`fmt\.Sprintf\(.*(?:SELECT|INSERT|UPDATE|DELETE|DROP)`),
		},
		{
			Rule: Rule{ID: "SC-003", Name: "Command Injection", Category: "injection",
				OWASP: "A03:2021", CWE: "CWE-78", Severity: SeverityCritical,
				Description: "Potential command injection: user input passed to exec/command",
				Languages:   []string{"go", "python", "javascript", "ruby", "php"}},
			Pattern: regexp.MustCompile(`(?i)(exec\.Command|os\.system|subprocess\.(run|call|Popen)|child_process\.exec)`),
		},
		{
			Rule: Rule{ID: "SC-004", Name: "LDAP Injection", Category: "injection",
				OWASP: "A03:2021", CWE: "CWE-90", Severity: SeverityHigh,
				Description: "Potential LDAP injection: user input in LDAP query",
				Languages:   []string{"*"}},
			Pattern: regexp.MustCompile(`(?i)ldap.*(?:search|find|filter).*\+`),
		},
		{
			Rule: Rule{ID: "SC-005", Name: "Template Injection", Category: "injection",
				OWASP: "A03:2021", CWE: "CWE-94", Severity: SeverityHigh,
				Description: "Potential server-side template injection",
				Languages:   []string{"go", "python", "javascript"}},
			Pattern: regexp.MustCompile(`(?i)(template\.HTML|render_template_string|eval\(|Function\()`),
		},
	}
}

func xssRules() []RuleDef {
	return []RuleDef{
		{
			Rule: Rule{ID: "SC-010", Name: "Cross-Site Scripting (Reflected)", Category: "xss",
				OWASP: "A03:2021", CWE: "CWE-79", Severity: SeverityHigh,
				Description: "Potential XSS: unescaped user input rendered in HTML",
				Languages:   []string{"javascript", "typescript", "python", "ruby", "php"}},
			Pattern: regexp.MustCompile(`(?i)(innerHTML|\.html\(|v-html|dangerouslySetInnerHTML|markupsafe|raw\()`),
		},
		{
			Rule: Rule{ID: "SC-011", Name: "XSS via document.write", Category: "xss",
				OWASP: "A03:2021", CWE: "CWE-79", Severity: SeverityHigh,
				Description: "Potential XSS: document.write with dynamic content",
				Languages:   []string{"javascript", "typescript"}},
			Pattern: regexp.MustCompile(`document\.write\(`),
		},
		{
			Rule: Rule{ID: "SC-012", Name: "URL Redirect (Open Redirect)", Category: "xss",
				OWASP: "A01:2021", CWE: "CWE-601", Severity: SeverityMedium,
				Description: "Potential open redirect: user-controlled URL in redirect",
				Languages:   []string{"go", "python", "javascript", "java", "ruby"}},
			Pattern: regexp.MustCompile(`(?i)(http\.Redirect|redirect\(|response\.redirect|res\.redirect).*(?:req|param|query|url)`),
		},
	}
}

func cryptoRules() []RuleDef {
	return []RuleDef{
		{
			Rule: Rule{ID: "SC-020", Name: "Weak Hash Algorithm (MD5)", Category: "crypto",
				OWASP: "A02:2021", CWE: "CWE-328", Severity: SeverityMedium,
				Description: "Weak hash algorithm MD5 used - use SHA-256 or stronger",
				Languages:   []string{"*"}},
			Pattern: regexp.MustCompile(`(?i)(md5|MD5\.Sum|hashlib\.md5|MessageDigest\.getInstance\("MD5"\))`),
		},
		{
			Rule: Rule{ID: "SC-021", Name: "Weak Hash Algorithm (SHA1)", Category: "crypto",
				OWASP: "A02:2021", CWE: "CWE-328", Severity: SeverityMedium,
				Description: "Weak hash algorithm SHA1 used - use SHA-256 or stronger",
				Languages:   []string{"*"}},
			Pattern: regexp.MustCompile(`(?i)(sha1|SHA1\.Sum|hashlib\.sha1|MessageDigest\.getInstance\("SHA1"\)|sha1\.New)`),
		},
		{
			Rule: Rule{ID: "SC-022", Name: "Hardcoded Secret", Category: "crypto",
				OWASP: "A02:2021", CWE: "CWE-798", Severity: SeverityCritical,
				Description: "Potential hardcoded secret/API key in source code",
				Languages:   []string{"*"}},
			Check: hardcodedSecretCheck,
		},
		{
			Rule: Rule{ID: "SC-023", Name: "Insecure Random", Category: "crypto",
				OWASP: "A02:2021", CWE: "CWE-330", Severity: SeverityMedium,
				Description: "Insecure random number generator used for security purposes",
				Languages:   []string{"go", "python", "java", "javascript"}},
			Pattern: regexp.MustCompile(`(?i)(math/rand|random\.Random|Math\.random\(\))`),
		},
		{
			Rule: Rule{ID: "SC-024", Name: "Weak Encryption (DES/RC4)", Category: "crypto",
				OWASP: "A02:2021", CWE: "CWE-326", Severity: SeverityHigh,
				Description: "Weak encryption algorithm used - use AES-256 or stronger",
				Languages:   []string{"*"}},
			Pattern: regexp.MustCompile(`(?i)(des\.NewCipher|rc4\.NewCipher|DES|RC4|Blowfish)`),
		},
		{
			Rule: Rule{ID: "SC-025", Name: "TLS Verification Disabled", Category: "crypto",
				OWASP: "A02:2021", CWE: "CWE-295", Severity: SeverityHigh,
				Description: "TLS certificate verification disabled",
				Languages:   []string{"go", "python"}},
			Pattern: regexp.MustCompile(`(?i)(InsecureSkipVerify\s*[:=]\s*true|verify\s*[:=]\s*False|CERT_NONE)`),
		},
	}
}

func pathTraversalRules() []RuleDef {
	return []RuleDef{
		{
			Rule: Rule{ID: "SC-030", Name: "Path Traversal", Category: "path_traversal",
				OWASP: "A01:2021", CWE: "CWE-22", Severity: SeverityHigh,
				Description: "Potential path traversal: user input used in file path",
				Languages:   []string{"go", "python", "java", "javascript", "ruby"}},
			Pattern: regexp.MustCompile(`(?i)(os\.Open|ioutil\.ReadFile|os\.ReadFile|open\(|f\.open|FileInputStream).*\+`),
		},
		{
			Rule: Rule{ID: "SC-031", Name: "Path Traversal (filepath.Join)", Category: "path_traversal",
				OWASP: "A01:2021", CWE: "CWE-22", Severity: SeverityMedium,
				Description: "Potential path traversal: user input in filepath.Join without validation",
				Languages:   []string{"go"}},
			Pattern: regexp.MustCompile(`filepath\.Join\(.*(?:req|param|input|query|args|user|body)`),
		},
	}
}

func authRules() []RuleDef {
	return []RuleDef{
		{
			Rule: Rule{ID: "SC-040", Name: "Hardcoded Credentials", Category: "auth",
				OWASP: "A07:2021", CWE: "CWE-798", Severity: SeverityCritical,
				Description: "Hardcoded username/password in source code",
				Languages:   []string{"*"}},
			Pattern: regexp.MustCompile(`(?i)(password|passwd|pwd|secret|token|api_key|apikey)\s*[:=]\s*["'][^"']{3,}["']`),
		},
		{
			Rule: Rule{ID: "SC-041", Name: "Missing Auth Check", Category: "auth",
				OWASP: "A07:2021", CWE: "CWE-306", Severity: SeverityHigh,
				Description: "HTTP handler may be missing authentication check",
				Languages:   []string{"go", "python", "java"}},
			Pattern: regexp.MustCompile(`(?i)(http\.HandleFunc|@app\.route|@GetMapping|@PostMapping)\(.*(?:admin|delete|update|config|settings|manage)`),
		},
		{
			Rule: Rule{ID: "SC-042", Name: "CORS Allow All", Category: "auth",
				OWASP: "A05:2021", CWE: "CWE-942", Severity: SeverityMedium,
				Description: "CORS configured to allow all origins",
				Languages:   []string{"go", "python", "javascript", "java"}},
			Pattern: regexp.MustCompile(`(?i)(Access-Control-Allow-Origin.*\*|cors.*allow_all|CORS_ALLOW_ALL|AllowAllOrigins)`),
		},
	}
}

func dataExposureRules() []RuleDef {
	return []RuleDef{
		{
			Rule: Rule{ID: "SC-050", Name: "Sensitive Data Exposure", Category: "data_exposure",
				OWASP: "A01:2021", CWE: "CWE-200", Severity: SeverityHigh,
				Description: "Potential sensitive data exposure in error message or log",
				Languages:   []string{"*"}},
			Pattern: regexp.MustCompile(`(?i)(log\.(Print|Fatal|Panic)|fmt\.Print|console\.log|print\().*(?:password|secret|token|key|credential)`),
		},
		{
			Rule: Rule{ID: "SC-051", Name: "Information Disclosure (Stack Trace)", Category: "data_exposure",
				OWASP: "A01:2021", CWE: "CWE-209", Severity: SeverityMedium,
				Description: "Stack trace or detailed error exposed to user",
				Languages:   []string{"go", "python", "java"}},
			Pattern: regexp.MustCompile(`(?i)(http\.Error\(.*err\.Error|response\.write\(.*exception|return.*stacktrace)`),
		},
		{
			Rule: Rule{ID: "SC-052", Name: "Debug Mode Enabled", Category: "data_exposure",
				OWASP: "A05:2021", CWE: "CWE-215", Severity: SeverityHigh,
				Description: "Debug mode enabled in configuration",
				Languages:   []string{"*"}},
			Pattern: regexp.MustCompile(`(?i)(DEBUG\s*[:=]\s*True|debug:\s*true|app\.debug|DEBUG_MODE\s*[:=]\s*1)`),
		},
	}
}

func misconfigRules() []RuleDef {
	return []RuleDef{
		{
			Rule: Rule{ID: "SC-060", Name: "Server-Side Request Forgery", Category: "misconfig",
				OWASP: "A10:2021", CWE: "CWE-918", Severity: SeverityCritical,
				Description: "Potential SSRF: user-controlled URL in HTTP request",
				Languages:   []string{"go", "python", "java", "javascript"}},
			Pattern: regexp.MustCompile(`(?i)(http\.Get\(|http\.Post\(|requests\.(get|post)\(|fetch\().*(?:req|param|input|query|url|body)`),
		},
		{
			Rule: Rule{ID: "SC-061", Name: "Insecure File Permission", Category: "misconfig",
				OWASP: "A05:2021", CWE: "CWE-732", Severity: SeverityLow,
				Description: "Overly permissive file creation (world-writable)",
				Languages:   []string{"go"}},
			Pattern: regexp.MustCompile(`os\.Create|os\.MkdirAll|os\.OpenFile`),
		},
		{
			Rule: Rule{ID: "SC-062", Name: "XML External Entity", Category: "misconfig",
				OWASP: "A05:2021", CWE: "CWE-611", Severity: SeverityHigh,
				Description: "Potential XXE: XML parser without entity restriction",
				Languages:   []string{"go", "python", "java"}},
			Pattern: regexp.MustCompile(`(?i)(xml\.Unmarshal|xml\.Decoder|etree\.parse|DocumentBuilderFactory)`),
		},
	}
}

func codeQualityRules() []RuleDef {
	return []RuleDef{
		{
			Rule: Rule{ID: "SC-070", Name: "Empty Error Handling", Category: "quality",
				OWASP: "", CWE: "CWE-390", Severity: SeverityMedium,
				Description: "Error returned but not checked or silently ignored",
				Languages:   []string{"go"}},
			Pattern: regexp.MustCompile(`(?:^|\s)(?:_\s*,)?\s*\w+\s*:?=\s*\w+\(.*\)\s*$`),
		},
		{
			Rule: Rule{ID: "SC-071", Name: "TODO/FIXME with Security Implication", Category: "quality",
				OWASP: "", CWE: "", Severity: SeverityInfo,
				Description: "TODO/FIXME comment that may indicate incomplete security implementation",
				Languages:   []string{"*"}},
			Pattern: regexp.MustCompile(`(?i)(TODO|FIXME|HACK|XXX).*(?:security|auth|encrypt|password|validate|sanitize)`),
		},
		{
			Rule: Rule{ID: "SC-072", Name: "Unbounded Resource Allocation", Category: "quality",
				OWASP: "", CWE: "CWE-400", Severity: SeverityMedium,
				Description: "Potential unbounded allocation without size limit",
				Languages:   []string{"go", "python", "java"}},
			Pattern: regexp.MustCompile(`(?i)(ioutil\.ReadAll|io\.ReadAll|read\(\)|readline\(\)).*(?:req|request|body|input)`),
		},
	}
}

var secretPatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)(?:api[_-]?key|apikey|access[_-]?key|secret[_-]?key)\s*[:=]\s*["'][a-zA-Z0-9_\-]{16,}["']`),
	regexp.MustCompile(`(?i)(?:aws[_-]?access[_-]?key[_-]?id)\s*[:=]\s*["']AKIA[0-9A-Z]{16}["']`),
	regexp.MustCompile(`(?i)(?:aws[_-]?secret[_-]?access[_-]?key)\s*[:=]\s*["'][a-zA-Z0-9/+=]{40}["']`),
	regexp.MustCompile(`sk-[a-zA-Z0-9]{20,}`),
	regexp.MustCompile(`ghp_[a-zA-Z0-9]{36}`),
	regexp.MustCompile(`eyJ[a-zA-Z0-9_\-]{20,}\.[a-zA-Z0-9_\-]{20,}`),
}

func hardcodedSecretCheck(content string, filePath string) []Finding {
	var findings []Finding
	lines := strings.Split(content, "\n")
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "//") || strings.HasPrefix(trimmed, "#") || strings.HasPrefix(trimmed, "/*") {
			continue
		}
		for _, pat := range secretPatterns {
			if pat.MatchString(line) {
				findings = append(findings, Finding{
					RuleID:     "SC-022",
					RuleName:   "Hardcoded Secret",
					File:       filePath,
					Line:       i + 1,
					Column:     1,
					Severity:   SeverityCritical,
					Message:    "Potential hardcoded secret detected - use environment variables or secure vault",
					OWASP:      "A02:2021",
					CWE:        "CWE-798",
					Category:   "crypto",
					Suggestion: "Replace with os.Getenv() or secure config",
				})
				break
			}
		}
	}
	if len(findings) > 10 {
		findings = findings[:10]
	}
	return findings
}

func (re *RuleEngine) ScanString(content string, filePath string) []Finding {
	ext := ""
	if idx := strings.LastIndex(filePath, "."); idx >= 0 {
		ext = filePath[idx:]
	}
	lang := detectLanguageFromExt(ext)
	return re.ScanFile(content, filePath, lang)
}

func (re *RuleEngine) Stats() map[string]int {
	stats := map[string]int{
		"total": len(re.rules),
	}
	for _, rd := range re.rules {
		stats[string(rd.Rule.Severity)]++
		stats[rd.Rule.Category]++
	}
	return stats
}

func (re *RuleEngine) ScanFileWithResult(content string, filePath string, language string) *ScanResult {
	findings := re.ScanFile(content, filePath, language)
	critical, high, medium, low, info := 0, 0, 0, 0, 0
	for _, f := range findings {
		switch f.Severity {
		case SeverityCritical:
			critical++
		case SeverityHigh:
			high++
		case SeverityMedium:
			medium++
		case SeverityLow:
			low++
		case SeverityInfo:
			info++
		}
	}
	healthScore := 100
	if len(findings) > 0 {
		healthScore = 100 - (critical*20 + high*10 + medium*5 + low*2 + info)
		if healthScore < 0 {
			healthScore = 0
		}
	}
	return &ScanResult{
		Findings:    findings,
		Total:       len(findings),
		Critical:    critical,
		High:        high,
		Medium:      medium,
		Low:         low,
		Info:        info,
		HealthScore: healthScore,
		File:        filePath,
	}
}

type ScanResult struct {
	Findings    []Finding `json:"findings"`
	Total       int       `json:"total"`
	Critical    int       `json:"critical"`
	High        int       `json:"high"`
	Medium      int       `json:"medium"`
	Low         int       `json:"low"`
	Info        int       `json:"info"`
	HealthScore int       `json:"healthScore"`
	File        string    `json:"file"`
}

func (sr *ScanResult) Summary() string {
	return fmt.Sprintf("Found %d issues: %d critical, %d high, %d medium, %d low, %d info (health: %d/100)",
		sr.Total, sr.Critical, sr.High, sr.Medium, sr.Low, sr.Info, sr.HealthScore)
}
