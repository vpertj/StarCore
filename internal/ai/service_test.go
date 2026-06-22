package ai

import (
	"context"
	"testing"

	"StarCore/internal/provider"
)

func TestCalcMaxAgentLoops_Chat(t *testing.T) {
	if got := calcMaxAgentLoops("chat", "gpt-4o"); got != 15 {
		t.Errorf("chat mode = %d, want 15", got)
	}
}

func TestCalcMaxAgentLoops_Plan(t *testing.T) {
	if got := calcMaxAgentLoops("plan", "gpt-4o"); got != 35 {
		t.Errorf("plan mode = %d, want 35", got)
	}
}

func TestCalcMaxAgentLoops_Build(t *testing.T) {
	tests := []struct {
		model    string
		minLoops int
		maxLoops int
	}{
		{"gpt-4o", 35, 80},
		{"gpt-4o-mini", 35, 80},
		{"claude-3-5-sonnet-20241022", 35, 80},
		{"gpt-3.5-turbo", 15, 80},
		{"deepseek-coder", 15, 80},
	}
	for _, tt := range tests {
		got := calcMaxAgentLoops("build", tt.model)
		if got < tt.minLoops || got > tt.maxLoops {
			t.Errorf("build mode model=%s = %d, want [%d,%d]", tt.model, got, tt.minLoops, tt.maxLoops)
		}
	}
}

func TestCalcMaxAgentLoops_UnknownMode(t *testing.T) {
	got := calcMaxAgentLoops("unknown", "gpt-4o")
	if got <= 0 {
		t.Error("unknown mode should return positive value")
	}
}

func TestEstimateTokens(t *testing.T) {
	tests := []struct {
		text string
		min  int
	}{
		{"hello world", 2},
		{"你好世界", 4},
		{"", 0},
		{"func main() { fmt.Println(\"hello\") }", 5},
	}
	for _, tt := range tests {
		got := estimateTokens(tt.text)
		if got < tt.min {
			t.Errorf("estimateTokens(%q) = %d, want >= %d", tt.text, got, tt.min)
		}
	}
}

func TestIsSimpleMessage(t *testing.T) {
	tests := []struct {
		msgs []provider.Message
		want bool
	}{
		{[]provider.Message{{Role: "user", Content: "hi"}}, true},
		{[]provider.Message{{Role: "user", Content: "你好"}}, true},
		{[]provider.Message{{Role: "user", Content: "fix the bug"}}, false},
		{[]provider.Message{{Role: "user", Content: "请帮我重构这个函数"}}, false},
		{[]provider.Message{{Role: "user", Content: "This is a long message that exceeds ten chars"}}, false},
	}
	for i, tt := range tests {
		got := isSimpleMessage(tt.msgs)
		if got != tt.want {
			t.Errorf("test %d: isSimpleMessage = %v, want %v", i, got, tt.want)
		}
	}
}

func TestBuildToolSuppressHint(t *testing.T) {
	msgs := []provider.Message{{Role: "user", Content: "hi"}}
	hint := buildToolSuppressHint(msgs)
	if hint == "" {
		t.Error("should return suppress hint for simple message")
	}

	msgs2 := []provider.Message{{Role: "user", Content: "fix the bug"}}
	hint2 := buildToolSuppressHint(msgs2)
	if hint2 != "" {
		t.Error("should not suppress for technical message")
	}
}

func TestPreCheckProvider_EmptyProviderID(t *testing.T) {
	mgr := provider.NewManager("", func() context.Context { return context.Background() })
	err := preCheckProvider(mgr, "nonexistent")
	if err == nil {
		t.Error("should error for nonexistent provider")
	}
}
