package ai

import (
	"testing"

	"StarCore/internal/agent"
	"StarCore/internal/provider"
)

func TestTaskRouter_SimpleIntent(t *testing.T) {
	router := NewTaskRouter()
	intent := &agent.IntentResult{Intent: agent.IntentChat, Confidence: 0.8}
	messages := []provider.Message{{Role: "user", Content: "你好"}}

	route := router.Route(messages, intent)
	if route.Route != "direct" {
		t.Errorf("expected direct, got %s", route.Route)
	}
	if route.Complexity != ComplexitySimple {
		t.Errorf("expected simple, got %d", route.Complexity)
	}
}

func TestTaskRouter_ModerateIntent(t *testing.T) {
	router := NewTaskRouter()
	intent := &agent.IntentResult{Intent: agent.IntentCodeEdit, Confidence: 0.7}
	messages := []provider.Message{{Role: "user", Content: "修改 main.go 中的函数签名，添加一个新参数 ctx context.Context"}}

	route := router.Route(messages, intent)
	if route.Route != "agent" && route.Route != "decompose" {
		t.Errorf("expected agent or decompose, got %s", route.Route)
	}
}

func TestTaskRouter_ComplexIntent(t *testing.T) {
	router := NewTaskRouter()
	intent := &agent.IntentResult{Intent: agent.IntentRefactor, Confidence: 0.9}
	messages := []provider.Message{{Role: "user", Content: "重构所有文件，将 main.go 和 utils.go 和 handler.go 中的公共函数提取到 shared.go 中"}}

	route := router.Route(messages, intent)
	if route.Complexity != ComplexityComplex {
		t.Errorf("expected complex, got %d", route.Complexity)
	}
}

func TestExtractFileReferences(t *testing.T) {
	tests := []struct {
		name     string
		message  string
		expected int
	}{
		{"backtick paths", "修改 `main.go` 和 `utils.go`", 2},
		{"plain paths", "修改 src/main.go 和 src/utils.go", 2},
		{"no files", "你好世界", 0},
		{"single file", "读取 main.go", 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			refs := extractFileReferences(tt.message)
			if len(refs) != tt.expected {
				t.Errorf("expected %d refs, got %d: %v", tt.expected, len(refs), refs)
			}
		})
	}
}

func TestSplitByActions(t *testing.T) {
	tests := []struct {
		name     string
		message  string
		minParts int
	}{
		{"Chinese connectors", "读取 main.go 然后修改函数签名 接着运行测试", 2},
		{"English connectors", "read main.go then modify the function also run tests", 2},
		{"no connectors", "修改 main.go", 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parts := splitByActions(tt.message)
			if len(parts) < tt.minParts {
				t.Errorf("expected at least %d parts, got %d: %v", tt.minParts, len(parts), parts)
			}
		})
	}
}

func TestDecomposeTask_ByFiles(t *testing.T) {
	intent := &agent.IntentResult{Intent: agent.IntentRefactor}
	message := "重构 main.go 和 utils.go 和 handler.go"

	subTasks := decomposeTask(message, intent)
	if len(subTasks) < 2 {
		t.Errorf("expected at least 2 subtasks, got %d", len(subTasks))
	}
}

func TestDecomposeTask_ByActions(t *testing.T) {
	intent := &agent.IntentResult{Intent: agent.IntentCodeEdit}
	message := "读取配置文件 然后修改端口设置 接着重启服务"

	subTasks := decomposeTask(message, intent)
	if len(subTasks) < 2 {
		t.Errorf("expected at least 2 subtasks, got %d", len(subTasks))
	}
}

func TestDecomposeTask_NoSplit(t *testing.T) {
	intent := &agent.IntentResult{Intent: agent.IntentCodeEdit}
	message := "修改 main.go"

	subTasks := decomposeTask(message, intent)
	if len(subTasks) != 0 {
		t.Errorf("expected 0 subtasks for simple task, got %d", len(subTasks))
	}
}

func TestGetLastUserMessage(t *testing.T) {
	messages := []provider.Message{
		{Role: "system", Content: "system"},
		{Role: "user", Content: "first"},
		{Role: "assistant", Content: "response"},
		{Role: "user", Content: "second"},
	}
	got := getLastUserMessage(messages)
	if got != "second" {
		t.Errorf("expected 'second', got %q", got)
	}
}
