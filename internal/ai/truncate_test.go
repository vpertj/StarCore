package ai

import (
	"strconv"
	"strings"
	"testing"
)

func TestCalcToolResultBudget(t *testing.T) {
	tests := []struct {
		name        string
		contextUsed int
		contextMax  int
		wantMin     int
		wantMax     int
	}{
		{"low usage", 10000, 100000, 9000, 12000},
		{"high usage", 90000, 100000, 2000, 3000},
		{"medium usage", 50000, 100000, 5000, 12000},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := calcToolResultBudget(tt.contextUsed, tt.contextMax)
			if got < tt.wantMin || got > tt.wantMax {
				t.Errorf("calcToolResultBudget(%d, %d) = %d, want [%d, %d]",
					tt.contextUsed, tt.contextMax, got, tt.wantMin, tt.wantMax)
			}
		})
	}
}

func TestSmartTruncateToolResult_ShortUnchanged(t *testing.T) {
	got := smartTruncateToolResult("read_file", "short", 8000)
	if got != "short" {
		t.Errorf("expected unchanged, got %q", got)
	}
}

func TestSmartTruncateToolResult_CommandOutput(t *testing.T) {
	var output string
	for i := 0; i < 100; i++ {
		output += "normal line\n"
	}
	output += "error: something failed\n"

	got := smartTruncateToolResult("execute_command", output, 500)
	if len(got) > 600 {
		t.Errorf("result too long: %d", len(got))
	}
	if !strings.Contains(got, "error") {
		t.Error("expected error line preserved")
	}
}

func TestSmartTruncateToolResult_SearchResults(t *testing.T) {
	var content string
	content += "Found 200 matches\n"
	for i := 0; i < 200; i++ {
		content += "file" + strconv.Itoa(i) + ".go:10: match\n"
	}

	got := smartTruncateToolResult("search_files", content, 2000)
	if len(got) > 2100 {
		t.Errorf("result too long: %d", len(got))
	}
	if !strings.Contains(got, "200 matches") {
		t.Error("expected stats line preserved")
	}
}

func TestTruncateHeadTail(t *testing.T) {
	content := strings.Repeat("line\n", 100)
	got := truncateHeadTail(content, 200, 75, 25)
	if len(got) > 300 {
		t.Errorf("result too long: %d", len(got))
	}
	if !strings.Contains(got, "omitted") {
		t.Error("expected omission marker")
	}
}

func TestDetectTextRepetition_RepeatedLines(t *testing.T) {
	content := strings.Repeat("好的，我来全面审查你的项目\n", 5)
	if !detectTextRepetition(content) {
		t.Error("expected repetition detected for repeated lines")
	}
}

func TestDetectTextRepetition_RepeatedSentences(t *testing.T) {
	// Need > 200 chars total for detection to kick in
	content := strings.Repeat("好的，我来全面审查你的项目，先看看整体结构。", 6)
	if !detectTextRepetition(content) {
		t.Error("expected repetition detected for repeated sentences")
	}
}

func TestDetectTextRepetition_NoRepetition(t *testing.T) {
	content := "这是一段正常的技术分析文本，包含不同内容。每句话都不一样。没有重复。"
	if detectTextRepetition(content) {
		t.Error("expected no repetition for normal text")
	}
}

func TestDetectTextRepetition_TooShort(t *testing.T) {
	content := "短文本"
	if detectTextRepetition(content) {
		t.Error("expected no repetition for short text")
	}
}
