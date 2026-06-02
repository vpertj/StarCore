# StarCore AI IDE — 产品需求文档 (PRD)

## 1. 文档信息

| 项目 | 内容 |
|------|------|
| 版本 | v2.0 |
| 作者 | 产品总监 |
| 日期 | 2026-05-10 |
| 状态 | Draft |
| 目标读者 | 产品团队、架构团队、开发团队 |

---

## 2. 背景与目标

### 2.1 背景

当前 AI IDE 市场由 Cursor、Trae、Windsurf 等产品主导，它们以 AI 为核心重新定义了开发体验。StarCore IDE 以 Go + Wails + Svelte 技术栈为基础，具备轻量、高性能、原生体验的优势，但在 AI 能力深度、多 Agent 协作、自定义提供商等方面存在明显差距。

现有问题：
- AI 对话面板位置不合理（嵌入左侧边栏，应独立右侧面板）
- AI 提供商耦合单一 OpenAI 兼容接口，无法灵活扩展
- 缺少多 Agent/角色协作机制
- 缺少 Skills（技能插件）和 Memory（记忆）功能
- 界面布局与标准 AI IDE（Trae/VSCode）有较大差距
- 缺少命令面板、快捷键体系、Activity Bar 等核心 IDE 交互

### 2.2 目标

1. **对标 Trae 的 AI IDE 体验** — 右侧独立 AI 面板、Agent 选择器、上下文感知对话
2. **多 AI 提供商架构** — 每个提供商独立实现，接口标准化，易于维护和扩展
3. **多 Agent 协作** — 前端专家、后端架构师、产品经理、性能优化师、DevOps 工程师等角色
4. **Skills 技能系统** — 可复用的 AI 技能模块（代码审查、重构、测试生成等）
5. **Memory 记忆系统** — 跨会话上下文记忆、项目级知识库、用户偏好学习
6. **流畅交互** — 命令面板、快捷键、拖拽、分屏等标准 IDE 交互

---

## 3. 用户角色与场景

### 3.1 用户角色

| 角色 | 描述 | 核心诉求 |
|------|------|----------|
| 全栈开发者 | 日常编码、调试、部署 | 快速编写代码、AI 辅助补全、一键部署 |
| 前端工程师 | UI/UX 开发 | 组件生成、样式优化、AI 设计建议 |
| 后端工程师 | API/服务开发 | 接口设计、数据库优化、AI 架构建议 |
| DevOps 工程师 | CI/CD、容器化 | 部署脚本生成、监控配置、AI 运维建议 |
| 技术负责人 | 架构决策、代码审查 | 多 Agent 协作、代码质量把控、合规审查 |
| AI 应用开发者 | LLM 集成、Prompt 工程 | AI Provider 调试、模型对比、Prompt 优化 |

### 3.2 核心场景

| 场景 | 描述 | 涉及功能 |
|------|------|----------|
| SC-1 AI 对话编程 | 开发者在右侧面板与 AI 对话，选中代码作为上下文，AI 返回修改建议并一键 Apply | AI Chat、Code Context、Apply Diff |
| SC-2 多 Agent 协作 | 产品需求 → 前端设计 → 后端实现 → 测试生成，多 Agent 按流程协作 | Agent System、Workflow、Memory |
| SC-3 切换 AI 提供商 | 用户从 OpenAI 切换到 DeepSeek 或本地 Ollama，无缝切换 | Provider Manager、Settings |
| SC-4 使用 Skill | 开发者触发"生成单元测试"Skill，AI 自动为当前函数生成测试 | Skills、Code Context |
| SC-5 项目记忆 | AI 记住项目的技术栈、代码规范、历史决策，后续对话无需重复说明 | Memory、Project Knowledge |
| SC-6 命令面板 | Ctrl+Shift+P 呼出命令面板，搜索并执行任意命令 | Command Palette |
| SC-7 内联补全 | 编写代码时 AI 自动在光标位置生成灰度补全建议，Tab 接受 | Inline Completion |
| SC-8 分屏编辑 | 左右分屏对比两个文件，或一边编辑一边看 AI 对话 | Split Editor |

---

## 4. 功能需求

### 4.1 界面布局重构 (FR-1)

参照 Trae / VSCode 标准 AI IDE 布局：

```
┌──────────────────────────────────────────────────────────────┐
│ Title Bar (菜单 + 面包屑 + 窗口控制)                          │
├────┬─────────────────────────────┬──────────────────────────┤
│    │  Editor Group               │  AI Panel (右侧)         │
│ A  │  ┌──────┬──────┐           │  ┌──────────────────┐    │
│ c  │  │ Tab1 │ Tab2 │           │  │ Agent Selector   │    │
│ t  │  ├──────┴──────┤           │  ├──────────────────┤    │
│ i  │  │            │           │  │ Chat Messages     │    │
│ v  │  │  Code      │           │  │                  │    │
│ i  │  │  Editor    │           │  │                  │    │
│ t  │  │            │           │  ├──────────────────┤    │
│ y  │  ├────────────┤           │  │ Input Area       │    │
│    │  │ Terminal   │           │  │ + Context + Skills│    │
│ B  │  │ (Panel)    │           │  └──────────────────┘    │
│ a  │  └────────────┘           │                          │
│ r  │                           │                          │
├────┴─────────────────────────────┴──────────────────────────┤
│ Status Bar                                                   │
└──────────────────────────────────────────────────────────────┘
```

| 需求 ID | 需求描述 | 验收标准 |
|---------|----------|----------|
| FR-1.1 | Activity Bar 左侧 48px 图标栏 | 包含 Explorer/Search/Git/Extensions/AI 五个图标入口 |
| FR-1.2 | Sidebar 左侧可折叠面板（200px~400px） | 跟随 Activity Bar 切换内容，支持拖拽调整宽度 |
| FR-1.3 | Editor Group 中央编辑区域 | 支持多 Tab、分屏（水平/垂直）、欢迎页 |
| FR-1.4 | AI Panel 右侧独立面板（300px~600px） | 可拖拽调整宽度、可关闭/打开、Agent 选择器 + 对话区 + 输入区 |
| FR-1.5 | Bottom Panel 底部面板 | Terminal / Output / Problems 三个标签，可拖拽调整高度 |
| FR-1.6 | Title Bar 自定义标题栏 | 菜单按钮 + 文件路径面包屑 + 窗口控制（最小化/最大化/关闭） |
| FR-1.7 | Status Bar 底部状态栏 | 分支/语言/编码/AI 状态/通知等 |
| FR-1.8 | Command Palette 命令面板 | Ctrl+Shift+P 呼出，模糊搜索，执行命令 |
| FR-1.9 | Welcome Page 欢迎页 | 无文件打开时显示：最近项目/快捷操作/AI 快速入口 |

### 4.2 AI Provider 系统 (FR-2)

每个 Provider 独立实现，统一接口，避免耦合：

| 需求 ID | 需求描述 | 验收标准 |
|---------|----------|----------|
| FR-2.1 | Provider Interface 统一接口 | Go 接口定义：Chat/ChatStream/Completion/Embedding/ListModels |
| FR-2.2 | OpenAI Provider | 支持 GPT-4o/GPT-4.1/o3 等，流式/非流式 |
| FR-2.3 | Anthropic Provider | 支持 Claude 3.5/4 Sonnet/Opus，流式/非流式 |
| FR-2.4 | DeepSeek Provider | 支持 DeepSeek-V3/Coder，流式/非流式 |
| FR-2.5 | Ollama Provider | 本地模型，自动发现已安装模型 |
| FR-2.6 | Azure OpenAI Provider | Azure 部署的 OpenAI 模型 |
| FR-2.7 | 自定义 Provider | 用户可配置任意 OpenAI 兼容 API 端点 |
| FR-2.8 | Provider Manager 提供商管理 | 添加/删除/切换/测试连接/设置默认 |
| FR-2.9 | Model Picker 模型选择器 | AI 面板顶部下拉，按 Provider 分组显示模型 |
| FR-2.10 | Provider 配置隔离 | 每个 Provider 的 API Key/Endpoint/参数独立存储，加密保存 |

### 4.3 Agent 系统 (FR-3)

多 Agent 角色协作，每个 Agent 有专属 System Prompt + Tools + Skills：

| 需求 ID | 需求描述 | 验收标准 |
|---------|----------|----------|
| FR-3.1 | Agent Interface 统一接口 | 定义：ID/Name/Icon/SystemPrompt/Tools/Skills/Description |
| FR-3.2 | Agent Selector 选择器 | AI 面板顶部下拉/面板，显示所有可用 Agent |
| FR-3.3 | 内置 Agent — 全能助手 | 通用编程助手，默认 Agent |
| FR-3.4 | 内置 Agent — 前端架构师 | 前端框架/组件/状态管理/性能优化专家 |
| FR-3.5 | 内置 Agent — 后端架构师 | 后端架构/API 设计/数据库/微服务专家 |
| FR-3.6 | 内置 Agent — 产品经理 | 需求分析/PRD/用户故事/优先级排序 |
| FR-3.7 | 内置 Agent — UI 设计师 | UI/UX 设计/组件设计/样式/配色/设计系统 |
| FR-3.8 | 内置 Agent — DevOps 工程师 | CI/CD/Docker/K8s/部署/监控 |
| FR-3.9 | 内置 Agent — 性能优化师 | 性能分析/瓶颈定位/优化建议 |
| FR-3.10 | 内置 Agent — API 测试工程师 | API 测试/Mock/压力测试/覆盖率 |
| FR-3.11 | 内置 Agent — 合规审查员 | 代码合规/安全审计/规范检查 |
| FR-3.12 | 内置 Agent — AI 集成工程师 | LLM 集成/Prompt 工程/AI 应用开发 |
| FR-3.13 | Agent 配置 | 每个 Agent 可配置：默认模型/温度/最大 Token/System Prompt 追加 |
| FR-3.14 | Agent 对话隔离 | 每个 Agent 维护独立对话历史 |
| FR-3.15 | 多 Agent 协作流程 | 支持 Agent 间传递上下文，如产品经理输出 → 前端实现 |

### 4.4 Skills 技能系统 (FR-4)

Skills 是可复用的 AI 能力模块，可被 Agent 调用或用户直接触发：

| 需求 ID | 需求描述 | 验收标准 |
|---------|----------|----------|
| FR-4.1 | Skill Interface 统一接口 | 定义：ID/Name/Icon/Trigger/Handler/Agent 关联 |
| FR-4.2 | Skill — 生成单元测试 | 为选中函数/文件生成测试代码 |
| FR-4.3 | Skill — 代码审查 | 审查选中代码，输出问题列表和修改建议 |
| FR-4.4 | Skill — 重构建议 | 分析代码并提供重构方案 |
| FR-4.5 | Skill — 生成文档 | 为函数/模块生成 JSDoc/Godoc/Markdown 文档 |
| FR-4.6 | Skill — 解释代码 | 逐行解释选中代码的逻辑 |
| FR-4.7 | Skill — 修复 Bug | 分析错误信息，定位并修复 Bug |
| FR-4.8 | Skill — 生成 Commit Message | 分析 diff 生成规范 Commit Message |
| FR-4.9 | Skill — SQL 优化 | 分析 SQL 并提供索引/查询优化建议 |
| FR-4.10 | Skill 快捷入口 | 编辑器右键菜单 + 命令面板 + AI 面板输入区 /skill 触发 |
| FR-4.11 | Skill 结果操作 | 结果支持：Apply(应用修改)/Copy(复制)/Insert(插入到编辑器)/Discard |

### 4.5 Memory 记忆系统 (FR-5)

| 需求 ID | 需求描述 | 验收标准 |
|---------|----------|----------|
| FR-5.1 | 对话历史持久化 | 对话记录保存到本地，支持查看/搜索/删除历史 |
| FR-5.2 | 项目级知识库 | 记住项目的技术栈、目录结构、编码规范、重要决策 |
| FR-5.3 | 用户偏好记忆 | 记住用户的编码风格、常用模式、偏好设置 |
| FR-5.4 | 上下文自动收集 | 自动收集当前文件/选中代码/错误信息/项目结构作为 AI 上下文 |
| FR-5.5 | 上下文手动添加 | 用户可 @file @folder @url @web 添加额外上下文 |
| FR-5.6 | 记忆摘要压缩 | 长对话自动摘要压缩，保留关键信息减少 Token 消耗 |
| FR-5.7 | 知识库管理 | 查看/编辑/删除已记忆的知识条目 |

### 4.6 AI 对话交互 (FR-6)

| 需求 ID | 需求描述 | 验收标准 |
|---------|----------|----------|
| FR-6.1 | 流式响应 | AI 回复逐字流式渲染，支持 Markdown/代码块渲染 |
| FR-6.2 | 代码块操作 | 代码块支持：Copy/Apply/Diff 预览/在终端运行 |
| FR-6.3 | Apply Diff | AI 修改建议以 Diff 形式展示，用户可逐块 Accept/Reject |
| FR-6.4 | 上下文指示器 | 输入区显示当前附加上下文（文件/代码/错误），可编辑/删除 |
| FR-6.5 | 对话分支 | 支持从某条消息分叉出新对话分支 |
| FR-6.6 | 对话导出 | 导出对话为 Markdown/JSON |
| FR-6.7 | 新对话 / 历史对话 | 快速新建对话，浏览历史对话列表 |
| FR-6.8 | 消息操作 | 每条消息支持：复制/重新生成/编辑后重发/删除 |
| FR-6.9 | 思考过程展示 | 显示 AI 的推理/思考过程（如 o3/Claude 的 thinking） |
| FR-6.10 | 工具调用展示 | 显示 AI 调用了哪些工具（读文件/搜索/执行命令等）及结果 |

### 4.7 内联 AI 补全 (FR-7)

| 需求 ID | 需求描述 | 验收标准 |
|---------|----------|----------|
| FR-7.1 | 行内补全 | 光标位置 AI 生成灰度补全建议，Tab 接受，Esc 拒绝 |
| FR-7.2 | 多行补全 | 支持生成多行代码补全 |
| FR-7.3 | 部分接受 | Ctrl+→ 接受补全的下一个词 |
| FR-7.4 | 补全触发控制 | 可配置：自动/手动(Alt+\)/仅触发词后 |
| FR-7.5 | 补全状态指示 | 状态栏显示补全延迟/Provider/模型 |

### 4.8 编辑器增强 (FR-8)

| 需求 ID | 需求描述 | 验收标准 |
|---------|----------|----------|
| FR-8.1 | 分屏编辑 | 支持水平/垂直分屏，最多 4 个编辑器组 |
| FR-8.2 | 面包屑导航 | 标题栏下方显示文件路径面包屑，点击跳转 |
| FR-8.3 | Minimap | 代码缩略图，点击快速定位 |
| FR-8.4 | 多光标编辑 | Alt+Click / Ctrl+D 多光标 |
| FR-8.5 | 拖拽排序 Tab | 拖拽调整 Tab 顺序 |
| FR-8.6 | Pin Tab | 固定 Tab，不被自动关闭 |
| FR-8.7 | 文件差异编辑 | 打开 diff 视图对比两个文件 |

### 4.9 MCP 集成 (FR-9)

Model Context Protocol 是 AI IDE 与外部工具/数据源交互的标准协议：

| 需求 ID | 需求描述 | 验收标准 |
|---------|----------|----------|
| FR-9.1 | MCP Client | 支持 MCP 协议连接 MCP Server，获取 tools/resources/prompts |
| FR-9.2 | MCP Server 管理 | 添加/删除/启停 MCP Server（stdio/SSE 传输） |
| FR-9.3 | MCP Tool 集成 | MCP Server 暴露的 Tool 自动注册为 Agent 可用 Tool |
| FR-9.4 | MCP Resource 集成 | MCP Server 暴露的 Resource 可通过 @resource 引用为上下文 |
| FR-9.5 | 内置 MCP Server — FileSystem | 文件系统访问（受限于项目目录） |
| FR-9.6 | 内置 MCP Server — Git | Git 操作（status/diff/log/commit） |
| FR-9.7 | 内置 MCP Server — Terminal | 终端命令执行 |
| FR-9.8 | 社区 MCP Server | 支持配置外部 MCP Server（如 fetch/brave-search/slack 等） |

### 4.10 Agent Workflow/Pipeline (FR-10)

| 需求 ID | 需求描述 | 验收标准 |
|---------|----------|----------|
| FR-10.1 | Workflow 定义 | 定义多 Agent 串行/并行执行流程，如"需求→设计→实现→测试" |
| FR-10.2 | Workflow 触发 | 从 AI 面板或命令面板触发预定义 Workflow |
| FR-10.3 | Workflow 步骤传递 | 前一步骤输出自动作为下一步骤输入上下文 |
| FR-10.4 | Workflow 状态展示 | 显示当前执行到哪一步，每步状态（等待/执行中/完成/失败） |
| FR-10.5 | Workflow 暂停/继续 | 用户可在任意步骤暂停审查后再继续 |
| FR-10.6 | 预置 Workflow | 内置常见流程：代码实现全流程、Bug 修复流程、代码审查流程 |

### 4.11 Token 用量追踪 (FR-11)

| 需求 ID | 需求描述 | 验收标准 |
|---------|----------|----------|
| FR-11.1 | 对话级 Token 统计 | 每次对话显示 input/output Token 数 |
| FR-11.2 | 项目级 Token 汇总 | 按天/周/月汇总 Token 用量 |
| FR-11.3 | Provider 费用估算 | 基于模型定价估算费用，可配置价格 |
| FR-11.4 | 用量面板 | 设置中显示 Token 用量统计图表 |
| FR-11.5 | 预算提醒 | 可设置月度预算上限，接近时提醒 |

### 4.12 多模态支持 (FR-12)

| 需求 ID | 需求描述 | 验收标准 |
|---------|----------|----------|
| FR-12.1 | 图片输入 | 在 AI 对话中粘贴/拖拽图片，发送给 Vision 模型 |
| FR-12.2 | 截图输入 | 截取屏幕区域发送给 AI |
| FR-12.3 | 图片预览 | 对话中图片缩略图预览，点击放大 |
| FR-12.4 | Provider 能力检测 | 仅在支持 Vision 的模型时显示图片输入选项 |

### 4.13 插件/扩展系统 (FR-13)

| 需求 ID | 需求描述 | 验收标准 |
|---------|----------|----------|
| FR-13.1 | Extension API | 提供 JS/TS Extension API，可注册命令/视图/状态栏项/补全提供者 |
| FR-13.2 | Extension 加载 | 从 extensions/ 目录加载，沙箱化执行 |
| FR-13.3 | Extension Marketplace | 内置扩展列表界面，安装/卸载/更新 |
| FR-13.4 | Extension 配置 | 每个 Extension 可有 settings.json 配置 |
| FR-13.5 | Extension 通信 | Extension 可调用 Wails IPC 后端方法 |

### 4.14 调试运行 (FR-14)

| 需求 ID | 需求描述 | 验收标准 |
|---------|----------|----------|
| FR-14.1 | Run Configuration | launch.json 配置，支持配置运行参数 |
| FR-14.2 | DAP 集成 | Debug Adapter Protocol，支持断点/单步/变量查看 |
| FR-14.3 | 调试 UI | 断点红点/调试工具栏/变量面板/调用栈面板 |
| FR-14.4 | 内置 Go 调试 | Go 程序 Delve 调试支持 |

### 4.15 自动更新与遥测 (FR-15)

| 需求 ID | 需求描述 | 验收标准 |
|---------|----------|----------|
| FR-15.1 | 自动更新检测 | 启动时检查新版本，提示更新 |
| FR-15.2 | 增量更新 | 仅下载差异部分，减少下载量 |
| FR-15.3 | 匿名遥测 | 可选的匿名使用数据收集（功能使用频率/性能指标） |
| FR-15.4 | 崩溃报告 | 自动收集崩溃日志，提示上报 |

### 4.16 核心基础设施 (FR-16)

| 需求 ID | 需求描述 | 验收标准 |
|---------|----------|----------|
| FR-16.1 | 真实 PTY 终端 | Go 后端 PTY 连接，支持 bash/zsh/powershell |
| FR-16.2 | LSP 集成 | Go/TypeScript/Python LSP，提供诊断/补全/跳转 |
| FR-16.3 | Git 集成 | 文件变更指示/Commit/Branch/Merge/Diff 视图 |
| FR-16.4 | 快捷键体系 | 完整快捷键绑定，支持自定义 |
| FR-16.5 | 通知系统 | 右下角 Toast 通知，支持 info/warning/error/progress |
| FR-16.6 | 主题系统 | Light/Dark/High Contrast 主题 + 自定义主题 |
| FR-16.7 | 国际化 | 中/英双语，可扩展更多语言 |
| FR-16.8 | 可访问性 | ARIA 标签、键盘导航、屏幕阅读器兼容 |

---

## 5. 非功能需求

| 需求 ID | 描述 | 指标 |
|---------|------|------|
| NFR-1 | 启动时间 | 冷启动 < 800ms |
| NFR-2 | UI 延迟 | 交互响应 < 50ms |
| NFR-3 | 内存占用 | 空载 < 150MB，AI 对话时 < 300MB |
| NFR-4 | AI 流式延迟 | 首 Token 延迟 < 2s（取决于网络和 Provider） |
| NFR-5 | 安全性 | API Key 加密存储，不落日志，不泄露到前端 |
| NFR-6 | 可扩展性 | Provider/Agent/Skill 均可独立开发和注册 |
| NFR-7 | 可维护性 | 每个 Provider 独立文件，Agent 独立目录，Skill 独立模块 |
| NFR-8 | 兼容性 | Windows 10+/macOS 12+ |

---

## 6. 交互/原型规格

### 6.1 AI 面板详细交互

**面板结构（从上到下）：**

1. **面板头部** — Agent 选择器 + Model 选择器 + 新对话按钮 + 历史按钮 + 关闭按钮
2. **对话消息区** — 消息列表，支持滚动加载更多
   - 用户消息：头像 + 文本 + 附加上下文标签
   - AI 消息：Agent 头像 + 流式文本(Markdown渲染) + 代码块(带操作按钮) + 工具调用记录 + 思考过程(可折叠)
3. **上下文预览区** — 显示当前附加的文件/代码/错误上下文，可删除
4. **输入区** — 多行输入框 + 发送按钮 + 附件按钮 + Skill 快捷按钮 + 上下文按钮

**Agent 选择器交互：**
- 点击 Agent 名称 → 下拉面板显示所有 Agent 列表
- 每个 Agent 显示：图标 + 名称 + 简短描述
- 选中的 Agent 高亮，齿轮图标进入配置

### 6.2 Activity Bar 图标顺序

从上到下：Explorer → Search → Git → AI → Extensions

AI 图标点击：切换右侧 AI Panel 的显隐

### 6.3 命令面板

Ctrl+Shift+P 触发，顶部居中浮层，模糊搜索，分类展示：
- 文件操作
- 编辑器操作
- AI 操作（切换 Agent/切换 Model/触发 Skill）
- 视图操作
- 终端操作

### 6.4 右键菜单增强

编辑器右键新增：
- AI: 解释代码
- AI: 重构建议
- AI: 生成测试
- AI: 生成文档
- AI: 修复问题
- AI: 添加到对话上下文

---

## 7. 数据需求

### 7.1 Provider 配置数据

| 字段 | 类型 | 说明 |
|------|------|------|
| id | string | 唯一标识 (openai/anthropic/deepseek/ollama/azure/custom) |
| name | string | 显示名称 |
| api_key | string(encrypted) | 加密存储的 API Key |
| endpoint | string | API 端点 |
| models | Model[] | 可用模型列表 |
| is_default | bool | 是否默认提供商 |
| config | map | 额外配置参数 |

### 7.2 Agent 数据

| 字段 | 类型 | 说明 |
|------|------|------|
| id | string | 唯一标识 (frontend-architect 等) |
| name | string | 显示名称 |
| icon | string | 图标标识 |
| description | string | 简短描述 |
| system_prompt | string | 系统 Prompt |
| default_model | string | 默认模型 |
| skills | string[] | 关联 Skill ID 列表 |
| config | AgentConfig | 配置参数 |

### 7.3 Memory 数据

| 字段 | 类型 | 说明 |
|------|------|------|
| project_path | string | 项目路径 |
| knowledge_entries | Knowledge[] | 知识条目列表 |
| preferences | map | 用户偏好 |
| conversation_summaries | Summary[] | 对话摘要 |

---

## 8. 依赖与约束

| 依赖 | 说明 |
|------|------|
| Wails v2 | 框架约束，IPC 通信、事件系统 |
| CodeMirror 6 | 编辑器，需自行实现 AI 补全装饰层 |
| Go 1.23 | 后端语言，Provider 接口用 Go interface |
| Svelte 5 | 前端框架，组件化开发 |
| 各 AI Provider API | 外部依赖，需处理 API 变更和兼容 |

**约束：**
- API Key 绝不传输到前端，所有 AI 请求必须经 Go 后端代理
- Provider 实现必须遵循统一接口，禁止绕过接口直接调用
- Agent 的 System Prompt 不可被用户对话注入覆盖

---

## 9. 里程碑与实施计划

### Phase 1 — 界面布局重构 (2 周)
- FR-1.1 ~ FR-1.9：Activity Bar + 右侧 AI Panel + 命令面板 + 欢迎页
- 优先级：最高，所有后续功能依赖正确布局

### Phase 2 — AI Provider 系统重构 (2 周)
- FR-2.1 ~ FR-2.10：统一接口 + OpenAI + Anthropic + DeepSeek + Ollama 实现
- Provider Manager + Model Picker + 配置隔离

### Phase 3 — Agent 系统 (2 周)
- FR-3.1 ~ FR-3.15：Agent Interface + 全部内置 Agent + Agent Selector + 配置
- 多 Agent 协作流程基础

### Phase 4 — Skills + Memory (2 周)
- FR-4.1 ~ FR-4.11：Skill Interface + 全部内置 Skill
- FR-5.1 ~ FR-5.7：对话持久化 + 项目知识库 + 上下文收集

### Phase 5 — AI 对话交互增强 (1.5 周)
- FR-6.1 ~ FR-6.10：流式渲染增强 + Apply Diff + 工具调用展示 + 思考过程

### Phase 6 — 内联补全 + 编辑器增强 (2 周)
- FR-7.1 ~ FR-7.5：行内补全系统
- FR-8.1 ~ FR-8.7：分屏/面包屑/Minimap

### Phase 7 — 核心基础设施 (2 周)
- FR-16.1 ~ FR-16.8：PTY/LSP/Git/快捷键/通知/主题/i18n/可访问性

### Phase 8 — MCP + Workflow + 多模态 (2 周)
- FR-9.1 ~ FR-9.8：MCP Client/Server 集成
- FR-10.1 ~ FR-10.6：Agent Workflow/Pipeline
- FR-12.1 ~ FR-12.4：多模态支持

### Phase 9 — 扩展系统 + 调试 + 更新 (2 周)
- FR-13.1 ~ FR-13.5：Extension API + Marketplace
- FR-14.1 ~ FR-14.4：调试运行
- FR-15.1 ~ FR-15.4：自动更新 + 遥测

### Phase 10 — Token 追测 + 优化 + 发布 (1 周)
- FR-11.1 ~ FR-11.5：Token 用量追踪
- 全链路性能优化、安全审计、打包发布

### 总计：约 18.5 周

---

## 10. 附录

### A. 参考产品对比

| 功能 | Trae | Cursor | VSCode+Copilot | StarCore 目标 |
|------|------|--------|----------------|---------------|
| AI 右侧面板 | ✅ | ✅ | ❌(Copilot Chat) | ✅ |
| 多 Agent | ✅ | ❌ | ❌ | ✅ |
| Agent 选择器 | ✅ | ❌ | ❌ | ✅ |
| 多 Provider | ✅ | ✅ | ✅(有限) | ✅ |
| Skills | ❌ | ❌ | ❌ | ✅ |
| Memory | ❌ | ✅(有限) | ❌ | ✅ |
| 内联补全 | ✅ | ✅ | ✅ | ✅ |
| Apply Diff | ✅ | ✅ | ❌ | ✅ |
| 命令面板 | ✅ | ✅ | ✅ | ✅ |
| 原生性能 | ❌(Electron) | ❌(Electron) | ❌(Electron) | ✅(Wails) |

### B. 截图中 Agent 列表

与截图对应的 Agent 角色规划：

1. 合规审查员 / compliance-checker — 图标：盾牌✓
2. 性能优化师 / performance-expert — 图标：仪表盘
3. DevOps 工程师 / devops-architect — 图标：服务器
4. AI 集成工程师 / ai-integration-engineer — 图标：机器人
5. API 测试工程师 / api-test-pro — 图标：试管
6. 后端架构师 / backend-architect — 图标：代码块
7. 前端架构师 / frontend-architect — 图标：浏览器
8. UI 设计师 / ui-designer — 图标：调色板

### C. 核心快捷键映射表

| 快捷键 | 功能 | 上下文 |
|--------|------|--------|
| Ctrl+Shift+P | 命令面板 | 全局 |
| Ctrl+P | 快速打开文件 | 全局 |
| Ctrl+S | 保存文件 | 编辑器 |
| Ctrl+W | 关闭当前 Tab | 编辑器 |
| Ctrl+\ | 垂直分屏 | 编辑器 |
| Ctrl+K Ctrl+\ | 水平分屏 | 编辑器 |
| Ctrl+` | 切换终端面板 | 全局 |
| Ctrl+Shift+E | 聚焦资源管理器 | 全局 |
| Ctrl+Shift+F | 全局搜索 | 全局 |
| Ctrl+Shift+G | 聚焦 Git 面板 | 全局 |
| Ctrl+Shift+A | 切换 AI 面板 | 全局 |
| Ctrl+Shift+M | 切换 Agent 选择器 | AI 面板 |
| Ctrl+L | 清空 AI 对话 | AI 面板 |
| Ctrl+Enter | 发送 AI 消息 | AI 面板输入区 |
| @ | 触发上下文引用 | AI 面板输入区 |
| / | 触发 Skill 命令 | AI 面板输入区 |
| Tab | 接受内联补全 | 编辑器补全激活时 |
| Esc | 拒绝内联补全 | 编辑器补全激活时 |
| Ctrl+→ | 接受补全下一词 | 编辑器补全激活时 |
| Alt+\ | 手动触发补全 | 编辑器 |
| F5 | 开始调试 | 全局 |
| F9 | 切换断点 | 编辑器 |
| F10 | 单步跳过 | 调试中 |
| F11 | 单步进入 | 调试中 |

### D. 遗产代码迁移计划

现有 `app.go` 中的 AI 实现需逐步重构，迁移顺序：

| 步骤 | 内容 | 风险 |
|------|------|------|
| Step 1 | 新建 `internal/` 包结构，不动 app.go | 无风险 |
| Step 2 | 实现 Provider Interface + Manager，OpenAI Provider 复制 app.go 中现有逻辑 | 低风险 |
| Step 3 | app.go 中 AIChatStream 改为调用 ProviderManager | 中风险，需回归测试 |
| Step 4 | 逐步实现 Anthropic/DeepSeek/Ollama Provider | 低风险，新增代码 |
| Step 5 | 实现 Agent Runtime，app.go 新增 Agent 相关 IPC 方法 | 低风险 |
| Step 6 | 前端重构：AIChat → AIPanel + AgentSelector + ModelSelector | 高风险，UI 大改 |
| Step 7 | 前端布局重构：App.svelte 四区域布局 | 高风险，核心 UI |
| Step 8 | 实现 Memory/Skill/MCP 等增量模块 | 低风险，新增代码 |
