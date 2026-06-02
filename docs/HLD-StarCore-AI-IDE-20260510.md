# StarCore AI IDE — 高层架构设计 (HLD)

## 1. 文档信息

| 项目 | 内容 |
|------|------|
| 版本 | v1.0 |
| 日期 | 2026-05-10 |
| 状态 | Draft |

---

## 2. 背景与目标

基于 PRD v2.0 的功能需求，设计 StarCore AI IDE 的整体架构，确保：
- AI Provider 可独立开发、注册、替换
- Agent 系统可扩展，支持内置 + 用户自定义
- Skills 可复用，可被多个 Agent 共享
- Memory 层跨会话持久化
- 前后端职责清晰，API Key 安全

---

## 3. 系统架构总览

```
┌─────────────────────────────────────────────────────────────────┐
│                        StarCore IDE                             │
├──────────────────────────────┬──────────────────────────────────┤
│     Go Backend (Wails)       │      Svelte 5 Frontend          │
│                              │                                  │
│  ┌────────────────────┐     │  ┌────────────────────────────┐  │
│  │   Provider Layer   │     │  │    UI Component Layer      │  │
│  │  ┌──┐┌──┐┌──┐┌──┐ │     │  │  ActivityBar  EditorGroup  │  │
│  │  │OA││AN││DS││Ol│ │     │  │  Sidebar      AIPanel      │  │
│  │  └──┘└──┘└──┘└──┘ │     │  │  BottomPanel  StatusBar    │  │
│  └────────┬───────────┘     │  │  CommandPalette Settings    │  │
│           │                  │  └────────────────────────────┘  │
│  ┌────────┴───────────┐     │                                  │
│  │   Provider Manager  │     │  ┌────────────────────────────┐  │
│  │  (注册/路由/代理)    │◄──IPC──►│    Store Layer             │  │
│  └────────────────────┘     │  │  appStore  aiStore          │  │
│                              │  │  providerStore agentStore   │  │
│  ┌────────────────────┐     │  │  skillStore   memoryStore   │  │
│  │   Agent Runtime    │     │  └────────────────────────────┘  │
│  │  Agent Registry    │     │                                  │
│  │  Prompt Builder    │     │  ┌────────────────────────────┐  │
│  │  Tool Executor     │     │  │    Service Layer           │  │
│  └────────────────────┘     │  │  ProviderService (代理)    │  │
│                              │  │  AgentService   SkillService │  │
│  ┌────────────────────┐     │  │  MemoryService  ContextSvc │  │
│  │   Skill Engine     │     │  └────────────────────────────┘  │
│  │  Skill Registry    │     │                                  │
│  │  Skill Executor    │     │                                  │
│  └────────────────────┘     │                                  │
│                              │                                  │
│  ┌────────────────────┐     │                                  │
│  │   Memory Layer     │     │                                  │
│  │  Conversation Store│     │                                  │
│  │  Knowledge Base    │     │                                  │
│  │  Preference Store  │     │                                  │
│  │  Context Collector │     │                                  │
│  └────────────────────┘     │                                  │
│                              │                                  │
│  ┌────────────────────┐     │                                  │
│  │   Core Services    │     │                                  │
│  │  FileSystem  PTY   │     │                                  │
│  │  Search     Git    │     │                                  │
│  │  LSP        Config │     │                                  │
│  └────────────────────┘     │                                  │
└──────────────────────────────┴──────────────────────────────────┘
```

---

## 4. 模块分解

### 4.1 Provider Layer — AI 提供商层

**设计原则：** 每个 Provider 一个独立 Go 文件，实现统一接口，Provider Manager 负责注册和路由。

```
internal/provider/
├── provider.go          # Provider 接口定义
├── manager.go           # Provider Manager（注册/路由/代理）
├── openai.go            # OpenAI Provider 实现
├── anthropic.go         # Anthropic Provider 实现
├── deepseek.go          # DeepSeek Provider 实现
├── ollama.go            # Ollama Provider 实现
├── azure.go             # Azure OpenAI Provider 实现
└── custom.go            # 自定义 OpenAI 兼容 Provider
```

**核心接口：**

| 方法 | 签名 | 说明 |
|------|------|------|
| Chat | (ctx, req ChatRequest) → (ChatResponse, error) | 非流式对话 |
| ChatStream | (ctx, req ChatRequest) → (Stream, error) | 流式对话，返回事件流 |
| Completion | (ctx, req CompletionRequest) → (CompletionResponse, error) | 代码补全 |
| ListModels | (ctx) → ([]Model, error) | 列出可用模型 |
| Validate | (ctx) → (bool, error) | 验证连接和 API Key |

**Provider Manager 职责：**
- 启动时自动注册所有内置 Provider
- 根据 provider_id 路由请求到对应 Provider
- 管理 Provider 配置（加密存储 API Key）
- 提供统一的事件流接口给前端

### 4.2 Agent Runtime — Agent 运行时

```
internal/agent/
├── agent.go             # Agent 接口定义
├── registry.go          # Agent Registry（注册/查询）
├── runtime.go           # Agent Runtime（对话管理/Tool 调用）
├── prompt_builder.go    # System Prompt 构建（模板 + 上下文注入）
├── tool_executor.go     # Tool 执行器（读写文件/搜索/执行命令）
├── tools/
│   ├── read_file.go
│   ├── write_file.go
│   ├── search_files.go
│   ├── execute_command.go
│   └── list_directory.go
└── builtins/
    ├── universal_assistant.go
    ├── frontend_architect.go
    ├── backend_architect.go
    ├── product_manager.go
    ├── ui_designer.go
    ├── devops_engineer.go
    ├── performance_expert.go
    ├── api_test_engineer.go
    ├── compliance_checker.go
    └── ai_integration_engineer.go
```

**Agent 接口：**

| 字段 | 类型 | 说明 |
|------|------|------|
| ID | string | 唯一标识 |
| Name | string | 显示名称 |
| Icon | string | 图标标识 |
| Description | string | 简短描述 |
| SystemPrompt | string | 系统提示词模板 |
| DefaultModel | string | 默认模型 |
| Tools | []Tool | 可用工具列表 |
| Skills | []string | 关联 Skill ID |
| Config | AgentConfig | 配置参数 |

**Tool 系统设计：**
- Agent 可声明可使用的 Tools（文件读取/写入/搜索/命令执行等）
- Tool 执行需要用户确认（可配置自动批准）
- Tool 调用结果作为 AI 消息的一部分流式返回前端

### 4.3 Skill Engine — 技能引擎

```
internal/skill/
├── skill.go             # Skill 接口定义
├── registry.go          # Skill Registry
├── executor.go          # Skill Executor（组装 Prompt → 调用 AI → 返回结果）
└── builtins/
    ├── generate_test.go
    ├── code_review.go
    ├── refactor.go
    ├── generate_doc.go
    ├── explain_code.go
    ├── fix_bug.go
    ├── commit_message.go
    └── sql_optimize.go
```

**Skill 执行流程：**
1. 用户触发 Skill（命令面板/右键/AI 面板 /skill）
2. Skill 收集上下文（选中代码/当前文件/错误信息）
3. Skill 组装 Prompt（Skill 模板 + 上下文）
4. 调用当前 Agent 的 Provider 发送请求
5. 流式返回结果
6. 结果支持 Apply/Copy/Insert/Discard 操作

### 4.4 Memory Layer — 记忆层

```
internal/memory/
├── conversation_store.go  # 对话历史存储（SQLite）
├── knowledge_base.go      # 项目知识库
├── preference_store.go    # 用户偏好存储
├── context_collector.go   # 上下文自动收集
└── summarizer.go          # 对话摘要压缩
```

**存储方案：** SQLite（嵌入式，零配置）

| 表 | 说明 |
|----|------|
| conversations | 对话列表（id/agent/model/created_at/title） |
| messages | 消息列表（id/conversation_id/role/content/tokens/created_at） |
| knowledge | 知识条目（id/project_path/key/value/source/updated_at） |
| preferences | 用户偏好（key/value/scope） |
| token_usage | Token 用量（id/conversation_id/provider/model/tokens_in/tokens_out/cost/created_at） |

**上下文收集器：**
- 当前文件内容 + 光标位置
- 选中文本
- 诊断错误列表
- 项目结构摘要
- Git diff（可选）
- 用户手动 @file @folder 附加

### 4.5 MCP Layer — Model Context Protocol

```
internal/mcp/
├── client.go             # MCP Client（连接 Server）
├── server_manager.go     # Server 生命周期管理
├── transport/
│   ├── stdio.go          # stdio 传输
│   └── sse.go            # SSE 传输
├── protocol.go           # MCP 协议消息定义
└── builtins/
    ├── filesystem.go     # 内置 FileSystem Server
    ├── git.go            # 内置 Git Server
    └── terminal.go       # 内置 Terminal Server
```

**MCP 集成方式：**
- MCP Client 在 Go 后端运行，通过 stdio/SSE 连接 MCP Server
- MCP Server 暴露的 Tools 自动注册到 Tool Executor
- MCP Server 暴露的 Resources 通过 @resource 引用为 AI 上下文
- MCP Server 配置存储在 `mcp_servers.json`：

```json
{
  "servers": {
    "fetch": {
      "command": "npx",
      "args": ["-y", "@modelcontextprotocol/server-fetch"],
      "transport": "stdio"
    },
    "brave-search": {
      "endpoint": "http://localhost:3000/sse",
      "transport": "sse",
      "env": { "BRAVE_API_KEY": "***" }
    }
  }
}
```

### 4.6 Workflow Engine — Agent 工作流引擎

```
internal/workflow/
├── workflow.go           # Workflow 定义
├── engine.go             # 执行引擎
├── step.go               # 步骤定义
└── builtins/
    ├── full_implementation.go  # 完整实现流程
    ├── bug_fix.go              # Bug 修复流程
    └── code_review.go          # 代码审查流程
```

**Workflow 数据结构：**

| 字段 | 类型 | 说明 |
|------|------|------|
| ID | string | 工作流 ID |
| Name | string | 显示名称 |
| Steps | []Step | 步骤列表 |
| Step.AgentID | string | 该步骤使用的 Agent |
| Step.PromptTemplate | string | Prompt 模板 |
| Step.InputFrom | string | 前一步骤 ID（可选） |
| Step.AutoApprove | bool | 是否自动继续 |

**执行流程：**
1. Engine 按顺序执行 Steps
2. 每步调用 AgentRuntime.ChatStream，前步输出注入当前步上下文
3. 每步完成后推送 `workflow:step:done` 事件到前端
4. 若 Step.AutoApprove=false，等待用户确认后继续
5. 支持暂停/继续/取消

### 4.7 Extension System — 扩展系统

```
internal/extension/
├── host.go              # Extension Host（管理生命周期）
├── api.go               # Extension API 定义
├── registry.go          # 扩展注册表
└── sandbox.go           # 沙箱隔离
```

**Extension Host 设计：**
- 使用 Go 的 JS 引擎（goja）或嵌入 Node.js 运行 Extension 代码
- Extension API 暴露：registerCommand/registerView/registerStatusBarItem/onDidChangeActiveEditor
- Extension 通过 API 注册的功能统一由 Extension Host 调度
- 沙箱化：Extension 仅能访问声明的权限（文件/网络/终端）

---

## 5. 核心技术选型

| 层 | 技术 | 理由 |
|----|------|------|
| AI 请求代理 | Go net/http + streaming | 后端代理，API Key 不暴露到前端 |
| 流式传输 | Wails Events（SSE 模拟） | 已有 ai:stream:data/done/error 事件模式 |
| 数据持久化 | SQLite（go-sqlite3） | 嵌入式，对话历史/知识库/偏好存储 |
| 配置存储 | JSON 文件 + 加密 Keychain | Provider 配置 JSON，API Key 系统密钥环加密 |
| 前端状态 | Svelte 5 Runes + Stores | 响应式状态管理 |
| UI 组件 | shadcn-svelte + Tailwind 4 | 一致设计语言 |
| 终端 | Go PTY（creack/pty）+ Xterm.js | 真实 Shell 连接 |
| 搜索 | ripgrep（Go 绑定） | 高性能文件搜索 |
| Diff 计算 | go-diff | Apply Diff 功能 |
| 加密 | gobwas/wordwrap + OS Keychain | API Key 安全存储 |
| MCP | go-mcp 或自实现 | Model Context Protocol 客户端 |
| 调试 | DAP（delve/spawner） | Debug Adapter Protocol |
| JS 引擎 | goja（纯 Go JS 引擎） | Extension Host 运行扩展代码 |
| 图片处理 | image（Go 标准库）+ base64 | 多模态图片编码 |

---

## 6. 数据流 / 调用链

### 6.1 AI 对话流式响应

```
用户输入 → Svelte AIChat 组件
  → Wails IPC: AIChatStream(provider, model, messages, agent, context)
    → Go Provider Manager: 路由到对应 Provider
      → Provider.ChatStream(): 发起 HTTP SSE 请求
        → 逐 chunk 通过 Wails Events 推送:
          - ai:stream:data → 前端追加渲染
          - ai:stream:tool_call → 前端显示工具调用
          - ai:stream:thinking → 前端显示思考过程
          - ai:stream:done → 前端完成
          - ai:stream:error → 前端错误提示
```

### 6.2 Skill 执行

```
用户触发 Skill → Svelte SkillService
  → 收集上下文（选中代码/文件/错误）
  → Wails IPC: ExecuteSkill(skillId, context, agentId)
    → Go Skill Executor:
      → Skill 构建 Prompt
      → Agent Runtime 调用 Provider
      → 流式返回 → 前端渲染
```

### 6.3 内联补全

```
编辑器光标变化 → Svelte debounce (300ms)
  → Wails IPC: GetCompletion(file, content, cursor)
    → Go Provider Manager → Provider.Completion()
      → 返回补全文本 → 前端 CodeMirror 装饰层渲染灰度文本
```

### 6.4 Apply Diff

```
AI 返回代码修改 → 用户点击 Apply
  → Svelte DiffViewer 组件展示 Diff
  → 用户逐块 Accept/Reject
  → Wails IPC: ApplyDiff(filePath, hunks[])
    → Go 后端应用修改到文件
    → 前端刷新编辑器内容
```

---

## 7. 关键接口定义（高层）

### 7.1 Provider Interface（Go）

| 接口 | 方法 | 说明 |
|------|------|------|
| Provider | Chat(ctx, ChatRequest) → (ChatResponse, error) | 非流式 |
| Provider | ChatStream(ctx, ChatRequest) → (<-chan StreamEvent, error) | 流式 |
| Provider | Completion(ctx, CompletionRequest) → (CompletionResponse, error) | 补全 |
| Provider | ListModels(ctx) → ([]Model, error) | 模型列表 |
| Provider | Validate(ctx) → (bool, error) | 连接验证 |

### 7.2 Wails IPC 接口（前后端桥接）

| 方法 | 说明 |
|------|------|
| AIChatStream(req AIChatRequest) → error | 流式对话（事件推送） |
| AICompletion(req CompletionRequest) → (CompletionResponse, error) | 代码补全 |
| GetProviders() → ([]ProviderInfo, error) | 获取所有 Provider |
| AddProvider(config ProviderConfig) → error | 添加 Provider |
| RemoveProvider(id string) → error | 删除 Provider |
| TestProvider(id string) → (bool, error) | 测试 Provider 连接 |
| GetAgents() → ([]AgentInfo, error) | 获取所有 Agent |
| GetAgentConfig(id string) → (AgentConfig, error) | 获取 Agent 配置 |
| ExecuteSkill(skillId, context) → error | 执行 Skill（事件推送） |
| GetSkills() → ([]SkillInfo, error) | 获取所有 Skill |
| GetConversations(projectPath) → ([]Conversation, error) | 获取对话历史 |
| SaveConversation(conv Conversation) → error | 保存对话 |
| GetKnowledge(projectPath) → ([]Knowledge, error) | 获取项目知识 |
| SaveKnowledge(entry Knowledge) → error | 保存知识条目 |
| GetMCPServers() → ([]MCPServerConfig, error) | 获取 MCP Server 列表 |
| AddMCPServer(config MCPServerConfig) → error | 添加 MCP Server |
| RemoveMCPServer(id string) → error | 删除 MCP Server |
| ExecuteWorkflow(workflowID, context) → error | 执行 Workflow（事件推送） |
| GetWorkflows() → ([]WorkflowInfo, error) | 获取 Workflow 列表 |
| GetTokenUsage(projectPath, period) → (TokenUsageStats, error) | 获取 Token 用量 |
| ApplyDiff(filePath string, hunks []Hunk) → error | 应用 Diff |

---

## 8. 非功能设计

### 8.1 安全

- API Key 仅在 Go 后端使用，绝不通过 IPC 传到前端
- Provider 配置中 API Key 使用 OS Keychain（macOS Keychain / Windows Credential Manager）加密存储
- Agent System Prompt 不受用户消息注入影响
- Tool 执行需用户确认（可配置自动批准白名单）

### 8.2 性能

- AI 流式响应通过 Wails Events 直接推送，无轮询
- 补全请求 debounce 300ms，避免频繁请求
- 上下文收集异步执行，不阻塞编辑器
- 对话历史懒加载，滚动到顶部时加载更多
- SQLite WAL 模式，读写不互相阻塞

### 8.3 可扩展性

- 新 Provider：实现 Provider 接口 + 在 init() 中注册
- 新 Agent：实现 Agent 定义 + 在 builtins 中注册
- 新 Skill：实现 Skill 接口 + 在 builtins 中注册
- 新 Tool：实现 Tool 接口 + Agent 声明引用

### 8.4 部署架构

```
单个 Wails 二进制文件
├── Go 后端（编译链接）
├── Svelte 前端（嵌入资源）
├── SQLite 数据库（用户数据目录自动创建）
└── 配置文件（用户数据目录）
```

---

## 9. 风险与待决

| 风险 | 影响 | 缓解 |
|------|------|------|
| Provider API 频繁变更 | 请求失败 | 版本化 API 路径，接口抽象足够 |
| 流式 SSE 兼容性 | 部分 Provider 流式格式不同 | 每个 Provider 自行解析 SSE |
| SQLite 并发写入 | 数据损坏 | WAL 模式 + 单写协程 |
| Agent Prompt 注入 | AI 行为异常 | System Prompt 与用户消息严格隔离 |
| 内联补全延迟 | 体验差 | 本地缓存 + debounce + 取消机制 |
| Wails v2 事件系统限制 | 高频流式丢帧 | 批量合并推送 + 前端 requestAnimationFrame |
| MCP 协议复杂性 | 集成难度 | 先实现 stdio 传输，再扩展 SSE |