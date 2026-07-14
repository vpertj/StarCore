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

func TestEstimateTokensWithModel(t *testing.T) {
	// DeepSeek should give lower estimates for Chinese
	cnText := "你好世界这是一个测试"
	defaultEst := estimateTokensWithModel(cnText, "")
	dsEst := estimateTokensWithModel(cnText, "deepseek-chat")
	if dsEst >= defaultEst {
		t.Errorf("DeepSeek CJK estimate (%d) should be lower than default (%d)", dsEst, defaultEst)
	}

	// Claude should give higher estimates for CJK
	claudeEst := estimateTokensWithModel(cnText, "claude-3-5-sonnet")
	if claudeEst <= defaultEst {
		t.Errorf("Claude CJK estimate (%d) should be higher than default (%d)", claudeEst, defaultEst)
	}

	// English should be similar across models
	enText := "This is a test of English token estimation"
	gptEst := estimateTokensWithModel(enText, "gpt-4o")
	if gptEst <= 0 {
		t.Error("GPT English estimate should be positive")
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
		{[]provider.Message{{Role: "user", Content: "This is a long message that exceeds ten chars"}}, true}, // 48 chars, no tech keyword
		{[]provider.Message{{Role: "user", Content: "你看看我现在的项目还有什么问题吗"}}, false},                             // contains "问题" — tech keyword
		{[]provider.Message{{Role: "user", Content: "帮我看看这个项目怎么样"}}, true},                                   // conversational, no tech keyword
	}
	for i, tt := range tests {
		got := isSimpleMessage(tt.msgs)
		if got != tt.want {
			t.Errorf("test %d: isSimpleMessage(%q) = %v, want %v", i, tt.msgs[len(tt.msgs)-1].Content, got, tt.want)
		}
	}
}

func TestBuildToolSuppressHint(t *testing.T) {
	// Simple non-tech message → should suppress
	msgs := []provider.Message{{Role: "user", Content: "hi"}}
	hint := buildToolSuppressHint(msgs)
	if hint == "" {
		t.Error("should return suppress hint for simple message")
	}

	// Tech message → should NOT suppress
	msgs2 := []provider.Message{{Role: "user", Content: "fix the bug"}}
	hint2 := buildToolSuppressHint(msgs2)
	if hint2 != "" {
		t.Error("should not suppress for technical message")
	}

	// Longer conversational message → should suppress (≤50 chars, no tech keyword)
	msgs3 := []provider.Message{{Role: "user", Content: "你看看这个项目怎么样"}}
	hint3 := buildToolSuppressHint(msgs3)
	if hint3 == "" {
		t.Error("should suppress for conversational message")
	}
}

func TestPreCheckProvider_EmptyProviderID(t *testing.T) {
	mgr := provider.NewManager("", func() context.Context { return context.Background() })
	err := preCheckProvider(mgr, "nonexistent")
	if err == nil {
		t.Error("should error for nonexistent provider")
	}
}

func TestPruneMessages(t *testing.T) {
	// Create many system messages to simulate accumulated context
	msgs := make([]provider.Message, 0, 100)
	// 15 system messages (simulating accumulated Rules, Structure, Knowledge, RAG, summaries, etc.)
	for i := 0; i < 15; i++ {
		msgs = append(msgs, provider.Message{Role: "system", Content: "system msg"})
	}
	// 80 non-system messages
	for i := 0; i < 80; i++ {
		msgs = append(msgs, provider.Message{Role: "user", Content: "user msg"})
		msgs = append(msgs, provider.Message{Role: "assistant", Content: "assistant msg"})
	}
	// Total: 15 system + 160 non-system = 175 > 80 threshold

	result := pruneMessages(msgs, 60)

	// Should have trimmed system messages (prefix 6 + marker 1 + suffix 4 = 11 system)
	sysCount := 0
	for _, m := range result {
		if m.Role == "system" {
			sysCount++
		}
	}
	if sysCount > 12 {
		t.Errorf("system messages not trimmed: got %d, want <= 12", sysCount)
	}

	// Non-system messages should be <= 60 + possible summary marker
	otherCount := 0
	for _, m := range result {
		if m.Role != "system" {
			otherCount++
		}
	}
	if otherCount > 61 {
		t.Errorf("non-system messages not trimmed: got %d, want <= 61", otherCount)
	}

	// Total should be less than original
	if len(result) >= len(msgs) {
		t.Error("pruneMessages did not reduce message count")
	}
}

func TestPruneMessages_NoTrimNeeded(t *testing.T) {
	msgs := []provider.Message{
		{Role: "system", Content: "sys1"},
		{Role: "user", Content: "user1"},
	}
	result := pruneMessages(msgs, 60)
	if len(result) != len(msgs) {
		t.Errorf("should not trim small message list: got %d, want %d", len(result), len(msgs))
	}
}
