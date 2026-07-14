package ai

import (
	"testing"

	"StarCore/internal/agent"
)

func TestNewUnderstander(t *testing.T) {
	u := NewUnderstander()
	if u == nil {
		t.Fatal("NewUnderstander returned nil")
	}
	if u.intentClassifier == nil {
		t.Fatal("intentClassifier should not be nil")
	}
	if u.filePattern == nil {
		t.Fatal("filePattern should not be nil")
	}
	if u.funcPattern == nil {
		t.Fatal("funcPattern should not be nil")
	}
	if u.linePattern == nil {
		t.Fatal("linePattern should not be nil")
	}
}

func TestUnderstand_EmptyMessage(t *testing.T) {
	u := NewUnderstander()
	result := u.Understand("")
	if result.Intent != agent.IntentChat {
		t.Errorf("empty message should default to chat intent, got %s", result.Intent)
	}
	if result.Ambiguity != AmbiguityHigh {
		t.Errorf("empty message should be high ambiguity, got %s", result.Ambiguity)
	}
	if result.Clarification == nil {
		t.Error("empty message should have clarification")
	}
}

func TestUnderstand_CodeEdit(t *testing.T) {
	u := NewUnderstander()
	result := u.Understand("修改 main.go 中的 handleRequest 函数")
	if result.Intent != agent.IntentCodeEdit {
		t.Errorf("expected code_edit intent, got %s", result.Intent)
	}
	if result.Confidence < agent.LowConfidence {
		t.Errorf("confidence too low: %f", result.Confidence)
	}
	if len(result.Entities.Files) == 0 {
		t.Error("should detect file reference")
	}
}

func TestUnderstand_DebugIntent(t *testing.T) {
	u := NewUnderstander()
	result := u.Understand("fix the bug in auth.go line 42")
	if result.Intent != agent.IntentDebug {
		t.Errorf("expected debug intent, got %s", result.Intent)
	}
}

func TestUnderstand_ExplainIntent(t *testing.T) {
	u := NewUnderstander()
	result := u.Understand("explain how the router works")
	if result.Intent != agent.IntentCodeExplain {
		t.Errorf("expected code_explain intent, got %s", result.Intent)
	}
}

func TestUnderstand_SearchIntent(t *testing.T) {
	u := NewUnderstander()
	result := u.Understand("find where the config is loaded")
	if result.Intent != agent.IntentSearch {
		t.Errorf("expected search intent, got %s", result.Intent)
	}
}

func TestUnderstand_PlanIntent(t *testing.T) {
	u := NewUnderstander()
	result := u.Understand("帮我设计一个插件系统的架构")
	if result.Intent != agent.IntentPlan {
		t.Errorf("expected plan intent, got %s", result.Intent)
	}
}

func TestExtractEntities_Files(t *testing.T) {
	u := NewUnderstander()
	entities := u.extractEntities("look at src/main.go and utils/helpers.ts")
	if len(entities.Files) < 2 {
		t.Errorf("expected at least 2 files, got %d: %v", len(entities.Files), entities.Files)
	}
}

func TestExtractEntities_Functions(t *testing.T) {
	u := NewUnderstander()
	entities := u.extractEntities("the handleRequest() and processData() functions need fixing")
	if len(entities.Functions) < 1 {
		t.Errorf("expected at least 1 function, got %d: %v", len(entities.Functions), entities.Functions)
	}
}

func TestExtractEntities_LineNumbers(t *testing.T) {
	u := NewUnderstander()
	entities := u.extractEntities("see line 42 and :100 in main.go")
	if len(entities.LineNumbers) < 2 {
		t.Errorf("expected at least 2 line numbers, got %d: %v", len(entities.LineNumbers), entities.LineNumbers)
	}
}

func TestAmbiguity_VeryShortMessage(t *testing.T) {
	u := NewUnderstander()
	result := u.Understand("修复")
	if result.Ambiguity < AmbiguityMedium {
		t.Errorf("very short message should be at least medium ambiguity, got %s", result.Ambiguity)
	}
}

func TestAmbiguity_ClearIntent(t *testing.T) {
	u := NewUnderstander()
	result := u.Understand("修改 src/handler.go 中的 authenticate 函数，添加输入验证")
	if result.Ambiguity > AmbiguityLow {
		t.Errorf("clear message with file+function should be low/no ambiguity, got %s", result.Ambiguity)
	}
}

func TestClarification_HighAmbiguity(t *testing.T) {
	u := NewUnderstander()
	result := u.Understand("fix")
	if result.Ambiguity < AmbiguityMedium {
		t.Errorf("single word 'fix' should be medium+ ambiguity, got %s", result.Ambiguity)
	}
	if result.Clarification == nil {
		t.Error("medium+ ambiguity should produce clarification")
	}
}

func TestCanProceed(t *testing.T) {
	u := NewUnderstander()

	// Clear intent — should proceed
	clear := u.Understand("修改 main.go 中的 handleRequest 函数添加验证")
	if !u.CanProceed(clear) {
		t.Error("should proceed with clear intent")
	}

	// Vague — should not proceed
	vague := u.Understand("fix")
	if u.CanProceed(vague) {
		t.Error("should not proceed with vague intent")
	}
}

func TestSuggestRoute_Mode(t *testing.T) {
	u := NewUnderstander()

	// Code edit → build
	edit := u.Understand("修改 main.go 中的函数")
	if edit.RouteHint.Mode != "build" {
		t.Errorf("code edit should route to build mode, got %s", edit.RouteHint.Mode)
	}

	// Chat → chat
	chat := u.Understand("hello, how are you")
	if chat.RouteHint.Mode != "chat" {
		t.Errorf("greeting should route to chat mode, got %s", chat.RouteHint.Mode)
	}
}

func TestFormatForFrontend(t *testing.T) {
	c := &Clarification{
		Question: "你想修改哪个文件？",
		Options:  []string{"main.go", "utils.go"},
		Context:  "未检测到文件参考",
		Priority: 1,
	}
	formatted := c.FormatForFrontend()
	if formatted == "" {
		t.Error("formatted clarification should not be empty")
	}
	// Should contain the question
	if !contains(formatted, "你想修改哪个文件") {
		t.Errorf("formatted should contain question, got: %s", formatted)
	}
}

func TestIsChineseMessage(t *testing.T) {
	if !IsChineseMessage("你好世界") {
		t.Error("Chinese message should be detected as Chinese")
	}
	if IsChineseMessage("hello world this is english") {
		t.Error("English message should not be detected as Chinese")
	}
}

func TestToWailsEvent(t *testing.T) {
	u := NewUnderstander()
	result := u.Understand("修改 main.go 添加验证")
	event, data := result.ToWailsEvent()
	if event != "ai:understander:result" {
		t.Errorf("unexpected event name: %s", event)
	}
	if data["intent"] == nil {
		t.Error("event data should contain intent")
	}
}

func TestUnderstander_ConfidenceFloor(t *testing.T) {
	u := NewUnderstander()
	result := u.Understand("hello there, nice weather today")
	if result.Confidence < 0 {
		t.Error("confidence should never be negative")
	}
	if result.Confidence > 1.0 {
		t.Error("confidence should never exceed 1.0")
	}
}

func TestUnderstand_MultiRequest(t *testing.T) {
	u := NewUnderstander()
	// Long message with many conjunctions → medium ambiguity
	result := u.Understand("修改 main.go 并且添加一个函数 还有修复 utils.go 同时优化 config.ts 另外重构 database.go 还要写文档")
	if result.Ambiguity < AmbiguityLow {
		t.Errorf("multi-request message should have at least low ambiguity, got %s", result.Ambiguity)
	}
}

// contains checks if substr is in s
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsHelper(s, substr))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
