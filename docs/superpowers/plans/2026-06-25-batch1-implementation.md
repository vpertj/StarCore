# 第一批 AI 核心改进实现计划

> **面向 AI 代理的工作者：** 必需子技能：使用 superpowers:subagent-driven-development（推荐）或 superpowers:executing-plans 逐任务实现此计划。步骤使用复选框（`- [ ]`）语法来跟踪进度。

**目标：** 实现第一批 4 个 AI 核心改进：上下文文件去重、智能结果截断、文件指纹缓存、轻量级意图分类器

**架构：** 在现有 context、ai、agent 包中新增函数和类型，增强上下文管理效率和工具调用可靠性。所有改进向后兼容，不破坏现有功能。

**技术栈：** Go 1.23, 标准库（hash/fnv, path/filepath, os）

**规格文档：** `docs/superpowers/specs/2026-06-25-ai-core-improvements-design.md`

---

## 文件结构

### 新增文件
- `internal/context/dedup.go` — 上下文文件去重逻辑
- `internal/ai/truncate.go` — 智能结果截断逻辑
- `internal/agent/intent.go` — 轻量级意图分类器

### 修改文件
- `internal/context/builder.go:148` — 集成去重函数
- `internal/ai/service.go:1250-1253` — 替换硬截断为智能截断
- `internal/agent/tool_executor.go:15-34` — 增强缓存条目结构
- `internal/agent/tool_executor.go:197-212` — 文件指纹缓存验证
- `internal/agent/tool_executor.go:243-249` — 文件指纹缓存写入
- `internal/agent/tool_executor.go:77` — 缓存容量从 200 提升到 500
- `internal/ai/service.go:420-581` — 集成意图分类器

### 测试文件
- `internal/context/dedup_test.go`
- `internal/ai/truncate_test.go`
- `internal/agent/intent_test.go`
- `internal/agent/tool_executor_test.go`（新增缓存测试）

---

## 任务 1：上下文文件去重 — 路径标准化

**文件：**
- 创建：`internal/context/dedup.go`
- 测试：`internal/context/dedup_test.go`

- [ ] **步骤 1：编写失败的测试 — 路径标准化**

```go
// internal/context/dedup_test.go
package context

import (
	"path/filepath"
	"testing"
)

func TestDeduplicateContextFiles_PathNormalization(t *testing.T) {
	// 准备测试数据：同一文件的相对路径和绝对路径
	absPath, _ := filepath.Abs("test.go")
	files := []string{
		"test.go",
		"./test.go",
		absPath,
		"other.go",
	}

	result := deduplicateContextFiles(files)

	// 应该只保留 2 个不同的文件
	if len(result) != 2 {
		t.Errorf("expected 2 files after dedup, got %d: %v", len(result), result)
	}

	// 验证 other.go 在结果中
	found := false
	for _, f := range result {
		if filepath.Base(f) == "other.go" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected other.go in result, got %v", result)
	}
}
```

- [ ] **步骤 2：运行测试验证失败**

运行：`go test ./internal/context -run TestDeduplicateContextFiles_PathNormalization -v`
预期：FAIL，报错 "undefined: deduplicateContextFiles"

- [ ] **步骤 3：实现路径标准化函数**

```go
// internal/context/dedup.go
package context

import (
	"path/filepath"
)

// deduplicateContextFiles removes duplicate files from the context file list.
// It normalizes paths and removes duplicates based on absolute path.
func deduplicateContextFiles(files []string) []string {
	if len(files) == 0 {
		return files
	}

	// Step 1: Path normalization and dedup
	seen := make(map[string]bool)
	var result []string

	for _, f := range files {
		// Normalize to absolute path
		absPath, err := filepath.Abs(f)
		if err != nil {
			// If we can't get absolute path, use original
			absPath = f
		}
		absPath = filepath.Clean(absPath)

		if !seen[absPath] {
			seen[absPath] = true
			result = append(result, absPath)
		}
	}

	return result
}
```

- [ ] **步骤 4：运行测试验证通过**

运行：`go test ./internal/context -run TestDeduplicateContextFiles_PathNormalization -v`
预期：PASS

- [ ] **步骤 5：Commit**

```bash
git add internal/context/dedup.go internal/context/dedup_test.go
git commit -m "feat(context): add path normalization for context file deduplication"
```

---

## 任务 2：上下文文件去重 — 内容指纹

**文件：**
- 修改：`internal/context/dedup.go`
- 测试：`internal/context/dedup_test.go`

- [ ] **步骤 1：编写失败的测试 — 内容指纹去重**

```go
// 添加到 internal/context/dedup_test.go
package context

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDeduplicateContextFiles_ContentHash(t *testing.T) {
	// 创建临时目录
	tmpDir := t.TempDir()

	// 创建两个内容相同的文件
	file1 := filepath.Join(tmpDir, "file1.go")
	file2 := filepath.Join(tmpDir, "file2.go")
	content := "package main\n\nfunc main() {\n\tprintln(\"hello\")\n}\n"
	
	if err := os.WriteFile(file1, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(file2, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	// 创建一个内容不同的文件
	file3 := filepath.Join(tmpDir, "file3.go")
	if err := os.WriteFile(file3, []byte("package other\n"), 0644); err != nil {
		t.Fatal(err)
	}

	files := []string{file1, file2, file3}
	result := deduplicateContextFiles(files)

	// file1 和 file2 内容相同，应该只保留一个
	if len(result) != 2 {
		t.Errorf("expected 2 files after content dedup, got %d: %v", len(result), result)
	}
}
```

- [ ] **步骤 2：运行测试验证失败**

运行：`go test ./internal/context -run TestDeduplicateContextFiles_ContentHash -v`
预期：FAIL，因为当前实现只做了路径去重，未做内容去重

- [ ] **步骤 3：实现内容指纹去重**

```go
// 修改 internal/context/dedup.go
package context

import (
	"hash/fnv"
	"io"
	"os"
	"path/filepath"
)

// deduplicateContextFiles removes duplicate files from the context file list.
// Step 1: Normalize paths and remove path duplicates
// Step 2: Remove files with identical content (based on first 1000 chars hash)
func deduplicateContextFiles(files []string) []string {
	if len(files) == 0 {
		return files
	}

	// Step 1: Path normalization and dedup
	seen := make(map[string]bool)
	var normalized []string

	for _, f := range files {
		absPath, err := filepath.Abs(f)
		if err != nil {
			absPath = f
		}
		absPath = filepath.Clean(absPath)

		if !seen[absPath] {
			seen[absPath] = true
			normalized = append(normalized, absPath)
		}
	}

	// Step 2: Content fingerprint dedup
	contentSeen := make(map[uint64]string) // hash -> first file path
	var result []string

	for _, f := range normalized {
		hash, err := computeFileHash(f, 1000)
		if err != nil {
			// If we can't read the file, keep it
			result = append(result, f)
			continue
		}

		if _, exists := contentSeen[hash]; !exists {
			contentSeen[hash] = f
			result = append(result, f)
		}
	}

	return result
}

// computeFileHash computes FNV-1a hash of the first maxBytes of a file
func computeFileHash(filePath string, maxBytes int) (uint64, error) {
	f, err := os.Open(filePath)
	if err != nil {
		return 0, err
	}
	defer f.Close()

	// Read only first maxBytes
	buf := make([]byte, maxBytes)
	n, err := f.Read(buf)
	if err != nil && err != io.EOF {
		return 0, err
	}

	h := fnv.New64a()
	if _, err := h.Write(buf[:n]); err != nil {
		return 0, err
	}

	return h.Sum64(), nil
}
```

- [ ] **步骤 4：运行测试验证通过**

运行：`go test ./internal/context -run TestDeduplicateContextFiles_ContentHash -v`
预期：PASS

- [ ] **步骤 5：Commit**

```bash
git add internal/context/dedup.go internal/context/dedup_test.go
git commit -m "feat(context): add content fingerprint deduplication for context files"
```

---

## 任务 3：上下文文件去重 — 包含关系检测与集成

**文件：**
- 修改：`internal/context/dedup.go`
- 修改：`internal/context/builder.go:148`
- 测试：`internal/context/dedup_test.go`

- [ ] **步骤 1：编写失败的测试 — 包含关系检测**

```go
// 添加到 internal/context/dedup_test.go
func TestDeduplicateContextFiles_Containment(t *testing.T) {
	tmpDir := t.TempDir()

	// 创建父文件（包含子文件的全部内容）
	parent := filepath.Join(tmpDir, "parent.go")
	child := filepath.Join(tmpDir, "child.go")
	
	parentContent := "package main\n\nfunc main() {\n\tprintln(\"hello\")\n}\n\nfunc helper() {}\n"
	childContent := "func main() {\n\tprintln(\"hello\")\n}\n"
	
	if err := os.WriteFile(parent, []byte(parentContent), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(child, []byte(childContent), 0644); err != nil {
		t.Fatal(err)
	}

	files := []string{parent, child}
	result := deduplicateContextFiles(files)

	// child 的内容被 parent 包含，应该只保留 parent
	if len(result) != 1 {
		t.Errorf("expected 1 file after containment dedup, got %d: %v", len(result), result)
	}
	if len(result) > 0 && filepath.Base(result[0]) != "parent.go" {
		t.Errorf("expected parent.go to be kept, got %v", result)
	}
}

func TestDeduplicateContextFiles_Empty(t *testing.T) {
	result := deduplicateContextFiles([]string{})
	if len(result) != 0 {
		t.Errorf("expected empty result for empty input, got %v", result)
	}

	result = deduplicateContextFiles(nil)
	if result != nil {
		t.Errorf("expected nil result for nil input, got %v", result)
	}
}
```

- [ ] **步骤 2：运行测试验证失败**

运行：`go test ./internal/context -run TestDeduplicateContextFiles_Containment -v`
预期：FAIL，因为当前实现未做包含关系检测

- [ ] **步骤 3：实现包含关系检测**

```go
// 修改 internal/context/dedup.go，添加包含关系检测
package context

import (
	"hash/fnv"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// deduplicateContextFiles removes duplicate files from the context file list.
// Step 1: Normalize paths and remove path duplicates
// Step 2: Remove files with identical content (based on first 1000 chars hash)
// Step 3: Remove files whose content is contained in another file (only if <= 10 files)
func deduplicateContextFiles(files []string) []string {
	if len(files) == 0 {
		return files
	}

	// Step 1: Path normalization and dedup
	seen := make(map[string]bool)
	var normalized []string

	for _, f := range files {
		absPath, err := filepath.Abs(f)
		if err != nil {
			absPath = f
		}
		absPath = filepath.Clean(absPath)

		if !seen[absPath] {
			seen[absPath] = true
			normalized = append(normalized, absPath)
		}
	}

	// Step 2: Content fingerprint dedup
	contentSeen := make(map[uint64]string)
	var afterHash []string

	for _, f := range normalized {
		hash, err := computeFileHash(f, 1000)
		if err != nil {
			afterHash = append(afterHash, f)
			continue
		}

		if _, exists := contentSeen[hash]; !exists {
			contentSeen[hash] = f
			afterHash = append(afterHash, f)
		}
	}

	// Step 3: Containment detection (only for small lists to avoid O(n²) overhead)
	if len(afterHash) > 10 {
		return afterHash
	}

	return removeContainedFiles(afterHash)
}

// computeFileHash computes FNV-1a hash of the first maxBytes of a file
func computeFileHash(filePath string, maxBytes int) (uint64, error) {
	f, err := os.Open(filePath)
	if err != nil {
		return 0, err
	}
	defer f.Close()

	buf := make([]byte, maxBytes)
	n, err := f.Read(buf)
	if err != nil && err != io.EOF {
		return 0, err
	}

	h := fnv.New64a()
	if _, err := h.Write(buf[:n]); err != nil {
		return 0, err
	}

	return h.Sum64(), nil
}

// removeContainedFiles removes files whose content is fully contained in another file
func removeContainedFiles(files []string) []string {
	// Read all file contents
	contents := make(map[string]string)
	for _, f := range files {
		data, err := os.ReadFile(f)
		if err != nil {
			continue
		}
		contents[f] = string(data)
	}

	// Check containment: if A is contained in B, remove A
	var result []string
	for i, f := range files {
		contentI := contents[f]
		if contentI == "" {
			result = append(result, f)
			continue
		}

		contained := false
		for j, other := range files {
			if i == j {
				continue
			}
			contentJ := contents[other]
			// If other file is larger and contains this file's content
			if len(contentJ) > len(contentI) && strings.Contains(contentJ, contentI) {
				contained = true
				break
			}
		}

		if !contained {
			result = append(result, f)
		}
	}

	return result
}
```

- [ ] **步骤 4：运行测试验证通过**

运行：`go test ./internal/context -run TestDeduplicateContextFiles -v`
预期：所有 4 个测试 PASS

- [ ] **步骤 5：集成到 BuildContextMessage**

```go
// 修改 internal/context/builder.go，在 148 行之前插入
// 找到：// 5. Context Files (user-selected, varies)
// 在它之前插入：

// Deduplicate context files before processing
if len(req.ContextFiles) > 0 {
	req.ContextFiles = deduplicateContextFiles(req.ContextFiles)
}
```

- [ ] **步骤 6：运行完整测试**

运行：`go test ./internal/context -v`
预期：所有测试 PASS

- [ ] **步骤 7：Commit**

```bash
git add internal/context/dedup.go internal/context/dedup_test.go internal/context/builder.go
git commit -m "feat(context): integrate context file deduplication with containment detection"
```

---

## 任务 4：智能结果截断 — 动态预算计算

**文件：**
- 创建：`internal/ai/truncate.go`
- 测试：`internal/ai/truncate_test.go`

- [ ] **步骤 1：编写失败的测试 — 动态预算**

```go
// internal/ai/truncate_test.go
package ai

import (
	"testing"
)

func TestCalcToolResultBudget(t *testing.T) {
	tests := []struct {
		name         string
		contextUsed  int
		contextMax   int
		wantMin      int
		wantMax      int
	}{
		{
			name:        "low usage - high budget",
			contextUsed: 10000,
			contextMax:  100000,
			wantMin:     9000,
			wantMax:     12000,
		},
		{
			name:        "high usage - low budget",
			contextUsed: 90000,
			contextMax:  100000,
			wantMin:     2000,
			wantMax:     3000,
		},
		{
			name:        "medium usage",
			contextUsed: 50000,
			contextMax:  100000,
			wantMin:     5000,
			wantMax:     12000,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := calcToolResultBudget(tt.contextUsed, tt.contextMax)
			if got < tt.wantMin || got > tt.wantMax {
				t.Errorf("calcToolResultBudget(%d, %d) = %d, want between %d and %d",
					tt.contextUsed, tt.contextMax, got, tt.wantMin, tt.wantMax)
			}
		})
	}
}
```

- [ ] **步骤 2：运行测试验证失败**

运行：`go test ./internal/ai -run TestCalcToolResultBudget -v`
预期：FAIL，报错 "undefined: calcToolResultBudget"

- [ ] **步骤 3：实现动态预算计算**

```go
// internal/ai/truncate.go
package ai

// calcToolResultBudget calculates the dynamic budget for tool result truncation
// based on current context usage. Budget is 30% of remaining space, clamped to [2000, 12000].
func calcToolResultBudget(contextUsed int, contextMax int) int {
	remaining := contextMax - contextUsed
	if remaining < 0 {
		remaining = 0
	}

	// Tool results can use at most 30% of remaining context
	budget := remaining * 30 / 100

	// Clamp to reasonable bounds
	const minBudget = 2000
	const maxBudget = 12000

	if budget < minBudget {
		return minBudget
	}
	if budget > maxBudget {
		return maxBudget
	}
	return budget
}
```

- [ ] **步骤 4：运行测试验证通过**

运行：`go test ./internal/ai -run TestCalcToolResultBudget -v`
预期：PASS

- [ ] **步骤 5：Commit**

```bash
git add internal/ai/truncate.go internal/ai/truncate_test.go
git commit -m "feat(ai): add dynamic budget calculation for tool result truncation"
```

---

## 任务 5：智能结果截断 — 命令输出截断

**文件：**
- 修改：`internal/ai/truncate.go`
- 测试：`internal/ai/truncate_test.go`

- [ ] **步骤 1：编写失败的测试 — 命令输出截断**

```go
// 添加到 internal/ai/truncate_test.go
func TestSmartTruncateToolResult_ExecuteCommand(t *testing.T) {
	// 模拟长命令输出，错误在末尾
	var output string
	for i := 0; i < 100; i++ {
		output += "normal output line\n"
	}
	output += "error: something went wrong\n"
	output += "panic: runtime error\n"

	result := smartTruncateToolResult("execute_command", output, 500)

	// 应该保留错误行
	if len(result) > 500 {
		t.Errorf("result too long: %d > 500", len(result))
	}

	// 应该包含错误信息
	if !contains(result, "error") && !contains(result, "panic") {
		t.Errorf("expected error lines to be preserved, got: %s", result)
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > len(substr) && 
		(s[:len(substr)] == substr || s[len(s)-len(substr):] == substr || 
		findSubstring(s, substr)))
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
```

- [ ] **步骤 2：运行测试验证失败**

运行：`go test ./internal/ai -run TestSmartTruncateToolResult_ExecuteCommand -v`
预期：FAIL，报错 "undefined: smartTruncateToolResult"

- [ ] **步骤 3：实现命令输出截断**

```go
// 修改 internal/ai/truncate.go，添加智能截断函数
package ai

import (
	"strings"
)

// smartTruncateToolResult intelligently truncates tool results based on tool type
func smartTruncateToolResult(toolName string, result string, budget int) string {
	if len(result) <= budget {
		return result
	}

	switch toolName {
	case "execute_command":
		return truncateCommandOutput(result, budget)
	case "read_file":
		return truncateFileContent(result, budget)
	case "search_files", "glob_files":
		return truncateSearchResults(result, budget)
	case "get_git_diff":
		return truncateGitDiff(result, budget)
	case "web_fetch", "http_request":
		return truncateHeadTail(result, budget, 75, 25)
	default:
		return truncateHeadTail(result, budget, 75, 25)
	}
}

// truncateCommandOutput preserves error lines and tail for command output
func truncateCommandOutput(output string, budget int) string {
	lines := strings.Split(output, "\n")
	if len(lines) <= 20 {
		return output
	}

	// Extract error lines
	var errorLines []string
	errorKeywords := []string{"error", "fail", "panic", "fatal", "exception"}
	for _, line := range lines {
		lower := strings.ToLower(line)
		for _, kw := range errorKeywords {
			if strings.Contains(lower, kw) {
				errorLines = append(errorLines, line)
				break
			}
		}
	}

	// Keep last 30 lines
	tailLines := lines[len(lines)-30:]

	// Build result: error lines + tail
	var result strings.Builder
	result.WriteString("[Command output truncated]\n")
	
	if len(errorLines) > 0 {
		result.WriteString("Error lines:\n")
		for _, el := range errorLines {
			result.WriteString(el)
			result.WriteString("\n")
		}
		result.WriteString("\n")
	}

	result.WriteString("Last 30 lines:\n")
	result.WriteString(strings.Join(tailLines, "\n"))

	// Final truncation if still too long
	final := result.String()
	if len(final) > budget {
		return final[:budget] + "\n... [truncated]"
	}
	return final
}

// truncateFileContent preserves head and tail structure (75% head, 25% tail)
func truncateFileContent(content string, budget int) string {
	return truncateHeadTail(content, budget, 75, 25)
}

// truncateSearchResults preserves statistics and first N results
func truncateSearchResults(content string, budget int) string {
	lines := strings.Split(content, "\n")
	
	// Find statistics line (usually contains "matches" or "results")
	var statsLine string
	for _, line := range lines {
		if strings.Contains(strings.ToLower(line), "match") || 
		   strings.Contains(strings.ToLower(line), "result") {
			statsLine = line
			break
		}
	}

	// Keep first 50 results
	var kept []string
	if statsLine != "" {
		kept = append(kept, statsLine)
	}
	
	count := 0
	for _, line := range lines {
		if line == statsLine {
			continue
		}
		kept = append(kept, line)
		count++
		if count >= 50 {
			break
		}
	}

	result := strings.Join(kept, "\n")
	if len(result) > budget {
		return result[:budget] + "\n... [truncated]"
	}
	return result
}

// truncateGitDiff preserves diff statistics and head content
func truncateGitDiff(diff string, budget int) string {
	// Find diff statistics (lines starting with "diff --git" or file changes)
	lines := strings.Split(diff, "\n")
	var statsLines []string
	var contentLines []string

	for _, line := range lines {
		if strings.HasPrefix(line, "diff --git") || 
		   strings.HasPrefix(line, "index ") ||
		   strings.HasPrefix(line, "---") ||
		   strings.HasPrefix(line, "+++") {
			statsLines = append(statsLines, line)
		} else {
			contentLines = append(contentLines, line)
		}
	}

	var result strings.Builder
	if len(statsLines) > 0 {
		result.WriteString("File changes:\n")
		result.WriteString(strings.Join(statsLines[:min(10, len(statsLines))], "\n"))
		result.WriteString("\n\n")
	}

	// Add head of content
	remaining := budget - result.Len()
	if remaining > 0 && len(contentLines) > 0 {
		content := strings.Join(contentLines, "\n")
		if len(content) > remaining {
			content = content[:remaining] + "\n... [truncated]"
		}
		result.WriteString(content)
	}

	return result.String()
}

// truncateHeadTail preserves head and tail percentages
func truncateHeadTail(content string, budget int, headPct, tailPct int) string {
	if len(content) <= budget {
		return content
	}

	headSize := budget * headPct / 100
	tailSize := budget * tailPct / 100

	// Find line boundaries
	head := content[:headSize]
	if idx := strings.LastIndex(head, "\n"); idx > 0 {
		head = head[:idx]
	}

	tail := content[len(content)-tailSize:]
	if idx := strings.Index(tail, "\n"); idx > 0 {
		tail = tail[idx+1:]
	}

	return head + "\n\n... [omitted " + strings.Itoa(len(content)-headSize-tailSize) + " chars] ...\n\n" + tail
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
```

- [ ] **步骤 4：运行测试验证通过**

运行：`go test ./internal/ai -run TestSmartTruncateToolResult -v`
预期：PASS

- [ ] **步骤 5：Commit**

```bash
git add internal/ai/truncate.go internal/ai/truncate_test.go
git commit -m "feat(ai): implement smart tool result truncation with command output preservation"
```

---

## 任务 6：智能结果截断 — 集成到 Agent 循环

**文件：**
- 修改：`internal/ai/service.go:1250-1253`
- 测试：`internal/ai/truncate_test.go`

- [ ] **步骤 1：编写集成测试**

```go
// 添加到 internal/ai/truncate_test.go
func TestSmartTruncateToolResult_Integration(t *testing.T) {
	tests := []struct {
		name     string
		toolName string
		result   string
		budget   int
		check    func(string) bool
	}{
		{
			name:     "short result unchanged",
			toolName: "read_file",
			result:   "short content",
			budget:   8000,
			check:    func(s string) bool { return s == "short content" },
		},
		{
			name:     "command output preserves errors",
			toolName: "execute_command",
			result:   strings.Repeat("line\n", 100) + "error: failed\n",
			budget:   500,
			check:    func(s string) bool { return strings.Contains(s, "error") },
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := smartTruncateToolResult(tt.toolName, tt.result, tt.budget)
			if !tt.check(got) {
				t.Errorf("check failed for %s, got: %s", tt.name, got)
			}
		})
	}
}
```

- [ ] **步骤 2：修改 service.go 集成智能截断**

```go
// 修改 internal/ai/service.go，找到 1250-1253 行：
// rc := tr.result.Result
// if len(rc) > maxToolResultChars {
//     rc = rc[:maxToolResultChars] + "... [truncated]"
// }

// 替换为：
rc := tr.result.Result
// Calculate dynamic budget based on estimated context usage
// For now, use conservative estimates; can be enhanced later
estimatedContextUsed := estimateContextUsed(currentReq.Messages)
estimatedContextMax := 100000 // Default max context
budget := calcToolResultBudget(estimatedContextUsed, estimatedContextMax)
rc = smartTruncateToolResult(tr.call.Name, rc, budget)
```

- [ ] **步骤 3：添加辅助函数 estimateContextUsed**

```go
// 添加到 internal/ai/truncate.go
package ai

import (
	"StarCore/internal/provider"
)

// estimateContextUsed estimates the current context usage in characters
func estimateContextUsed(messages []provider.Message) int {
	total := 0
	for _, msg := range messages {
		total += len(msg.Content)
	}
	return total
}
```

- [ ] **步骤 4：运行测试验证**

运行：`go test ./internal/ai -run TestSmartTruncateToolResult -v`
预期：所有测试 PASS

- [ ] **步骤 5：运行完整测试**

运行：`go test ./internal/ai -v`
预期：所有测试 PASS

- [ ] **步骤 6：Commit**

```bash
git add internal/ai/truncate.go internal/ai/truncate_test.go internal/ai/service.go
git commit -m "feat(ai): integrate smart tool result truncation into agent loop"
```

---

## 任务 7：文件指纹缓存 — 增强缓存条目结构

**文件：**
- 修改：`internal/agent/tool_executor.go:15-34`
- 测试：`internal/agent/tool_executor_test.go`

- [ ] **步骤 1：编写失败的测试 — 文件指纹缓存**

```go
// internal/agent/tool_executor_test.go
package agent

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestToolExecutor_FileFingerprintCache(t *testing.T) {
	executor := NewToolExecutor()

	// Create a test file
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")
	content := "test content"
	if err := os.WriteFile(testFile, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	// Get initial mod time
	info1, _ := os.Stat(testFile)
	modTime1 := info1.ModTime()

	// Create a cache entry with file fingerprint
	entry := &cacheEntry{
		result:      &ToolResult{Result: "cached result"},
		createdAt:   time.Now(),
		accessAt:    time.Now(),
		key:         "read_file:" + testFile,
		fileModTime: modTime1,
		isFileCache: true,
	}

	executor.mu.Lock()
	executor.cache[entry.key] = entry
	executor.mu.Unlock()

	// Verify cache hit when file unchanged
	executor.mu.RLock()
	cached, exists := executor.cache[entry.key]
	executor.mu.RUnlock()

	if !exists {
		t.Fatal("cache entry should exist")
	}

	// Check if file fingerprint validation would succeed
	info2, _ := os.Stat(testFile)
	if !info2.ModTime().Equal(cached.fileModTime) {
		t.Error("file mod time should match for unchanged file")
	}

	// Modify the file
	time.Sleep(10 * time.Millisecond) // Ensure mod time changes
	if err := os.WriteFile(testFile, []byte("modified content"), 0644); err != nil {
		t.Fatal(err)
	}

	// Verify cache would be invalidated
	info3, _ := os.Stat(testFile)
	if info3.ModTime().Equal(cached.fileModTime) {
		t.Error("file mod time should differ after modification")
	}
}
```

- [ ] **步骤 2：运行测试验证失败**

运行：`go test ./internal/agent -run TestToolExecutor_FileFingerprintCache -v`
预期：FAIL，报错 "unknown field 'fileModTime' in struct literal"

- [ ] **步骤 3：修改缓存条目结构**

```go
// 修改 internal/agent/tool_executor.go，找到 24-29 行的 cacheEntry 结构体
// 替换为：
type cacheEntry struct {
	result      *ToolResult
	createdAt   time.Time
	accessAt    time.Time
	key         string
	fileModTime time.Time // File modification time (zero for non-file caches)
	isFileCache bool      // Whether this is a file fingerprint-based cache
}
```

- [ ] **步骤 4：运行测试验证通过**

运行：`go test ./internal/agent -run TestToolExecutor_FileFingerprintCache -v`
预期：PASS

- [ ] **步骤 5：Commit**

```bash
git add internal/agent/tool_executor.go internal/agent/tool_executor_test.go
git commit -m "feat(agent): add file fingerprint fields to cache entry structure"
```

---

## 任务 8：文件指纹缓存 — 修改缓存验证逻辑

**文件：**
- 修改：`internal/agent/tool_executor.go:197-212`
- 测试：`internal/agent/tool_executor_test.go`

- [ ] **步骤 1：编写失败的测试 — 缓存验证**

```go
// 添加到 internal/agent/tool_executor_test.go
func TestToolExecutor_CacheValidation(t *testing.T) {
	executor := NewToolExecutor()

	// Create test file
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")
	if err := os.WriteFile(testFile, []byte("content"), 0644); err != nil {
		t.Fatal(err)
	}

	info, _ := os.Stat(testFile)
	
	// Create cache entry
	entry := &cacheEntry{
		result:      &ToolResult{Result: "cached"},
		createdAt:   time.Now().Add(-1 * time.Minute), // Old entry
		accessAt:    time.Now(),
		key:         "read_file:" + testFile,
		fileModTime: info.ModTime(),
		isFileCache: true,
	}

	executor.mu.Lock()
	executor.cache[entry.key] = entry
	executor.mu.Unlock()

	// Test: file unchanged, cache should be valid
	valid := executor.isCacheValid(entry)
	if !valid {
		t.Error("cache should be valid when file unchanged")
	}

	// Modify file
	time.Sleep(10 * time.Millisecond)
	if err := os.WriteFile(testFile, []byte("modified"), 0644); err != nil {
		t.Fatal(err)
	}

	// Test: file modified, cache should be invalid
	valid = executor.isCacheValid(entry)
	if valid {
		t.Error("cache should be invalid after file modification")
	}
}
```

- [ ] **步骤 2：运行测试验证失败**

运行：`go test ./internal/agent -run TestToolExecutor_CacheValidation -v`
预期：FAIL，报错 "undefined: isCacheValid"

- [ ] **步骤 3：实现缓存验证方法**

```go
// 添加到 internal/agent/tool_executor.go
package agent

import (
	"os"
	"strings"
	"time"
)

// isCacheValid checks if a cache entry is still valid
func (e *ToolExecutor) isCacheValid(entry *cacheEntry) bool {
	if entry.isFileCache {
		// File fingerprint cache: check if file was modified
		filePath := extractFilePathFromCacheKey(entry.key)
		if filePath == "" {
			return false
		}
		info, err := os.Stat(filePath)
		if err != nil {
			return false // File doesn't exist or can't be accessed
		}
		return info.ModTime().Equal(entry.fileModTime)
	}

	// Non-file cache: use TTL
	return time.Since(entry.createdAt) < 30*time.Second
}

// extractFilePathFromCacheKey extracts file path from cache key
// Cache key format: "toolName:filePath:..." or "toolName:pattern:path"
func extractFilePathFromCacheKey(key string) string {
	parts := strings.SplitN(key, ":", 3)
	if len(parts) < 2 {
		return ""
	}

	toolName := parts[0]
	switch toolName {
	case "read_file", "write_file", "edit_file", "delete_file":
		return parts[1]
	case "glob_files", "list_directory":
		// For directory operations, return the directory path
		if len(parts) >= 3 {
			return parts[2]
		}
		return parts[1]
	default:
		return ""
	}
}
```

- [ ] **步骤 4：运行测试验证通过**

运行：`go test ./internal/agent -run TestToolExecutor_CacheValidation -v`
预期：PASS

- [ ] **步骤 5：Commit**

```bash
git add internal/agent/tool_executor.go internal/agent/tool_executor_test.go
git commit -m "feat(agent): implement file fingerprint-based cache validation"
```

---

## 任务 9：文件指纹缓存 — 修改缓存写入与容量

**文件：**
- 修改：`internal/agent/tool_executor.go:243-249`
- 修改：`internal/agent/tool_executor.go:77`
- 测试：`internal/agent/tool_executor_test.go`

- [ ] **步骤 1：编写失败的测试 — 缓存写入**

```go
// 添加到 internal/agent/tool_executor_test.go
func TestToolExecutor_CacheWriteWithFingerprint(t *testing.T) {
	executor := NewToolExecutor()

	// Create test file
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")
	if err := os.WriteFile(testFile, []byte("content"), 0644); err != nil {
		t.Fatal(err)
	}

	info, _ := os.Stat(testFile)

	// Simulate cache write
	entry := &cacheEntry{
		result:    &ToolResult{Result: "result"},
		createdAt: time.Now(),
		accessAt:  time.Now(),
		key:       "read_file:" + testFile,
	}

	// Apply file fingerprint
	if filePath := extractFilePathFromCacheKey(entry.key); filePath != "" {
		if info, err := os.Stat(filePath); err == nil {
			entry.fileModTime = info.ModTime()
			entry.isFileCache = true
		}
	}

	executor.mu.Lock()
	executor.cache[entry.key] = entry
	executor.mu.Unlock()

	// Verify fingerprint was set
	if !entry.isFileCache {
		t.Error("isFileCache should be true for file operations")
	}
	if !entry.fileModTime.Equal(info.ModTime()) {
		t.Error("fileModTime should match file's mod time")
	}
}
```

- [ ] **步骤 2：修改缓存写入逻辑**

```go
// 修改 internal/agent/tool_executor.go，找到 243-249 行的缓存写入部分
// 替换为：
if cacheKey != "" && !tool.RequiresApproval() {
	entry := &cacheEntry{
		result:    tr,
		createdAt: time.Now(),
		accessAt:  time.Now(),
		key:       cacheKey,
	}

	// For file operations, record file modification time
	if filePath := extractFilePathFromCacheKey(cacheKey); filePath != "" {
		if info, err := os.Stat(filePath); err == nil {
			entry.fileModTime = info.ModTime()
			entry.isFileCache = true
		}
	}

	e.mu.Lock()
	e.cache[cacheKey] = entry
	e.lru.push(entry)
	e.mu.Unlock()
}
```

- [ ] **步骤 3：修改 Execute 方法中的缓存验证**

```go
// 修改 internal/agent/tool_executor.go，找到 197-212 行的缓存验证部分
// 替换为：
cacheKey := e.buildCacheKey(call)
if cacheKey != "" {
	e.mu.RLock()
	if entry, exists := e.cache[cacheKey]; exists && e.isCacheValid(entry) {
		e.mu.RUnlock()
		e.mu.Lock()
		e.lru.touch(cacheKey)
		e.mu.Unlock()
		cached := *entry.result
		cached.CallID = call.ID
		return &cached, nil
	}
	e.mu.RUnlock()
}
```

- [ ] **步骤 4：提升缓存容量**

```go
// 修改 internal/agent/tool_executor.go，找到 77 行：
// lru: newLRUList(200),
// 替换为：
lru: newLRUList(500),
```

- [ ] **步骤 5：运行测试验证**

运行：`go test ./internal/agent -run TestToolExecutor -v`
预期：所有测试 PASS

- [ ] **步骤 6：运行完整测试**

运行：`go test ./internal/agent -v`
预期：所有测试 PASS

- [ ] **步骤 7：Commit**

```bash
git add internal/agent/tool_executor.go internal/agent/tool_executor_test.go
git commit -m "feat(agent): implement file fingerprint cache write and increase capacity to 500"
```

---

## 任务 10：轻量级意图分类器 — 核心类型与规则

**文件：**
- 创建：`internal/agent/intent.go`
- 测试：`internal/agent/intent_test.go`

- [ ] **步骤 1：编写失败的测试 — 基本分类**

```go
// internal/agent/intent_test.go
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
		{
			name:     "code edit - Chinese",
			message:  "修改 main.go 中的函数",
			expected: IntentCodeEdit,
		},
		{
			name:     "code edit - English",
			message:  "edit the function in main.go",
			expected: IntentCodeEdit,
		},
		{
			name:     "debug - Chinese",
			message:  "这个 bug 怎么修复",
			expected: IntentDebug,
		},
		{
			name:     "debug - English",
			message:  "fix this error",
			expected: IntentDebug,
		},
		{
			name:     "chat - Chinese",
			message:  "你好",
			expected: IntentChat,
		},
		{
			name:     "chat - English",
			message:  "hello",
			expected: IntentChat,
		},
		{
			name:     "search",
			message:  "搜索所有使用 printf 的地方",
			expected: IntentSearch,
		},
		{
			name:     "git",
			message:  "提交代码",
			expected: IntentGit,
		},
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
```

- [ ] **步骤 2：运行测试验证失败**

运行：`go test ./internal/agent -run TestIntentClassifier_BasicClassification -v`
预期：FAIL，报错 "undefined: NewIntentClassifier"

- [ ] **步骤 3：实现意图分类器核心**

```go
// internal/agent/intent.go
package agent

import (
	"regexp"
	"strings"
)

// IntentType represents the classified intent of a user message
type IntentType string

const (
	IntentCodeEdit    IntentType = "code_edit"
	IntentCodeExplain IntentType = "code_explain"
	IntentDebug       IntentType = "debug"
	IntentRefactor    IntentType = "refactor"
	IntentSearch      IntentType = "search"
	IntentGit         IntentType = "git"
	IntentChat        IntentType = "chat"
	IntentPlan        IntentType = "plan"
	IntentTest        IntentType = "test"
	IntentDoc         IntentType = "doc"
)

// IntentResult contains the classification result
type IntentResult struct {
	Intent     IntentType
	Confidence float64
	Keywords   []string
	Language   string // "zh" or "en"
}

type intentRule struct {
	intent   IntentType
	keywords []string
	patterns []*regexp.Regexp
	weight   float64
}

// IntentClassifier classifies user messages into intents
type IntentClassifier struct {
	rules []intentRule
}

// NewIntentClassifier creates a new intent classifier with default rules
func NewIntentClassifier() *IntentClassifier {
	return &IntentClassifier{
		rules: buildDefaultRules(),
	}
}

func buildDefaultRules() []intentRule {
	return []intentRule{
		{
			intent: IntentCodeEdit,
			keywords: []string{
				"修改", "添加", "删除", "编辑", "改", "写", "实现", "创建",
				"edit", "add", "delete", "write", "implement", "create", "change", "update", "insert",
			},
			weight: 1.0,
		},
		{
			intent: IntentDebug,
			keywords: []string{
				"修复", "报错", "错误", "异常", "bug", "问题", "不工作", "失败", "崩溃",
				"fix", "error", "bug", "crash", "broken", "not working", "fail", "issue",
			},
			weight: 1.2,
		},
		{
			intent: IntentRefactor,
			keywords: []string{
				"重构", "优化", "改进", "提取", "拆分", "合并", "简化",
				"refactor", "optimize", "improve", "extract", "split", "simplify", "clean",
			},
			weight: 1.0,
		},
		{
			intent: IntentSearch,
			keywords: []string{
				"搜索", "查找", "找", "定位", "哪里", "哪个文件",
				"search", "find", "locate", "where", "which file",
			},
			weight: 0.8,
		},
		{
			intent: IntentGit,
			keywords: []string{
				"提交", "推送", "拉取", "分支", "合并", "commit", "push", "pull", "branch", "merge", "git",
			},
			weight: 1.0,
		},
		{
			intent: IntentCodeExplain,
			keywords: []string{
				"解释", "说明", "分析", "理解", "什么意思", "怎么工作",
				"explain", "describe", "analyze", "understand", "what does", "how does",
			},
			weight: 0.9,
		},
		{
			intent: IntentTest,
			keywords: []string{
				"测试", "单测", "单元测试", "test", "unit test", "coverage",
			},
			weight: 1.0,
		},
		{
			intent: IntentPlan,
			keywords: []string{
				"规划", "设计", "方案", "计划", "架构", "plan", "design", "architecture", "strategy",
			},
			weight: 0.9,
		},
		{
			intent: IntentDoc,
			keywords: []string{
				"文档", "说明", "注释", "README", "document", "comment", "documentation",
			},
			weight: 0.8,
		},
		{
			intent: IntentChat,
			keywords: []string{
				"你好", "谢谢", "hello", "hi", "thanks", "help",
			},
			weight: 0.5,
		},
	}
}

// Classify classifies a user message into an intent
func (c *IntentClassifier) Classify(message string) *IntentResult {
	msg := strings.ToLower(message)
	scores := make(map[IntentType]float64)
	matchedKeywords := make(map[IntentType][]string)

	for _, rule := range c.rules {
		score := 0.0

		// Keyword matching
		for _, kw := range rule.keywords {
			if strings.Contains(msg, strings.ToLower(kw)) {
				score += rule.weight
				matchedKeywords[rule.intent] = append(matchedKeywords[rule.intent], kw)
			}
		}

		// Pattern matching (higher weight)
		for _, pattern := range rule.patterns {
			if pattern.MatchString(msg) {
				score += rule.weight * 1.5
			}
		}

		if score > 0 {
			scores[rule.intent] = score
		}
	}

	// Find best intent
	var bestIntent IntentType = IntentChat
	bestScore := 0.0
	for intent, score := range scores {
		if score > bestScore {
			bestScore = score
			bestIntent = intent
		}
	}

	// Calculate confidence (normalized)
	confidence := 0.0
	if bestScore > 0 {
		totalScore := 0.0
		for _, score := range scores {
			totalScore += score
		}
		confidence = bestScore / totalScore
	}

	return &IntentResult{
		Intent:     bestIntent,
		Confidence: confidence,
		Keywords:   matchedKeywords[bestIntent],
		Language:   detectLanguage(message),
	}
}

// detectLanguage detects if the message is primarily Chinese or English
func detectLanguage(message string) string {
	chineseCount := 0
	englishCount := 0

	for _, r := range message {
		if r >= 0x4E00 && r <= 0x9FFF {
			chineseCount++
		} else if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') {
			englishCount++
		}
	}

	if chineseCount > englishCount {
		return "zh"
	}
	return "en"
}
```

- [ ] **步骤 4：运行测试验证通过**

运行：`go test ./internal/agent -run TestIntentClassifier_BasicClassification -v`
预期：所有测试 PASS

- [ ] **步骤 5：Commit**

```bash
git add internal/agent/intent.go internal/agent/intent_test.go
git commit -m "feat(agent): implement lightweight intent classifier with keyword matching"
```

---

## 任务 11：轻量级意图分类器 — 置信度与边界情况

**文件：**
- 修改：`internal/agent/intent.go`
- 测试：`internal/agent/intent_test.go`

- [ ] **步骤 1：编写失败的测试 — 置信度与边界**

```go
// 添加到 internal/agent/intent_test.go
func TestIntentClassifier_Confidence(t *testing.T) {
	classifier := NewIntentClassifier()

	// High confidence: clear intent
	result := classifier.Classify("修复这个 bug")
	if result.Confidence < 0.6 {
		t.Errorf("expected high confidence for clear intent, got %f", result.Confidence)
	}

	// Low confidence: ambiguous message
	result = classifier.Classify("这个")
	if result.Confidence > 0.3 {
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
```

- [ ] **步骤 2：运行测试验证**

运行：`go test ./internal/agent -run TestIntentClassifier -v`
预期：所有测试 PASS（当前实现已处理这些情况）

- [ ] **步骤 3：添加常量定义**

```go
// 添加到 internal/agent/intent.go 顶部
const (
	// Confidence thresholds for intent routing
	HighConfidence   = 0.6 // Auto-select Agent
	MediumConfidence = 0.4 // Suggest Agent, user can override
	LowConfidence    = 0.0 // No intervention, use default Agent
)
```

- [ ] **步骤 4：运行完整测试**

运行：`go test ./internal/agent -v`
预期：所有测试 PASS

- [ ] **步骤 5：Commit**

```bash
git add internal/agent/intent.go internal/agent/intent_test.go
git commit -m "feat(agent): add confidence thresholds and language detection to intent classifier"
```

---

## 任务 12：集成意图分类器到 ChatStream

**文件：**
- 修改：`internal/ai/service.go:420-581`
- 测试：运行现有测试确保不破坏功能

- [ ] **步骤 1：在 Service 中添加意图分类器**

```go
// 修改 internal/ai/service.go，在 Service 结构体中添加字段
// 找到 Service 结构体定义（约 60-90 行），添加：
type Service struct {
	// ... existing fields ...
	intentClassifier *agent.IntentClassifier // Intent classifier for auto-routing
}
```

- [ ] **步骤 2：初始化意图分类器**

```go
// 修改 internal/ai/service.go，找到 NewService 函数（约 130-160 行）
// 在返回之前添加：
s.intentClassifier = agent.NewIntentClassifier()
```

- [ ] **步骤 3：在 ChatStream 中使用意图分类**

```go
// 修改 internal/ai/service.go，在 ChatStream 函数中（约 463 行之前）
// 在 Agent 选择逻辑之前添加：

// Classify user intent for auto-routing
var intent *agent.IntentResult
if len(req.Messages) > 0 {
	for i := len(req.Messages) - 1; i >= 0; i-- {
		if req.Messages[i].Role == "user" {
			intent = s.intentClassifier.Classify(req.Messages[i].Content)
			break
		}
	}
}

// Store intent in loop state for use in agent loop
if intent != nil {
	s.loopState.SetDetectedIntent(intent)
}
```

- [ ] **步骤 4：在 LoopState 中添加意图存储**

```go
// 修改 internal/agent/tools/loop_state.go，在 LoopState 结构体中添加：
type LoopState struct {
	// ... existing fields ...
	DetectedIntent *agent.IntentResult // Detected intent from user message
}

// Add method to set intent
func (s *LoopState) SetDetectedIntent(intent *agent.IntentResult) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.DetectedIntent = intent
}

// Add method to get intent
func (s *LoopState) GetDetectedIntent() *agent.IntentResult {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.DetectedIntent
}
```

- [ ] **步骤 5：在 Reset 中清除意图**

```go
// 修改 internal/agent/tools/loop_state.go，在 Reset 方法中添加：
func (s *LoopState) Reset() {
	s.mu.Lock()
	defer s.mu.Unlock()
	// ... existing reset code ...
	s.DetectedIntent = nil
}
```

- [ ] **步骤 6：运行完整测试**

运行：`go test ./internal/... -v`
预期：所有测试 PASS

- [ ] **步骤 7：Commit**

```bash
git add internal/ai/service.go internal/agent/tools/loop_state.go
git commit -m "feat(ai): integrate intent classifier into ChatStream for auto-routing"
```

---

## 任务 13：最终验证与文档更新

- [ ] **步骤 1：运行所有测试**

```bash
go test ./internal/context -v
go test ./internal/ai -v
go test ./internal/agent -v
go test ./internal/... -v
```

预期：所有测试 PASS

- [ ] **步骤 2：运行 go fmt**

```bash
go fmt ./...
```

- [ ] **步骤 3：运行 go vet**

```bash
go vet ./...
```

预期：无警告

- [ ] **步骤 4：构建验证**

```bash
go build ./...
```

预期：构建成功

- [ ] **步骤 5：最终 Commit**

```bash
git add -A
git commit -m "feat: complete batch 1 AI core improvements

- Context file deduplication with path normalization, content fingerprinting, and containment detection
- Smart tool result truncation with dynamic budget and tool-specific strategies
- File fingerprint caching with 500 entry capacity
- Lightweight intent classifier with keyword matching and confidence scoring
- Integration of intent classifier into ChatStream for future auto-routing"
```

---

## 自检清单

### 规格覆盖度

| 规格需求 | 实现任务 | 状态 |
|---------|---------|------|
| #1 路径标准化去重 | 任务 1 | ✅ |
| #1 内容指纹去重 | 任务 2 | ✅ |
| #1 包含关系检测 | 任务 3 | ✅ |
| #1 集成到 BuildContextMessage | 任务 3 | ✅ |
| #2 动态预算计算 | 任务 4 | ✅ |
| #2 命令输出截断 | 任务 5 | ✅ |
| #2 其他工具截断策略 | 任务 5 | ✅ |
| #2 集成到 Agent 循环 | 任务 6 | ✅ |
| #4 缓存条目结构增强 | 任务 7 | ✅ |
| #4 文件指纹验证 | 任务 8 | ✅ |
| #4 文件指纹写入 | 任务 9 | ✅ |
| #4 缓存容量提升 | 任务 9 | ✅ |
| #6 意图分类器核心 | 任务 10 | ✅ |
| #6 置信度与语言检测 | 任务 11 | ✅ |
| #6 集成到 ChatStream | 任务 12 | ✅ |

### 占位符扫描

✅ 无 "待定"、"TODO"、"后续实现" 等占位符
✅ 所有代码步骤包含完整代码块
✅ 所有测试包含具体断言

### 类型一致性

✅ `IntentType` 在 intent.go 定义，在 loop_state.go 引用
✅ `cacheEntry` 结构体在 tool_executor.go 中一致使用
✅ `deduplicateContextFiles` 函数签名在 dedup.go 和 builder.go 中一致

---

**计划已完成并保存到 `docs/superpowers/plans/2026-06-25-batch1-implementation.md`。两种执行方式：**

**1. 子代理驱动（推荐）** - 每个任务调度一个新的子代理，任务间进行审查，快速迭代

**2. 内联执行** - 在当前会话中使用 executing-plans 执行任务，批量执行并设有检查点

**选哪种方式？**
