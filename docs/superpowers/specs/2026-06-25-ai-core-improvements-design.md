# StarCore AI 核心改进设计文档

**日期**: 2026-06-25
**范围**: 9 项 AI 核心系统改进
**目标**: 提升上下文管理效率、Agent 调度准确度、工具调用可靠性、弱模型兼容性

---

## 目录

1. [改进 #1: 上下文文件智能去重](#1-上下文文件智能去重)
2. [改进 #2: 结构化感知结果截断](#2-结构化感知结果截断)
3. [改进 #3: 分级错误恢复策略](#3-分级错误恢复策略)
4. [改进 #4: 文件指纹缓存](#4-文件指纹缓存)
5. [改进 #5: 工具选择规则引擎](#5-工具选择规则引擎)
6. [改进 #6: 轻量级意图分类器](#6-轻量级意图分类器)
7. [改进 #7: 自动上下文推荐](#7-自动上下文推荐)
8. [改进 #8: Agent 能力标签与自动推荐](#8-agent-能力标签与自动推荐)
9. [改进 #9: 意图路由与任务拆分](#9-意图路由与任务拆分)

---

## 1. 上下文文件智能去重

### 1.1 问题

`builder.go:148-165` 中，`ContextFiles` 列表直接遍历读取，无去重。用户可能通过不同路径引用同一文件（相对路径 vs 绝对路径、符号链接），或添加多个有包含关系的文件。

### 1.2 设计方案

**新增函数**: `deduplicateContextFiles(files []string) []string`

**位置**: `internal/context/builder.go`，在 `BuildContextMessage` 的动态后缀部分调用。

**算法**:

```
输入: ContextFiles 列表
│
├─ Step 1: 路径标准化
│   对每个文件调用 filepath.Abs() + filepath.Clean()
│   用 map[string]bool 去重（O(n)）
│
├─ Step 2: 内容指纹去重
│   对标准化后的文件列表：
│   - 读取每个文件的前 1000 字符
│   - 计算 FNV-1a 哈希（快速，无需外部依赖）
│   - 哈希相同的只保留第一个
│
├─ Step 3: 包含关系检测（可选，仅当文件数 <= 10 时执行）
│   对每对文件 (A, B)：
│   - 如果 len(A) < len(B)，检查 B 是否包含 A 的全部内容
│   - 如果包含，移除 A（保留更大的文件）
│   - 使用 strings.Contains 做子串匹配
│
└─ 输出: 去重后的文件列表
```

**集成点**:

```go
// builder.go:148 之前插入
if len(req.ContextFiles) > 0 {
    req.ContextFiles = deduplicateContextFiles(req.ContextFiles)
}
```

**性能约束**:
- Step 1: O(n)，纯内存操作
- Step 2: O(n × 1000)，只读前 1000 字符
- Step 3: O(n² × fileSize)，仅在 n <= 10 时执行，避免大列表的性能问题

**测试**:
- `TestDeduplicateContextFiles_PathNormalization`: 相对路径 vs 绝对路径
- `TestDeduplicateContextFiles_ContentHash`: 符号链接/相同内容
- `TestDeduplicateContextFiles_Containment`: 子文件被父文件包含
- `TestDeduplicateContextFiles_Empty`: 空列表不 panic

---

## 2. 结构化感知结果截断

### 2.1 问题

`service.go:1250-1253` 中，所有工具结果统一截断到 8000 字符。这对大文件读取、长命令输出等场景不够智能。

### 2.2 设计方案

**新增函数**: `smartTruncateToolResult(toolName string, result string, budget int) string`

**位置**: `internal/ai/service.go`，替换现有的 `maxToolResultChars` 硬截断。

**核心逻辑**:

```
输入: toolName, result, budget（动态预算，默认 8000）
│
├─ 如果 len(result) <= budget → 直接返回
│
├─ 根据 toolName 选择截断策略：
│
│   ├── read_file:
│   │   使用已有的 smartTruncate（头 75% + 尾 25%）
│   │   保留行号信息
│   │
│   ├── execute_command:
│   │   优先保留：
│   │   1. 最后 30 行（错误通常在末尾）
│   │   2. 包含 "error"/"fail"/"panic" 的行
│   │   3. 退出码信息
│   │   格式: [命令输出截断] 保留最后 N 行 + 关键错误行
│   │
│   ├── search_files / glob_files:
│   │   保留：
│   │   1. 结果统计（"共 N 个匹配"）
│   │   2. 前 50 条结果
│   │   3. 如果结果有行号，保留行号
│   │
│   ├── get_git_diff:
│   │   保留：
│   │   1. diff 统计摘要（文件变更统计）
│   │   2. 前 60% 的 diff 内容
│   │   3. 如果 diff 太长，只保留变更的文件名列表 + 每个文件的前 10 行 diff
│   │
│   ├── web_fetch / http_request:
│   │   保留头部 + 尾部（75%/25%），与 smartTruncate 相同
│   │
│   └── 默认:
│       使用 smartTruncate（头 75% + 尾 25%）
│
└─ 输出: 截断后的字符串 + 截断说明
```

**动态预算计算**:

```go
func calcToolResultBudget(contextUsed int, contextMax int) int {
    remaining := contextMax - contextUsed
    // 工具结果最多占剩余空间的 30%
    budget := remaining * 30 / 100
    // 下限 2000，上限 12000
    if budget < 2000 { budget = 2000 }
    if budget > 12000 { budget = 12000 }
    return budget
}
```

**集成点**:

```go
// service.go:1250-1253 替换为：
budget := calcToolResultBudget(estimatedContextUsed, estimatedContextMax)
rc := smartTruncateToolResult(tr.call.Name, tr.result.Result, budget)
```

**新增辅助函数**:

```go
// 提取命令输出中的关键错误行
func extractErrorLines(output string, maxLines int) []string

// 保留最后 N 行
func tailLines(s string, n int) string

// 提取 diff 统计摘要
func extractDiffStats(diff string) string
```

**测试**:
- `TestSmartTruncate_CommandOutput`: 保留错误行和尾部
- `TestSmartTruncate_SearchResults`: 保留统计和前 N 条
- `TestSmartTruncate_DynamicBudget`: 不同上下文使用率下的预算计算

---

## 3. 分级错误恢复策略

### 3.1 问题

`service.go:1233-1237` 中，所有工具错误统一作为 `tool` 消息注入对话，LLM 自行决定重试。缺乏结构化引导，弱模型容易陷入无效重试循环。

### 3.2 设计方案

**新增类型**: `ToolErrorClassifier`

**位置**: `internal/agent/tool_error.go`（新文件）

```go
type ErrorSeverity int

const (
    ErrorRetryable    ErrorSeverity = iota // 可自动重试：文件不存在、超时
    ErrorNeedsLLM                          // 需 LLM 介入：语法错误、逻辑错误
    ErrorFatal                             // 致命错误：安全沙箱拒绝、路径越界
)

type ClassifiedError struct {
    Severity    ErrorSeverity
    Category    string  // "file_not_found", "timeout", "syntax", "permission", "security"
    Message     string  // 用户可读的错误描述
    Suggestion  string  // 修复建议
    RetryTool   string  // 建议重试时使用的工具（可选）
    AutoRetry   bool    // 是否应自动重试
}
```

**错误分类规则**:

```
工具错误消息 → 关键词匹配 → 分类

├── "file not found" / "no such file" / "cannot find"
│   → ErrorRetryable, Category: "file_not_found"
│   → Suggestion: "请检查文件路径是否正确，或使用 glob_files 搜索文件"
│   → AutoRetry: false（让 LLM 修正路径）
│
├── "timeout" / "deadline exceeded"
│   → ErrorRetryable, Category: "timeout"
│   → Suggestion: "操作超时，可能是命令执行时间过长。考虑拆分操作或使用更短的超时"
│   → AutoRetry: true（自动重试 1 次）
│
├── "permission denied" / "access denied"
│   → ErrorNeedsLLM, Category: "permission"
│   → Suggestion: "权限不足。检查文件权限或使用其他路径"
│
├── "syntax error" / "parse error" / "compile error"
│   → ErrorNeedsLLM, Category: "syntax"
│   → Suggestion: "代码有语法错误。请使用 get_diagnostics 获取详细错误信息"
│   → 自动附加 get_diagnostics 结果
│
├── "path traversal" / "sandbox" / "security"
│   → ErrorFatal, Category: "security"
│   → Suggestion: "此操作被安全策略阻止。请在项目目录内操作"
│   → 不可重试
│
└── 默认
    → ErrorNeedsLLM, Category: "unknown"
    → Suggestion: "操作失败，请分析错误信息并尝试其他方法"
```

**自动重试机制**:

```go
// service.go 中工具执行后
if tr.err != nil {
    classified := agent.ClassifyToolError(tr.call.Name, tr.err)
    if classified.AutoRetry && retryCount[classified.Category] < 1 {
        // 自动重试 1 次，不消耗 Agent 循环
        retryCount[classified.Category]++
        // 重新执行工具
        // ...
    } else {
        // 注入结构化错误信息
        errorMsg := formatClassifiedError(classified)
        currentReq.Messages = append(currentReq.Messages, provider.Message{
            Role: "tool", Content: errorMsg, ToolCallID: tr.call.ID, Name: tr.call.Name,
        })
    }
}
```

**连续失败检测增强**:

在 `loop_state.go` 中新增：

```go
// 按工具名+错误类别追踪连续失败
func (s *LoopState) RecordToolFailure(toolName string, category string)
func (s *LoopState) GetConsecutiveFailures(toolName string) int

// 连续 3 次同类失败后注入"换策略"提示
if s.loopState.GetConsecutiveFailures(call.Name) >= 3 {
    currentReq.Messages = append(currentReq.Messages, provider.Message{
        Role: "system",
        Content: fmt.Sprintf("工具 %s 已连续失败 3 次。请换一种方法完成任务，或向用户说明困难。", call.Name),
    })
}
```

**自动诊断附加**:

当错误类别为 `syntax` 时，自动调用 `get_diagnostics` 获取 LSP 诊断信息并附加到错误消息中：

```go
if classified.Category == "syntax" {
    // 自动获取诊断信息
    // extractFilePathFromError: 从错误消息中用正则提取文件路径
    //   匹配模式: "file.go:10:5" 或 "path/to/file.ts(10,5)" 或 "./src/main.py line 42"
    //   如果无法提取，使用当前工具调用参数中的 path 字段
    filePath := extractFilePathFromError(classified.Message)
    if filePath == "" {
        filePath = extractFilePathFromToolArgs(call.Args)
    }
    if filePath != "" {
        if diagTool, ok := s.toolExec.Get("get_diagnostics"); ok {
            diagResult, diagErr := diagTool.Execute(ctx, map[string]any{
                "path": filePath,
            })
            if diagErr == nil && diagResult != "" {
                classified.Suggestion += "\n\n[LSP 诊断信息]\n" + diagResult
            }
        }
    }
}
```

**测试**:
- `TestClassifyToolError_FileNotFound`
- `TestClassifyToolError_Timeout`
- `TestClassifyToolError_Security`
- `TestAutoRetry_Timeout`
- `TestConsecutiveFailureDetection`

---

## 4. 文件指纹缓存

### 4.1 问题

`tool_executor.go:197-212` 中，缓存使用固定 30s TTL。文件未修改时缓存也会过期，导致重复读取。

### 4.2 设计方案

**修改 `cacheEntry` 结构体**:

```go
type cacheEntry struct {
    result    *ToolResult
    createdAt time.Time
    accessAt  time.Time
    key       string
    // 新增字段
    fileModTime time.Time  // 文件的最后修改时间（0 表示非文件缓存）
    isFileCache bool       // 是否基于文件指纹的缓存
}
```

**修改缓存验证逻辑**:

```go
// tool_executor.go:197-212 替换为：
if cacheKey != "" {
    e.mu.RLock()
    if entry, exists := e.cache[cacheKey]; exists {
        valid := false
        if entry.isFileCache {
            // 文件指纹缓存：检查文件是否修改
            if info, err := os.Stat(extractFilePathFromCacheKey(cacheKey)); err == nil {
                valid = info.ModTime().Equal(entry.fileModTime)
            }
        } else {
            // 非文件缓存：使用 TTL
            valid = time.Since(entry.createdAt) < 30*time.Second
        }
        if valid {
            e.mu.RUnlock()
            e.mu.Lock()
            e.lru.touch(cacheKey)
            e.mu.Unlock()
            cached := *entry.result
            cached.CallID = call.ID
            return &cached, nil
        }
    }
    e.mu.RUnlock()
}
```

**修改缓存写入逻辑**:

```go
// tool_executor.go:243-249 替换为：
if cacheKey != "" && !tool.RequiresApproval() {
    entry := &cacheEntry{
        result:    tr,
        createdAt: time.Now(),
        accessAt:  time.Now(),
        key:       cacheKey,
    }
    // 对文件操作工具记录文件修改时间
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

**缓存容量提升**:

```go
// tool_executor.go:77
// 从 200 提升到 500
lru: newLRUList(500),
```

**目录操作缓存优化**:

对 `glob_files` 和 `list_directory`，使用目录的 `mtime` 判断：

```go
// 在缓存写入时记录目录的 mtime
if isDirectoryCache(cacheKey) {
    dirPath := extractDirPathFromCacheKey(cacheKey)
    if info, err := os.Stat(dirPath); err == nil {
        entry.fileModTime = info.ModTime()
        entry.isFileCache = true
    }
}
```

**写操作失效增强**:

`InvalidateCacheForFile` 已经按前缀清除，无需修改。但新增对目录级缓存的失效：

```go
func (e *ToolExecutor) InvalidateCacheForDirectory(dirPath string) {
    e.mu.Lock()
    defer e.mu.Unlock()
    prefix := "glob_files:" + dirPath
    prefix2 := "list_directory:" + dirPath
    for k := range e.cache {
        if strings.HasPrefix(k, prefix) || strings.HasPrefix(k, prefix2) {
            delete(e.cache, k)
        }
    }
    e.lru.removeByKey(prefix)
    e.lru.removeByKey(prefix2)
}
```

**测试**:
- `TestFileFingerprintCache_UnmodifiedFile`: 文件未修改时缓存命中
- `TestFileFingerprintCache_ModifiedFile`: 文件修改后缓存失效
- `TestDirFingerprintCache`: 目录操作缓存
- `TestCacheCapacityIncrease`: 500 条容量

---

## 5. 工具选择规则引擎

### 5.1 问题

工具选择完全依赖 LLM 的 function calling 能力。弱模型（小参数 Ollama、旧版 GPT）可能频繁选错工具或不调用工具。

### 5.2 设计方案

**新增文件**: `internal/agent/tool_router.go`

**核心类型**:

```go
type ToolRouter struct {
    rules []RoutingRule
}

type RoutingRule struct {
    // 触发条件
    IntentKeywords []string // 意图关键词
    FilePattern    string   // 文件模式（可选）
    
    // 推荐工具
    PrimaryTool    string   // 首选工具
    FallbackTools  []string // 备选工具
    
    // 提示增强
    HintTemplate   string   // 提示模板
}
```

**预定义规则**:

```go
var defaultRules = []RoutingRule{
    {
        IntentKeywords: []string{"修改", "添加", "删除", "编辑", "change", "edit", "add", "remove", "update"},
        PrimaryTool:    "edit_file",
        FallbackTools:  []string{"write_file"},
        HintTemplate:   "用户要求修改代码。请使用 edit_file 工具进行精确修改，或 write_file 重写整个文件。",
    },
    {
        IntentKeywords: []string{"运行", "执行", "测试", "构建", "run", "execute", "test", "build"},
        PrimaryTool:    "execute_command",
        FallbackTools:  nil,
        HintTemplate:   "用户要求执行命令。请使用 execute_command 工具。",
    },
    {
        IntentKeywords: []string{"搜索", "查找", "定位", "search", "find", "locate", "where"},
        PrimaryTool:    "search_files",
        FallbackTools:  []string{"glob_files"},
        HintTemplate:   "用户要求搜索内容。请使用 search_files 搜索文件内容，或 glob_files 按文件名搜索。",
    },
    {
        IntentKeywords: []string{"读取", "查看", "打开", "read", "view", "open", "show"},
        PrimaryTool:    "read_file",
        FallbackTools:  nil,
        HintTemplate:   "用户要求查看文件。请使用 read_file 工具。",
    },
    {
        IntentKeywords: []string{"提交", "commit", "push", "pull", "git"},
        PrimaryTool:    "git_commit",
        FallbackTools:  []string{"get_git_diff", "execute_command"},
        HintTemplate:   "用户要求 Git 操作。请使用 git_commit/git_pull/git_push 工具。",
    },
    {
        IntentKeywords: []string{"解释", "分析", "理解", "explain", "analyze", "understand", "what"},
        PrimaryTool:    "read_file",
        FallbackTools:  []string{"search_files"},
        HintTemplate:   "用户要求解释代码。请先使用 read_file 读取相关文件，然后分析解释。",
    },
    {
        IntentKeywords: []string{"修复", "bug", "错误", "报错", "fix", "debug", "error"},
        PrimaryTool:    "read_file",
        FallbackTools:  []string{"search_files", "get_diagnostics"},
        HintTemplate:   "用户要求修复 bug。请先读取相关文件和错误信息，分析根因，然后使用 edit_file 修复。",
    },
}
```

**工具路由方法**:

```go
func (r *ToolRouter) SuggestTools(userMessage string) *RoutingSuggestion {
    // 1. 关键词匹配
    var matched []RoutingRule
    for _, rule := range r.rules {
        for _, kw := range rule.IntentKeywords {
            if strings.Contains(strings.ToLower(userMessage), kw) {
                matched = append(matched, rule)
                break
            }
        }
    }
    
    if len(matched) == 0 {
        return nil // 无匹配，不干预
    }
    
    // 2. 返回最佳匹配
    best := matched[0] // 规则按优先级排序
    return &RoutingSuggestion{
        PrimaryTool:   best.PrimaryTool,
        FallbackTools: best.FallbackTools,
        Hint:          best.HintTemplate,
    }
}
```

**集成方式 — 提示增强（非强制路由）**:

在 `service.go` 的 `runAgentLoop` 中，当检测到 LLM 未调用工具时：

```go
// 在 nudge 机制中增加工具建议
if !toolCallsSeen && assistantContent != "" {
    suggestion := s.toolRouter.SuggestTools(lastUserMessage)
    if suggestion != nil {
        nudgeContent += fmt.Sprintf("\n建议使用的工具: %s\n%s", suggestion.PrimaryTool, suggestion.Hint)
    }
}
```

**系统提示增强**:

在 Agent 系统提示中注入任务→工具映射表（仅对弱模型）：

```go
func buildToolMappingHint() string {
    return `
## 任务→工具映射
- 修改代码 → edit_file（精确修改）或 write_file（重写文件）
- 运行命令 → execute_command
- 搜索内容 → search_files（内容搜索）或 glob_files（文件名搜索）
- 查看文件 → read_file
- 修复 bug → read_file → 分析 → edit_file
- Git 操作 → get_git_diff / git_commit / git_pull / git_push
`
}
```

**测试**:
- `TestToolRouter_KeywordMatch`: 关键词匹配正确工具
- `TestToolRouter_NoMatch`: 无匹配时返回 nil
- `TestToolRouter_MultipleMatch`: 多规则匹配时返回最佳

---

## 6. 轻量级意图分类器

### 6.1 问题

无独立意图分类，所有请求走同一 Agent 循环。无法根据意图自动选择 Agent 或调整工具集。

### 6.2 设计方案

**新增文件**: `internal/agent/intent.go`

**核心类型**:

```go
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

type IntentResult struct {
    Intent     IntentType
    Confidence float64   // 0.0 - 1.0
    Keywords   []string  // 匹配到的关键词
    Language   string    // 检测到的语言（zh/en）
}

type IntentClassifier struct {
    rules []intentRule
}

type intentRule struct {
    intent     IntentType
    keywords   []string  // 中英文关键词
    patterns   []*regexp.Regexp  // 正则模式
    weight     float64   // 规则权重
}
```

**分类规则**:

```go
var intentRules = []intentRule{
    {
        intent:   IntentCodeEdit,
        keywords: []string{"修改", "添加", "删除", "编辑", "改", "写", "实现", "创建",
                           "edit", "add", "delete", "write", "implement", "create", "change", "update", "insert"},
        patterns: []*regexp.Regexp{
            regexp.MustCompile(`(在.+中|给.+)(添加|写|实现)`),
            regexp.MustCompile(`(把|将).+(改成|改为|换成)`),
        },
        weight: 1.0,
    },
    {
        intent:   IntentDebug,
        keywords: []string{"修复", "报错", "错误", "异常", "bug", "问题", "不工作", "失败", "崩溃",
                           "fix", "error", "bug", "crash", "broken", "not working", "fail", "issue"},
        patterns: []*regexp.Regexp{
            regexp.MustCompile(`(报错|错误|异常).+:`),
            regexp.MustCompile(`(为什么|why).+(不|没).+(工作|运行|显示)`),
        },
        weight: 1.2, // debug 意图权重更高
    },
    {
        intent:   IntentRefactor,
        keywords: []string{"重构", "优化", "改进", "提取", "拆分", "合并", "简化",
                           "refactor", "optimize", "improve", "extract", "split", "simplify", "clean"},
        weight: 1.0,
    },
    {
        intent:   IntentSearch,
        keywords: []string{"搜索", "查找", "找", "定位", "哪里", "哪个文件",
                           "search", "find", "locate", "where", "which file"},
        weight: 0.8,
    },
    {
        intent:   IntentGit,
        keywords: []string{"提交", "推送", "拉取", "分支", "合并", "commit", "push", "pull", "branch", "merge", "git"},
        weight: 1.0,
    },
    {
        intent:   IntentCodeExplain,
        keywords: []string{"解释", "说明", "分析", "理解", "什么意思", "怎么工作",
                           "explain", "describe", "analyze", "understand", "what does", "how does"},
        weight: 0.9,
    },
    {
        intent:   IntentTest,
        keywords: []string{"测试", "单测", "单元测试", "test", "unit test", "coverage"},
        weight: 1.0,
    },
    {
        intent:   IntentPlan,
        keywords: []string{"规划", "设计", "方案", "计划", "架构", "plan", "design", "architecture", "strategy"},
        weight: 0.9,
    },
    {
        intent:   IntentDoc,
        keywords: []string{"文档", "说明", "注释", "README", "document", "comment", "documentation"},
        weight: 0.8,
    },
    {
        intent:   IntentChat,
        keywords: []string{"你好", "谢谢", "hello", "hi", "thanks", "help"},
        weight: 0.5, // 低权重，作为兜底
    },
}
```

**分类算法**:

```go
func (c *IntentClassifier) Classify(message string) *IntentResult {
    msg := strings.ToLower(message)
    scores := make(map[IntentType]float64)
    matchedKeywords := make(map[IntentType][]string)
    
    for _, rule := range c.rules {
        score := 0.0
        
        // 关键词匹配
        for _, kw := range rule.keywords {
            if strings.Contains(msg, strings.ToLower(kw)) {
                score += rule.weight
                matchedKeywords[rule.intent] = append(matchedKeywords[rule.intent], kw)
            }
        }
        
        // 正则匹配（权重更高）
        for _, pattern := range rule.patterns {
            if pattern.MatchString(msg) {
                score += rule.weight * 1.5
            }
        }
        
        if score > 0 {
            scores[rule.intent] = score
        }
    }
    
    // 找最高分
    var bestIntent IntentType = IntentChat
    bestScore := 0.0
    for intent, score := range scores {
        if score > bestScore {
            bestScore = score
            bestIntent = intent
        }
    }
    
    // 计算置信度（归一化）
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
```

**置信度阈值**:

```go
const (
    HighConfidence   = 0.6  // 高置信度：自动选择 Agent
    MediumConfidence = 0.4  // 中置信度：推荐 Agent 但用户可覆盖
    LowConfidence    = 0.0  // 低置信度：不干预，使用默认 Agent
)
```

**集成点**:

在 `service.go` 的 `ChatStream` 中：

```go
// 在 Agent 选择之前
intent := s.intentClassifier.Classify(lastUserMessage)

// 如果用户未手动选择 Agent，且意图分类置信度高
if req.AgentID == "" && intent.Confidence >= HighConfidence {
    suggestedAgent := s.agentReg.FindByIntent(intent.Intent)
    if suggestedAgent != "" {
        req.AgentID = suggestedAgent
    }
}

// 将意图信息传递给 Agent 循环
s.loopState.SetDetectedIntent(intent)
```

**测试**:
- `TestIntentClassifier_CodeEdit`: "修改 main.go 中的函数" → code_edit
- `TestIntentClassifier_Debug`: "这个 bug 怎么修复" → debug
- `TestIntentClassifier_Chat`: "你好" → chat
- `TestIntentClassifier_LowConfidence`: 模糊消息 → 低置信度
- `TestIntentClassifier_MultiIntent`: 多意图消息 → 最高分胜出

---

## 7. 自动上下文推荐

### 7.1 问题

用户需手动添加上下文文件。系统不会自动判断"这个任务需要读哪些文件"。

### 7.2 设计方案

**新增文件**: `internal/context/auto_suggest.go`

**核心类型**:

```go
type ContextSuggestion struct {
    FilePath string
    Reason   string  // "依赖文件" / "相关文件" / "RAG 匹配"
    Score    float64 // 相关度分数
}

type ContextSuggester struct {
    projectPath string
}
```

**推荐策略（三层）**:

```
输入: 活动文件路径 + 用户查询 + 项目路径
│
├─ Layer 1: 依赖图推荐
│   利用 code_analysis.go 的 BuildDependencyGraph
│   从活动文件的 imports 中提取依赖文件
│   按依赖深度排序（直接依赖 > 间接依赖）
│   最多推荐 3 个
│
├─ Layer 2: 同目录/同模块推荐
│   查找活动文件同目录下的相关文件
│   优先推荐：同包的其他 .go 文件、同组件的其他 .svelte/.ts 文件
│   最多推荐 2 个
│
├─ Layer 3: 语义搜索推荐
│   复用已有的 RAG 搜索
│   用用户查询做语义检索
│   最多推荐 2 个
│
└─ 合并去重，按分数排序，取 top 5
    总字符不超过 10000
```

**集成点**:

在 `builder.go` 的 `BuildContextMessage` 中：

```go
// 在动态后缀部分，Context Files 之前
if req.ProjectPath != "" && req.ActiveFile != "" {
    suggester := NewContextSuggester(req.ProjectPath)
    suggestions := suggester.Suggest(req.ActiveFile, queryContext, 5)
    if len(suggestions) > 0 {
        var parts []string
        totalChars := 0
        for _, s := range suggestions {
            if totalChars > 10000 { break }
            data, err := os.ReadFile(s.FilePath)
            if err != nil { continue }
            content := string(data)
            if len(content) > 3000 { content = smartTruncate(content, 3000) }
            parts = append(parts, fmt.Sprintf("--- [Auto-Suggested: %s] %s ---\n%s", s.Reason, s.FilePath, content))
            totalChars += len(content)
        }
        if len(parts) > 0 {
            dynamicParts = append(dynamicParts, "[Auto-Suggested Context]\n以下文件与当前任务相关，已自动添加：\n\n"+strings.Join(parts, "\n\n"))
        }
    }
}
```

**性能约束**:
- 依赖图分析：`BuildContextMessage` 的步骤 0 已调用 `AnalyzeProjectStructure(projectPath, 50)`，将结果缓存到 `Builder.codeStructures` 字段（新增），供 `ContextSuggester` 复用。避免重复 Walk
- 同目录查找：`os.ReadDir` + 扩展名过滤，O(n) 目录扫描
- 语义搜索：复用 RAG，已有 3 结果限制
- 总延迟增加 < 100ms（依赖图已缓存，仅做图遍历 + 目录扫描）

**测试**:
- `TestAutoSuggest_DependencyFiles`: 活动文件的依赖文件被推荐
- `TestAutoSuggest_SameDirectory`: 同目录文件被推荐
- `TestAutoSuggest_CharLimit`: 总字符不超过 10000
- `TestAutoSuggest_NoDuplicates`: 不重复推荐已存在的文件

---

## 8. Agent 能力标签与自动推荐

### 8.1 问题

9 种 Agent 角色需用户手动选择。系统不会根据任务自动推荐。

### 8.2 设计方案

**修改 `AgentDef` 结构体**:

```go
// agent.go
type AgentDef struct {
    // ... 现有字段 ...
    Capabilities []IntentType `json:"capabilities"` // 能力标签
    Priority     int          `json:"priority"`      // 同能力下的优先级
}
```

**为现有 Agent 添加能力标签**:

```go
// builtins.go
{
    ID: "universal-assistant", Name: "全能助手",
    Capabilities: []IntentType{IntentCodeEdit, IntentDebug, IntentSearch, IntentChat, IntentCodeExplain},
    Priority: 0, // 最低优先级（兜底）
},
{
    ID: "frontend-architect", Name: "前端架构师",
    Capabilities: []IntentType{IntentCodeEdit, IntentRefactor, IntentCodeExplain},
    Priority: 10,
},
{
    ID: "backend-architect", Name: "后端架构师",
    Capabilities: []IntentType{IntentCodeEdit, IntentDebug, IntentRefactor, IntentPlan},
    Priority: 10,
},
{
    ID: "performance-expert", Name: "性能优化师",
    Capabilities: []IntentType{IntentRefactor, IntentDebug},
    Priority: 15, // 性能优化场景优先级更高
},
{
    ID: "api-test-engineer", Name: "API 测试工程师",
    Capabilities: []IntentType{IntentTest, IntentCodeEdit},
    Priority: 10,
},
{
    ID: "compliance-checker", Name: "合规审查员",
    Capabilities: []IntentType{IntentCodeExplain, IntentSearch},
    Priority: 10,
},
// ... 其他 Agent 类似
```

**新增 Registry 方法**:

```go
// registry.go
func (r *Registry) FindByIntent(intent IntentType) string {
    r.mu.RLock()
    defer r.mu.RUnlock()
    
    var bestAgent string
    bestPriority := -1
    
    for _, a := range r.agents {
        for _, cap := range a.Capabilities {
            if cap == intent && a.Priority > bestPriority {
                bestAgent = a.ID
                bestPriority = a.Priority
            }
        }
    }
    return bestAgent
}

func (r *Registry) SuggestAgents(intent IntentType, topN int) []AgentDef {
    r.mu.RLock()
    defer r.mu.RUnlock()
    
    var matched []AgentDef
    for _, a := range r.agents {
        for _, cap := range a.Capabilities {
            if cap == intent {
                matched = append(matched, a)
                break
            }
        }
    }
    
    // 按优先级排序
    sort.Slice(matched, func(i, j int) bool {
        return matched[i].Priority > matched[j].Priority
    })
    
    if len(matched) > topN {
        matched = matched[:topN]
    }
    return matched
}
```

**前端集成**:

通过事件通知前端显示推荐：

```go
// service.go ChatStream 中
if intent.Confidence >= MediumConfidence {
    suggestions := s.agentReg.SuggestAgents(intent.Intent, 3)
    if len(suggestions) > 0 {
        s.emitFn("ai:agent:suggested", map[string]any{
            "intent":      intent.Intent,
            "confidence":  intent.Confidence,
            "suggestions": suggestions,
        })
    }
}
```

**测试**:
- `TestRegistry_FindByIntent`: 根据意图找到最高优先级 Agent
- `TestRegistry_SuggestAgents`: 返回 top N 推荐
- `TestRegistry_Fallback`: 无匹配时返回全能助手

---

## 9. 意图路由与任务拆分

### 9.1 问题

所有请求走同一 Agent 循环。复杂任务无法自动拆分给子代理。

### 9.2 设计方案

**新增文件**: `internal/ai/task_router.go`

**核心类型**:

```go
type TaskComplexity int

const (
    ComplexitySimple   TaskComplexity = iota // 单步任务：问答、简单解释
    ComplexityModerate                       // 中等任务：单文件修改、简单 debug
    ComplexityComplex                        // 复杂任务：多文件修改、跨模块重构
)

type TaskRoute struct {
    Complexity TaskComplexity
    Intent     *IntentResult
    Route      string  // "direct" | "agent" | "decompose"
    SubTasks   []SubTaskSpec  // 仅当 Route == "decompose" 时
}

type SubTaskSpec struct {
    Description string
    AgentID     string  // 建议的 Agent
    Files       []string // 相关文件
    DependsOn   []int   // 依赖的子任务索引
}
```

**复杂度评估**:

```go
func evaluateComplexity(msgs []provider.ChatRequest, intent *IntentResult) TaskComplexity {
    lastMsg := getLastUserMessage(msgs)
    score := 0
    
    // 消息长度指标
    if len(lastMsg) > 500 { score += 2 }
    if len(lastMsg) > 200 { score += 1 }
    
    // 意图指标
    switch intent.Intent {
    case IntentChat, IntentCodeExplain:
        score += 0
    case IntentCodeEdit, IntentDebug, IntentTest:
        score += 1
    case IntentRefactor, IntentPlan:
        score += 2
    }
    
    // 关键词指标
    complexKeywords := []string{"所有", "全部", "整个", "多个", "重构", "迁移",
                                "all", "every", "multiple", "refactor", "migrate"}
    for _, kw := range complexKeywords {
        if strings.Contains(strings.ToLower(lastMsg), kw) {
            score += 2
            break
        }
    }
    
    // 文件引用指标
    fileRefs := countFileReferences(lastMsg)
    if fileRefs >= 3 { score += 2 }
    if fileRefs >= 1 { score += 1 }
    
    switch {
    case score <= 1:
        return ComplexitySimple
    case score <= 3:
        return ComplexityModerate
    default:
        return ComplexityComplex
    }
}
```

**任务拆分规则**:

```go
// extractFileReferences 从消息中提取文件路径引用
// 匹配模式：
//   - 显式路径: "src/main.go", "./components/App.tsx", "F:\code\file.go"
//   - 反引号包裹: `main.go`
//   - 文件扩展名关键词: ".go", ".ts", ".py", ".js" 等
// 返回去重后的文件路径列表
func extractFileReferences(message string) []string

// countFileReferences 返回 extractFileReferences 的结果长度
func countFileReferences(message string) int

// splitByActions 将消息按动作连接词拆分为多个独立动作
// 连接词: "然后", "接着", "之后", "并且", "并", "then", "and then", "after that", "also"
// 示例: "读取 main.go 然后修改函数签名并运行测试"
//   → ["读取 main.go", "修改函数签名", "运行测试"]
// 每个动作段至少 5 个字符，否则合并到前一个
func splitByActions(message string) []string

// dependentIndices 判断第 i 个动作是否依赖前面的动作
// 规则: 如果后面的动作引用了前面动作中提到的文件，则标记为依赖
func dependentIndices(actions []string, currentIdx int) []int

func decomposeTask(msgs []provider.ChatRequest, intent *IntentResult, projectPath string) []SubTaskSpec {
    lastMsg := getLastUserMessage(msgs)
    
    // 策略 1: 基于文件引用拆分
    fileRefs := extractFileReferences(lastMsg)
    if len(fileRefs) >= 2 {
        var subTasks []SubTaskSpec
        for _, f := range fileRefs {
            subTasks = append(subTasks, SubTaskSpec{
                Description: fmt.Sprintf("处理文件 %s 的相关修改", f),
                Files:       []string{f},
            })
        }
        return subTasks
    }
    
    // 策略 2: 基于动作连接词拆分
    actionSegments := splitByActions(lastMsg)
    if len(actionSegments) >= 2 {
        var subTasks []SubTaskSpec
        for i, seg := range actionSegments {
            subTasks = append(subTasks, SubTaskSpec{
                Description: seg,
                DependsOn:   dependentIndices(actionSegments, i),
            })
        }
        return subTasks
    }
    
    // 策略 3: 无法拆分，返回 nil（走普通 Agent 循环）
    return nil
}
```

**集成点**:

在 `service.go` 的 `ChatStream` 中：

```go
// 意图分类
intent := s.intentClassifier.Classify(lastUserMessage)

// 任务路由
route := s.taskRouter.Route(req.Messages, intent)

switch route.Route {
case "direct":
    // 简单任务：直接走 Agent 循环
    go s.runAgentLoop(req, agentCtx)
    
case "agent":
    // 中等任务：自动选择 Agent 后走 Agent 循环
    if suggestedAgent := s.agentReg.FindByIntent(intent.Intent); suggestedAgent != "" {
        req.AgentID = suggestedAgent
    }
    go s.runAgentLoop(req, agentCtx)
    
case "decompose":
    // 复杂任务：拆分为子任务
    s.emitFn("ai:task:decomposed", map[string]any{
        "subtasks": route.SubTasks,
    })
    go s.runDecomposedTask(req, route, agentCtx)
}
```

**渐进式拆分（兜底）**:

在 `runAgentLoop` 中，当检测到任务过大时建议拆分：

```go
// 在循环中检测
if loop == 10 && s.progress.fileCount() > 8 {
    s.emitFn("ai:task:complexity_warning", map[string]any{
        "message": "任务涉及大量文件修改，建议拆分为多个子任务以提高效率",
        "filesModified": s.progress.filesModified,
    })
}
```

**测试**:
- `TestTaskComplexity_Simple`: 简单问答 → ComplexitySimple
- `TestTaskComplexity_Moderate`: 单文件修改 → ComplexityModerate
- `TestTaskComplexity_Complex`: 多文件重构 → ComplexityComplex
- `TestTaskDecomposition_ByFiles`: 基于文件引用拆分
- `TestTaskDecomposition_ByActions`: 基于动作拆分

---

## 实现依赖关系

```
第一批（基础设施）:
  #1 上下文去重 → 独立
  #2 智能截断 → 独立
  #4 文件指纹缓存 → 独立
  #6 意图分类器 → 独立（#5 #8 #9 依赖它）

第二批（Agent 智能化）:
  #3 错误恢复 → 依赖 #2（截断影响错误信息展示）
  #5 工具选择引擎 → 依赖 #6（意图分类）
  #7 上下文推荐 → 依赖 #1（去重后推荐）
  #8 Agent 标签 → 依赖 #6（意图分类）

第三批（编排层）:
  #9 意图路由 → 依赖 #6 #8（意图分类 + Agent 标签）
```

---

## 风险与缓解

| 风险 | 缓解措施 |
|------|---------|
| 意图分类器误判 | 置信度阈值 + 用户手动覆盖 |
| 自动上下文推荐增加延迟 | 性能约束 < 100ms + 异步预计算 |
| 任务拆分质量差 | 渐进式拆分兜底 + 用户确认 |
| 工具路由规则过于激进 | 仅作为提示增强，不强制路由 |
| 文件指纹缓存内存增长 | LRU 500 条上限 + 定期清理 |

---

## 测试策略

每个改进包含：
1. **单元测试**: 核心算法和边界条件
2. **集成测试**: 与现有系统的交互
3. **性能测试**: 延迟和内存开销验证

运行命令：
```bash
go test ./internal/context/... -v   # #1 #7
go test ./internal/ai/... -v        # #2 #3 #9
go test ./internal/agent/... -v     # #4 #5 #6 #8
```
