package agent

import (
	"testing"
)

func TestIntentClassifier_BasicClassification(t *testing.T) {
	classifier := NewIntentClassifier()

	tests := []struct {
		name     string
		message  string
		expected IntentType
	}{
		{"code edit Chinese", "修改 main.go 中的函数", IntentCodeEdit},
		{"code edit English", "edit the function in main.go", IntentCodeEdit},
		{"debug Chinese", "这个 bug 怎么修复", IntentDebug},
		{"debug English", "fix this error", IntentDebug},
		{"chat Chinese", "你好", IntentChat},
		{"chat English", "hello", IntentChat},
		{"search", "搜索所有使用 printf 的地方", IntentSearch},
		{"git", "提交代码", IntentGit},
		{"refactor", "重构这个模块", IntentRefactor},
		{"test", "写单元测试", IntentTest},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := classifier.Classify(tt.message)
			if result.Intent != tt.expected {
				t.Errorf("Classify(%q) = %v, want %v", tt.message, result.Intent, tt.expected)
			}
		})
	}
}

func TestIntentClassifier_Confidence(t *testing.T) {
	classifier := NewIntentClassifier()

	result := classifier.Classify("修复这个 bug")
	if result.Confidence < 0.6 {
		t.Errorf("expected high confidence for clear intent, got %f", result.Confidence)
	}

	result = classifier.Classify("这个")
	if result.Confidence > 0.5 {
		t.Errorf("expected low confidence for ambiguous message, got %f", result.Confidence)
	}
}

func TestIntentClassifier_EmptyMessage(t *testing.T) {
	classifier := NewIntentClassifier()
	result := classifier.Classify("")
	if result.Intent != IntentChat {
		t.Errorf("empty message should default to chat, got %v", result.Intent)
	}
}

func TestIntentClassifier_LanguageDetection(t *testing.T) {
	classifier := NewIntentClassifier()

	result := classifier.Classify("修改代码")
	if result.Language != "zh" {
		t.Errorf("expected Chinese, got %s", result.Language)
	}

	result = classifier.Classify("edit the code")
	if result.Language != "en" {
		t.Errorf("expected English, got %s", result.Language)
	}
}
