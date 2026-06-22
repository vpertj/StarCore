package sandbox

import (
	"fmt"
	"regexp"
	"strings"
)

var injectionPatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)ignore\s+(previous|above|all)\s+(instructions|rules|prompts|directions)`),
	regexp.MustCompile(`(?i)you\s+are\s+now\s+(?:a\s+)?(?:DAN|jailbreak|unrestricted|uncensored)`),
	regexp.MustCompile(`(?i)system\s*:\s*(?:you\s+are|you\s+must|ignore|forget|disregard)`),
	regexp.MustCompile(`(?i)(?:pretend|act|roleplay)\s+(?:you\s+are|to\s+be)\s+(?:a\s+)?(?:unrestricted|uncensored|DAN)`),
	regexp.MustCompile(`(?i)output\s+(?:your|the)\s+(?:system|initial|original)\s+(?:prompt|instructions|message)`),
	regexp.MustCompile(`(?i)(?:reveal|show|display|print|repeat)\s+(?:the\s+)?(?:system|initial|original)\s+(?:prompt|instructions)`),
	regexp.MustCompile(`(?i)<!--\s*system.*?-->`),
	regexp.MustCompile(`(?i)\[SYSTEM\].*?\[/SYSTEM\]`),
	regexp.MustCompile(`(?i)<\|im_start\|>system`),
	regexp.MustCompile(`(?i)忽略\s*(?:以上|前面|所有|全部)\s*(?:指令|规则|提示|要求)`),
	regexp.MustCompile(`(?i)请\s*(?:忘记|忽略|放弃| disreg)\s*(?:以上|前面|所有|全部)`),
	regexp.MustCompile(`(?i)###\s*system\s*:`),
	regexp.MustCompile(`(?i)\n\s*system\s*:\s*\n`),
	regexp.MustCompile(`(?i)<system>.*?</system>`),
	regexp.MustCompile(`(?i)\b(?:human|assistant)\s*:\s*\n`),
}

var instructionOverrideKeywords = []string{
	"ignore all previous instructions",
	"ignore your instructions",
	"forget your instructions",
	"disregard your instructions",
	"override your instructions",
	"new instructions:",
	"updated instructions:",
	"jailbreak",
	"DAN mode",
	"developer mode",
	"忽略以上所有指令",
	"忽略前面所有指令",
	"请忘记所有指令",
	"请忽略所有指令",
	"切换到开发者模式",
	"解锁限制",
}

type InjectionRisk struct {
	Detected bool   `json:"detected"`
	Level    string `json:"level"`
	Pattern  string `json:"pattern,omitempty"`
	Message  string `json:"message,omitempty"`
}

func DetectPromptInjection(input string) InjectionRisk {
	trimmed := strings.TrimSpace(input)
	if len(trimmed) == 0 {
		return InjectionRisk{Detected: false}
	}

	lower := strings.ToLower(trimmed)

	for _, kw := range instructionOverrideKeywords {
		if strings.Contains(lower, kw) {
			return InjectionRisk{
				Detected: true,
				Level:    "high",
				Pattern:  kw,
				Message:  fmt.Sprintf("检测到可能的指令劫持: %q", kw),
			}
		}
	}

	for _, pat := range injectionPatterns {
		if pat.MatchString(trimmed) {
			return InjectionRisk{
				Detected: true,
				Level:    "high",
				Pattern:  pat.String(),
				Message:  fmt.Sprintf("检测到注入模式: %s", pat.String()),
			}
		}
	}

	if hasRoleSpoofing(trimmed) {
		return InjectionRisk{
			Detected: true,
			Level:    "medium",
			Pattern:  "role_spoofing",
			Message:  "检测到角色伪装尝试",
		}
	}

	return InjectionRisk{Detected: false}
}

var sanitizePatterns = []struct {
	re   *regexp.Regexp
	repl string
}{
	{regexp.MustCompile(`(?s)<\|im_start\|>.*?<\|im_end\|>`), "[filtered]"},
	{regexp.MustCompile(`(?is)<!--\s*system.*?-->`), "[filtered]"},
	{regexp.MustCompile(`(?is)\[SYSTEM\].*?\[/SYSTEM\]`), "[filtered]"},
	{regexp.MustCompile(`(?is)<system>.*?</system>`), "[filtered]"},
	{regexp.MustCompile(`(?is)###\s*system\s*:.*`), "[filtered]"},
	{regexp.MustCompile(`(?i)\n\s*human\s*:\s*\n`), "\n[filtered]\n"},
	{regexp.MustCompile(`(?i)\n\s*assistant\s*:\s*\n`), "\n[filtered]\n"},
}

func SanitizeUserInput(input string) string {
	s := input
	for _, p := range sanitizePatterns {
		s = p.re.ReplaceAllString(s, p.repl)
	}
	return s
}

func hasRoleSpoofing(input string) bool {
	patterns := []string{
		"system:",
		"assistant:",
		"<|im_start|>",
		"<|im_sep|>",
	}
	lower := strings.ToLower(input)
	for _, p := range patterns {
		if strings.Contains(lower, p) {
			return true
		}
	}
	return false
}
