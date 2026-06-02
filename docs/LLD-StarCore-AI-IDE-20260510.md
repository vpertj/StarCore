# StarCore AI IDE — 详细设计文档 (LLD)

## 1. 文档信息

| 项目 | 内容 |
|------|------|
| 版本 | v1.0 |
| 日期 | 2026-05-10 |
| 状态 | Draft |

---

## 2. 设计目标

将 HLD 中的架构细化为可编码的接口、数据结构、组件树和流程，确保开发团队可直接依据本文档实现。

---

## 3. 数据模型

### 3.1 Provider 数据模型

**Go 结构体：**

| 结构体 | 字段 | 类型 | 说明 |
|--------|------|------|------|
| ProviderConfig | ID | string | 唯一标识：openai/anthropic/deepseek/ollama/azure/custom_{n} |
| | Name | string | 显示名称 |
| | APIKey | string | 加密后的 API Key（内存中为明文，存储为密文） |
| | Endpoint | string | API 端点 URL |
| | ExtraConfig | map[string]any | 额外参数（如 Azure 的 DeploymentName） |
| | IsDefault | bool | 是否默认 |
| | Enabled | bool | 是否启用 |
| | CreatedAt | time.Time | 创建时间 |
| Model | ID | string | 模型 ID（如 gpt-4o） |
| | Name | string | 显示名称（如 GPT-4o） |
| | ProviderID | string | 所属 Provider |
| | MaxTokens | int | 最大 Token |
| | SupportsVision | bool | 是否支持图片 |
| | SupportsTool | bool | 是否支持 Tool Calling |
| | SupportsThinking | bool | 是否支持思考过程 |

**前端 TS 类型：**

| 类型 | 字段 | 说明 |
|------|------|------|
| ProviderInfo | id, name, endpoint, enabled, isDefault, models[] | 前端可见信息（无 API Key） |
| ModelOption | id, name, providerId, maxTokens | 模型选择器用 |

### 3.2 Agent 数据模型

**Go 结构体：**

| 结构体 | 字段 | 类型 | 说明 |
|--------|------|------|------|
| AgentDef | ID | string | 如 frontend-architect |
| | Name | string | 如 前端架构师 |
| | Icon | string | Lucide 图标名 |
| | Description | string | 一句话描述 |
| | SystemPrompt | string | 系统 Prompt 模板 |
| | DefaultModel | string | 默认模型 ID |
| | Tools | []string | 可用 Tool ID 列表 |
| | Skills | []string | 关联 Skill ID 列表 |
| | Category | string | 分类：dev/design/ops/qa |
| | Config | AgentConfig | 可覆盖配置 |

**AgentConfig：**

| 字段 | 类型 | 默认值 | 说明 |
|------|------|--------|------|
| Temperature | float64 | 0.7 | 生成温度 |
| MaxTokens | int | 4096 | 最大输出 Token |
| AutoApproveTools | bool | false | 自动批准 Tool 执行 |
| CustomPromptAppend | string | "" | 用户追加的 System Prompt |

### 3.3 Skill 数据模型

| 结构体 | 字段 | 类型 | 说明 |
|--------|------|------|------|
| SkillDef | ID | string | 如 generate-test |
| | Name | string | 如 生成单元测试 |
| | Icon | string | Lucide 图标名 |
| | Description | string | 描述 |
| | Trigger | string | 触发方式：selection/file/error/manual |
| | PromptTemplate | string | Prompt 模板（可含 {code} {file} {error} 占位符） |
| | ResultType | string | 结果类型：diff/text/code |
| | AssociatedAgents | []string | 推荐使用的 Agent |

### 3.4 Memory 数据模型

**SQLite 表结构：**

表 `conversations`：
| 列 | 类型 | 说明 |
|----|------|------|
| id | TEXT PK | UUID |
| project_path | TEXT | 项目路径 |
| agent_id | TEXT | Agent ID |
| model | TEXT | 模型 ID |
| provider_id | TEXT | Provider ID |
| title | TEXT | 对话标题（自动生成或用户编辑） |
| summary | TEXT | 摘要（压缩后） |
| created_at | DATETIME | 创建时间 |
| updated_at | DATETIME | 更新时间 |
| message_count | INT | 消息数 |

表 `messages`：
| 列 | 类型 | 说明 |
|----|------|------|
| id | TEXT PK | UUID |
| conversation_id | TEXT FK | 对话 ID |
| seq | INT | 序号 |
| role | TEXT | user/assistant/tool/system |
| content | TEXT | 消息内容（Markdown） |
| tool_calls | TEXT | JSON: Tool 调用列表 |
| tool_results | TEXT | JSON: Tool 结果列表 |
| thinking | TEXT | 思考过程 |
| tokens_in | INT | 输入 Token |
| tokens_out | INT | 输出 Token |
| created_at | DATETIME | |

表 `knowledge`：
| 列 | 类型 | 说明 |
|----|------|------|
| id | TEXT PK | UUID |
| project_path | TEXT | 项目路径 |
| category | TEXT | 分类：techstack/convention/decision/structure |
| key | TEXT | 键 |
| value | TEXT | 值 |
| source | TEXT | 来源：user/auto/agent |
| updated_at | DATETIME | |

表 `preferences`：
| 列 | 类型 | 说明 |
|----|------|------|
| key | TEXT PK | 键 |
| value | TEXT | 值 |
| scope | TEXT | 范围：global/project |

### 3.5 MCP 数据模型

**MCPServerConfig：**

| 字段 | 类型 | 说明 |
|------|------|------|
| ID | string | 唯一标识 |
| Name | string | 显示名称 |
| Command | string | 启动命令（stdio 传输） |
| Args | []string | 命令参数 |
| Endpoint | string | SSE 端点（SSE 传输） |
| Transport | string | stdio / sse |
| Env | map[string]string | 环境变量 |
| Enabled | bool | 是否启用 |

### 3.6 Workflow 数据模型

| 字段 | 类型 | 说明 |
|------|------|------|
| WorkflowDef.ID | string | 工作流 ID |
| WorkflowDef.Name | string | 显示名称 |
| WorkflowDef.Description | string | 描述 |
| WorkflowDef.Steps | []StepDef | 步骤列表 |
| StepDef.ID | string | 步骤 ID |
| StepDef.Name | string | 步骤名称 |
| StepDef.AgentID | string | 使用的 Agent |
| StepDef.PromptTemplate | string | Prompt 模板 |
| StepDef.InputFromStep | string | 从哪一步获取输入 |
| StepDef.AutoApprove | bool | 是否自动继续 |

### 3.7 Token 用量数据模型

表 `token_usage`：
| 列 | 类型 | 说明 |
|----|------|------|
| id | TEXT PK | UUID |
| conversation_id | TEXT FK | 对话 ID |
| provider_id | TEXT | Provider ID |
| model | TEXT | 模型 ID |
| tokens_in | INT | 输入 Token |
| tokens_out | INT | 输出 Token |
| cost | REAL | 估算费用 |
| created_at | DATETIME | |

表 `model_pricing`：
| 列 | 类型 | 说明 |
|----|------|------|
| model_id | TEXT PK | 模型 ID |
| price_input | REAL | 每百万 Token 输入价格 |
| price_output | REAL | 每百万 Token 输出价格 |
| currency | TEXT | 货币单位 |

### 3.8 Chat Request / Response 数据模型

**AIChatRequest（已有，扩展）：**

| 字段 | 类型 | 说明 |
|------|------|------|
| ProviderID | string | Provider 标识 |
| Model | string | 模型 ID |
| Messages | []Message | 消息列表 |
| AgentID | string | Agent ID |
| Skills | []string | 触发的 Skill ID |
| ContextFiles | []string | 附加的文件路径 |
| ContextCode | string | 选中的代码片段 |
| Temperature | float64 | |
| MaxTokens | int | |
| Stream | bool | |

**StreamEvent（Wails 事件）：**

| 类型 | 字段 | 说明 |
|------|------|------|
| StreamDelta | Content string | 文本增量 |
| StreamToolCall | ToolName, Args, Result | Tool 调用 |
| StreamThinking | Content string | 思考过程 |
| StreamDone | TokensIn, TokensOut int | 完成 |
| StreamError | Message string | 错误 |

---

## 4. 核心类/接口定义

### 4.1 Provider Interface（Go）

```go
type Provider interface {
    ID() string
    Name() string
    Chat(ctx context.Context, req ChatRequest) (*ChatResponse, error)
    ChatStream(ctx context.Context, req ChatRequest) (<-chan StreamEvent, error)
    Completion(ctx context.Context, req CompletionRequest) (*CompletionResponse, error)
    ListModels(ctx context.Context) ([]Model, error)
    Validate(ctx context.Context) error
}
```

**各 Provider 实现要点：**

| Provider | 文件 | 特殊处理 |
|----------|------|----------|
| OpenAI | openai.go | SSE 解析 data: 前缀，支持 tool_calls |
| Anthropic | anthropic.go | 消息格式不同（content 为数组），支持 thinking |
| DeepSeek | deepseek.go | OpenAI 兼容格式，前缀填充优化 |
| Ollama | ollama.go | 本地 HTTP，/api/tags 获取模型列表 |
| Azure | azure.go | URL 路径含 deployment name，认证用 api-key header |
| Custom | custom.go | 用户配置端点，OpenAI 兼容格式 |

### 4.2 Provider Manager

```go
type ProviderManager struct {
    providers map[string]Provider
    configs   map[string]ProviderConfig
    keyring   KeyringStore
    mu        sync.RWMutex
}

func (m *ProviderManager) Register(p Provider)
func (m *ProviderManager) ChatStream(ctx, providerID, req) (<-chan StreamEvent, error)
func (m *ProviderManager) Completion(ctx, providerID, req) (*CompletionResponse, error)
func (m *ProviderManager) GetProviders() []ProviderInfo
func (m *ProviderManager) GetModels(providerID) ([]Model, error)
func (m *ProviderManager) SaveConfig(config ProviderConfig) error
func (m *ProviderManager) DeleteProvider(id string) error
func (m *ProviderManager) TestConnection(id string) error
```

### 4.3 Agent Runtime

```go
type AgentRuntime struct {
    registry    *AgentRegistry
    providerMgr *ProviderManager
    toolExec    *ToolExecutor
    promptBld   *PromptBuilder
    memory      *MemoryLayer
}

func (r *AgentRuntime) ChatStream(ctx, agentID, req) (<-chan StreamEvent, error)
// 内部流程:
// 1. 从 Registry 获取 AgentDef
// 2. PromptBuilder 组装 System Prompt
// 3. 注入 Memory 上下文（知识库 + 偏好）
// 4. 调用 ProviderManager.ChatStream
// 5. 如果 AI 返回 tool_call → ToolExecutor 执行 → 结果注入消息 → 继续调用
// 6. 流式推送给前端
```

### 4.4 Tool Executor

```go
type Tool interface {
    ID() string
    Name() string
    Description() string
    Parameters() ToolParameters  // JSON Schema
    Execute(ctx context.Context, args map[string]any) (string, error)
    RequiresApproval() bool
}
```

内置 Tools：

| Tool | 参数 | 返回 | 需审批 |
|------|------|------|--------|
| read_file | path, start_line?, end_line? | 文件内容 | 否 |
| write_file | path, content | 成功/失败 | 是 |
| search_files | query, include_pattern? | 搜索结果 | 否 |
| list_directory | path | 文件列表 | 否 |
| execute_command | command, cwd? | 命令输出 | 是 |
| get_diagnostics | file_path? | 诊断列表 | 否 |
| get_git_diff | file_path? | diff 内容 | 否 |

### 4.5 Skill Executor

```go
type SkillExecutor struct {
    registry   *SkillRegistry
    providerMgr *ProviderManager
    memory     *MemoryLayer
}

func (e *SkillExecutor) Execute(ctx, skillID, context SkillContext) (<-chan StreamEvent, error)
// 内部流程:
// 1. 从 Registry 获取 SkillDef
// 2. 用 context 填充 PromptTemplate
// 3. 构造 ChatRequest（system = skill prompt, user = context）
// 4. 调用 ProviderManager.ChatStream
// 5. 流式返回
```

---

## 5. 前端组件树

```
App.svelte
├── TitleBar.svelte
│   ├── MenuButton
│   ├── Breadcrumb (文件路径)
│   └── WindowControls (最小化/最大化/关闭)
├── MainLayout.svelte (flex 水平)
│   ├── ActivityBar.svelte (48px 固定)
│   │   ├── ActivityIcon (Explorer)
│   │   ├── ActivityIcon (Search)
│   │   ├── ActivityIcon (Git)
│   │   ├── ActivityIcon (AI) ← 点击切换右侧 AI Panel
│   │   └── ActivityIcon (Extensions)
│   ├── Sidebar.svelte (200~400px 可拖拽)
│   │   ├── ProjectExplorer (文件树)
│   │   ├── SearchPanel (搜索/替换)
│   │   ├── GitPanel (变更/提交)
│   │   └── ExtensionsPanel (扩展)
│   ├── EditorGroup.svelte (flex: 1)
│   │   ├── TabBar
│   │   ├── EditorContainer (flex 垂直)
│   │   │   ├── CodeEditor (CodeMirror 6)
│   │   │   └── InlineCompletion (装饰层)
│   │   └── BottomPanel (可拖拽高度)
│   │       ├── TerminalTab (Xterm.js)
│   │       ├── OutputTab
│   │       └── ProblemsTab
│   └── AIPanel.svelte (300~600px 可拖拽，右侧)
│       ├── AIPanelHeader
│       │   ├── AgentSelector (下拉)
│       │   ├── ModelSelector (下拉)
│       │   ├── NewChatButton
│       │   ├── HistoryButton
│       │   └── CloseButton
│       ├── ChatMessages (滚动列表)
│       │   ├── UserMessage
│       │   │   ├── Avatar
│       │   │   ├── Content (Markdown)
│       │   │   └── ContextTags (附加文件/代码)
│       │   └── AssistantMessage
│       │       ├── AgentAvatar
│       │       ├── ThinkingBlock (可折叠)
│       │       ├── Content (Markdown + 代码块)
│       │       ├── CodeBlock (Copy/Apply/Run 按钮)
│       │       └── ToolCallBlock (调用过程+结果)
│       ├── ContextPreview (当前上下文预览)
│       └── ChatInput
│           ├── TextArea (多行, Shift+Enter 换行, Enter 发送)
│           ├── ContextButton (@ 触发)
│           ├── SkillButton (/ 触发)
│           ├── AttachButton (粘贴图片/拖拽文件)
│           └── SendButton
├── CommandPalette.svelte (浮层, Ctrl+Shift+P)
├── Settings.svelte (模态)
├── Notifications.svelte (Toast, 右下角)
├── WorkflowPanel.svelte (Workflow 执行状态)
│   ├── WorkflowStepList
│   └── WorkflowStepItem (Agent名/状态/输出预览)
├── MCPManager.svelte (MCP Server 管理, 设置中)
├── TokenUsagePanel.svelte (Token 用量统计, 设置中)
└── DebugToolbar.svelte (调试工具栏)
    ├── StartDebugButton
    ├── StepOverButton
    ├── StepIntoButton
    ├── StepOutButton
    ├── StopDebugButton
    ├── VariablesPanel
    └── CallStackPanel
```

### 5.1 Svelte Stores 设计

**新增 Store 文件：**

| Store | 文件 | 核心状态 |
|-------|------|----------|
| providerStore | stores/provider.ts | providers[], activeProviderId, models[], activeModelId |
| agentStore | stores/agent.ts | agents[], activeAgentId, agentConfigs{} |
| skillStore | stores/skill.ts | skills[], executingSkillId |
| memoryStore | stores/memory.ts | conversations[], activeConversationId, knowledge[], contextFiles[] |
| uiStore | stores/ui.ts | sidebarVisible, sidebarWidth, aiPanelVisible, aiPanelWidth, bottomPanelVisible, bottomPanelHeight, commandPaletteOpen |
| mcpStore | stores/mcp.ts | servers[], serverStatuses{}, tools[] |
| workflowStore | stores/workflow.ts | workflows[], activeWorkflow, steps[], stepStates{} |
| debugStore | stores/debug.ts | isDebugging, breakpoints[], variables[], callStack[], debugState |

---

## 6. 详细流程

### 6.1 AI 对话完整流程（含 Tool 调用）

```
[前端] 用户输入消息 + 上下文
  ↓
[前端] aiStore.sendMessage(content, contextFiles, contextCode)
  ↓
[IPC] Wails.Call("AIChatStream", req)
  ↓
[Go] AgentRuntime.ChatStream()
  ├── 获取 Agent → 构建 System Prompt
  ├── 注入 Memory（知识库 + 对话摘要）
  ├── ProviderManager.ChatStream() → 发起 HTTP SSE
  ↓
[Go] 逐 chunk 推送 StreamEvent
  ├── StreamDelta → EventsEmit("ai:stream:data", delta)
  ├── StreamToolCall → EventsEmit("ai:stream:tool_call", toolCall)
  │   ↓
  │   [前端] 显示 Tool 调用中
  │   [Go] ToolExecutor.Execute() → 获取结果
  │   [Go] 结果注入消息 → 继续调用 AI
  │   ↓
  ├── StreamThinking → EventsEmit("ai:stream:thinking", content)
  ├── StreamDone → EventsEmit("ai:stream:done", stats)
  └── StreamError → EventsEmit("ai:stream:error", msg)
  ↓
[前端] EventsOn 监听 → 渲染到 ChatMessages
[Go] Memory 保存消息到 SQLite
```

### 6.2 Apply Diff 流程

```
[前端] AI 消息包含代码修改
  ↓
[前端] DiffViewer 组件：
  ├── 解析 AI 返回的 diff/代码块
  ├── go-diff 计算原文件 vs 修改后
  ├── 逐 hunk 显示：绿色(新增)/红色(删除)
  ↓
[前端] 用户点击 Accept/Reject（逐块或全部）
  ↓
[IPC] Wails.Call("ApplyDiff", {filePath, acceptedHunks})
  ↓
[Go] 读取原文件 → 应用接受的 hunk → 写回文件
  ↓
[前端] 刷新编辑器内容，显示通知"已应用 N 处修改"
```

### 6.3 内联补全流程

```
[前端] CodeMirror onChange → debounce 300ms
  ↓
[前端] 补全请求：当前文件 + 光标位置 + 前后上下文
  ↓
[IPC] Wails.Call("GetCompletion", {file, content, cursorPos, language})
  ↓
[Go] ProviderManager.Completion()
  ↓
[Go] 返回补全文本
  ↓
[前端] CodeMirror 装饰层：
  ├── 在光标位置插入灰度文本
  ├── Tab → 接受全部，替换灰度文本为真实文本
  ├── Ctrl+→ → 接受下一个词
  ├── Esc → 拒绝，移除灰度文本
  └── 任意其他输入 → 拒绝
```

### 6.4 Skill 触发流程

```
[前端] 触发方式之一：
  ├── 右键菜单 → "AI: 生成单元测试"
  ├── 命令面板 → "Skill: Generate Test"
  └── AI 面板输入 → "/test"
  ↓
[前端] skillStore.execute(skillId, context)
  ├── context = { selectedCode, filePath, diagnostics }
  ↓
[IPC] Wails.Call("ExecuteSkill", {skillId, context, agentId})
  ↓
[Go] SkillExecutor.Execute()
  ├── 获取 SkillDef → 填充 PromptTemplate
  ├── ProviderManager.ChatStream()
  ├── 流式推送给前端
  ↓
[前端] 渲染结果 → 显示操作按钮（Apply/Copy/Insert/Discard）
```

### 6.5 Memory 上下文收集流程

```
[前端] 用户发送消息时：
  ├── 自动收集：
  │   ├── 当前活动文件路径 + 内容
  │   ├── 选中的代码片段
  │   ├── 编辑器诊断错误列表
  │   └── 项目根目录路径
  ├── 手动附加（@ 触发）：
  │   ├── @file → 文件路径 + 内容
  │   ├── @folder → 目录结构
  │   ├── @error → 最近错误
  │   └── @web → 网页内容（URL fetch）
  ├── @resource → MCP Resource 引用
  ↓
[IPC] 发送到后端，后端组装完整上下文
```

### 6.6 Workflow 执行流程

```
[前端] 用户选择 Workflow → workflowStore.execute(workflowId, context)
  ↓
[IPC] Wails.Call("ExecuteWorkflow", {workflowId, context})
  ↓
[Go] WorkflowEngine.Run():
  ├── 遍历 Steps
  ├── Step 1: AgentRuntime.ChatStream(agentID, prompt + context)
  │   ├── 流式推送到前端
  │   ├── EventsEmit("workflow:step:progress", {stepId, content})
  │   └── 完成后 EventsEmit("workflow:step:done", {stepId, output})
  ├── 若 AutoApprove=false → EventsEmit("workflow:step:await_approval", {stepId})
  │   └── [前端] 显示步骤结果，用户点击"继续"或"修改后继续"
  │   └── [IPC] Wails.Call("WorkflowContinue", {workflowId, stepId, userAction})
  ├── Step 2: 上下文 = Step 1 输出 → AgentRuntime.ChatStream(...)
  ├── ... 重复
  └── EventsEmit("workflow:done", {workflowId, summary})
```

### 6.7 MCP Tool 集成流程

```
[Go] 应用启动 → MCPServerManager.StartAll()
  ├── 读取 mcp_servers.json
  ├── 对每个已启用的 Server:
  │   ├── stdio 传输: exec.Command(cmd, args...) → stdin/stdout
  │   ├── SSE 传输: http.Client → GET endpoint/sse
  │   └── MCP Initialize 握手 → 获取 tools/resources/prompts
  ↓
[Go] MCP Tools 注册到 ToolExecutor
  ├── 每个 MCP tool 生成 Tool Adapter（适配 Tool 接口）
  ├── Tool.Execute() → MCP Client.CallTool(name, args) → 返回结果
  ↓
[Go] Agent 对话时，MCP Tools 与内置 Tools 统一可用
  └── AI 返回 tool_call → ToolExecutor 查找 → 若为 MCP Tool → MCP Client.CallTool
```

---

## 7. 错误处理与边界

| 场景 | 处理 |
|------|------|
| Provider API Key 无效 | Validate 失败 → 前端提示配置 API Key |
| Provider 端点不可达 | 超时 30s → StreamError → 前端显示重试按钮 |
| 流式中断 | 前端检测超时 10s 无数据 → 显示"响应中断" + 重试 |
| Token 超限 | 后端估算 Token → 超限时自动摘要压缩历史 |
| Tool 执行失败 | 返回错误信息给 AI，AI 决定是否重试 |
| 并发对话 | 每个 Agent 独立对话，互不干扰 |
| 大文件上下文 | 超过 10000 行的文件截断，保留关键部分 |
| 无网络 | Provider 调用失败 → 前端提示 + 切换到本地 Ollama 建议 |

---

## 8. 性能与资源估算

| 模块 | 内存 | 启动耗时 | 说明 |
|------|------|----------|------|
| Go 后端 | ~30MB | ~100ms | Wails 启动 + Provider 注册 |
| Svelte 前端 | ~50MB | ~200ms | WebView 初始化 + 组件渲染 |
| SQLite | ~5MB | ~10ms | 嵌入式数据库 |
| CodeMirror | ~10MB | ~50ms | 编辑器实例 |
| Xterm.js | ~5MB | ~20ms | 终端实例 |
| **总计** | **~100MB** | **~380ms** | 满足 < 150MB / < 800ms 目标 |

---

## 9. 测试要点

| 模块 | 测试类型 | 测试内容 |
|------|----------|----------|
| Provider | 单元测试 | 每个 Provider 的 Chat/ChatStream/Completion |
| Provider Manager | 单元测试 | 注册/路由/配置存储 |
| Agent Runtime | 集成测试 | 对话流程/Tool 调用/Prompt 构建 |
| Skill Executor | 单元测试 | Prompt 填充/执行/结果处理 |
| Memory | 单元测试 | CRUD/摘要压缩/上下文收集 |
| IPC | 集成测试 | 前后端数据序列化/流式事件 |
| UI | E2E 测试 | 面板布局/Agent 切换/对话交互 |
| 性能 | 基准测试 | 启动时间/内存/流式延迟 |

---

## 10. 实现优先级与文件清单

### Phase 1 实现文件清单

**Go 后端新增/修改：**

| 文件 | 说明 |
|------|------|
| internal/provider/provider.go | Provider 接口 |
| internal/provider/manager.go | Provider Manager |
| internal/provider/openai.go | OpenAI 实现（从 app.go 重构） |
| internal/provider/anthropic.go | Anthropic 实现 |
| internal/provider/deepseek.go | DeepSeek 实现 |
| internal/provider/ollama.go | Ollama 实现 |
| internal/agent/agent.go | Agent 接口 + 定义 |
| internal/agent/registry.go | Agent Registry |
| internal/agent/runtime.go | Agent Runtime |
| internal/agent/prompt_builder.go | Prompt 构建 |
| internal/agent/tool_executor.go | Tool 执行器 |
| internal/agent/tools/*.go | 各 Tool 实现 |
| internal/agent/builtins/*.go | 各内置 Agent |
| internal/skill/skill.go | Skill 接口 |
| internal/skill/registry.go | Skill Registry |
| internal/skill/executor.go | Skill 执行器 |
| internal/skill/builtins/*.go | 各内置 Skill |
| internal/memory/conversation_store.go | 对话存储 |
| internal/memory/knowledge_base.go | 知识库 |
| internal/memory/context_collector.go | 上下文收集 |
| internal/config/keyring.go | API Key 加密存储 |
| internal/mcp/client.go | MCP Client |
| internal/mcp/server_manager.go | MCP Server 管理 |
| internal/mcp/transport/stdio.go | MCP stdio 传输 |
| internal/mcp/transport/sse.go | MCP SSE 传输 |
| internal/mcp/protocol.go | MCP 协议消息 |
| internal/mcp/builtins/*.go | 内置 MCP Server |
| internal/workflow/workflow.go | Workflow 定义 |
| internal/workflow/engine.go | Workflow 执行引擎 |
| internal/workflow/builtins/*.go | 内置 Workflow |
| internal/extension/host.go | Extension Host |
| internal/extension/api.go | Extension API |
| internal/extension/registry.go | 扩展注册表 |
| internal/debug/adapter.go | DAP 调试适配器 |
| internal/debug/session.go | 调试会话 |
| internal/memory/token_usage.go | Token 用量追踪 |
| internal/update/checker.go | 版本更新检测 |
| app.go | 重构：移除 AI 直接实现，改调 Provider Manager |

**Svelte 前端新增/修改：**

| 文件 | 说明 |
|------|------|
| src/components/ActivityBar.svelte | Activity Bar |
| src/components/AIPanel.svelte | 右侧 AI 面板（重构自 AIChat） |
| src/components/AIPanelHeader.svelte | 面板头部 |
| src/components/AgentSelector.svelte | Agent 选择器 |
| src/components/ModelSelector.svelte | Model 选择器 |
| src/components/ChatMessages.svelte | 消息列表 |
| src/components/ChatInput.svelte | 输入区 |
| src/components/ContextPreview.svelte | 上下文预览 |
| src/components/DiffViewer.svelte | Diff 预览 |
| src/components/CommandPalette.svelte | 命令面板 |
| src/components/BottomPanel.svelte | 底部面板 |
| src/components/WelcomePage.svelte | 欢迎页 |
| src/components/Notifications.svelte | 通知 |
| src/components/InlineCompletion.svelte | 内联补全装饰 |
| src/stores/provider.ts | Provider 状态 |
| src/stores/agent.ts | Agent 状态 |
| src/stores/skill.ts | Skill 状态 |
| src/stores/memory.ts | Memory 状态 |
| src/stores/ui.ts | UI 状态 |
| src/services/providerService.ts | Provider 服务 |
| src/services/agentService.ts | Agent 服务 |
| src/services/skillService.ts | Skill 服务 |
| src/services/memoryService.ts | Memory 服务 |
| src/services/mcpService.ts | MCP 服务 |
| src/services/workflowService.ts | Workflow 服务 |
| src/services/debugService.ts | 调试服务 |
| src/components/WorkflowPanel.svelte | Workflow 面板 |
| src/components/MCPManager.svelte | MCP 管理 |
| src/components/TokenUsagePanel.svelte | Token 用量面板 |
| src/components/DebugToolbar.svelte | 调试工具栏 |
| App.svelte | 布局重构（三栏 → 四区域） |
