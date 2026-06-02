# StarCore IDE

**AI 驱动的下一代桌面 IDE** — 对标 Cursor / Claude Code，用 Go + Svelte 5 构建。

## 特性

- 🤖 **AI Agent 自动编程** — 让 AI 读代码、写代码、搜文件、运行命令，自主完成任务
- 💡 **24 个内置技能** — 代码审查、生成测试、安全检查、性能分析、API 设计、数据库建模...
- 🎯 **9 种专业 Agent** — 通用助手、前后端架构师、DevOps 工程师、产品经理...
- 📝 **智能代码补全** — Ghost text 实时补全，按 Tab 接受
- 🔧 **完整 IDE** — 编辑器 (CodeMirror 6)、终端、文件树、Git 面板
- 🌐 **多模型支持** — OpenAI / Anthropic / DeepSeek / Ollama 本地模型
- 🧩 **Skills 扩展** — 可视化创建自定义 AI 技能，一键安装
- 🎨 **现代 UI** — 无边框深色主题、绿色视觉标识、流畅动画

## 快速开始

### 前提条件
- Go 1.23+
- Node.js 18+
- Wails CLI: `go install github.com/wailsapp/wails/v2/cmd/wails@latest`

### 开发模式
```bash
git clone <repo>
cd StarCore
wails dev
```

### 构建
```bash
# Windows
wails build -platform windows -arch amd64

# macOS
wails build -platform darwin -arch arm64

# Linux
wails build -platform linux -arch amd64
```

或使用 Makefile：
```bash
make build-windows  # 构建 Windows exe
make test           # 运行测试
```

## 配置 AI 提供商

首次启动会自动弹出配置引导。也可在 **设置 → AI** 中手动配置：

| 提供商 | 默认端点 | 获取 API Key |
|--------|----------|-------------|
| OpenAI | `https://api.openai.com/v1` | [platform.openai.com](https://platform.openai.com) |
| Anthropic | `https://api.anthropic.com/v1` | [console.anthropic.com](https://console.anthropic.com) |
| DeepSeek | `https://api.deepseek.com/v1` | [platform.deepseek.com](https://platform.deepseek.com) |
| Ollama | `http://localhost:11434` | 无需 Key（本地运行） |

**推荐免费方案**：安装 [Ollama](https://ollama.com)，运行 `ollama pull qwen2.5-coder:7b`，然后在 StarCore 中添加 Ollama 提供商即可。

## Skills 系统

内置 **24 个**开箱即用的技能，覆盖全开发流程。在 AI 对话中可用 `/skill-name` 触发。

### 代码类
| Skill | 用途 |
|-------|------|
| 🧪 生成单元测试 | 自动生成测试，覆盖正常/边界/错误场景 |
| 🔍 代码审查 | 全方位审查：正确性、可读性、性能、安全 |
| ♻️ 重构建议 | 单一职责、消除重复、简化逻辑 |
| 🐛 调试分析 | 根据错误 + 调用栈深度追踪根因 |
| 🛡️ 安全检查 | OWASP Top 10 漏洞扫描 |
| 📊 性能分析 | 时间/空间复杂度 + 优化方案 |
| 🔄 错误处理完善 | wrap/unwrap、降级、重试、日志 |

### 项目/运维
| Skill | 用途 |
|-------|------|
| ⚙️ 项目初始化配置 | ESLint/Docker/CI/.gitignore 一键生成 |
| 📦 依赖分析 | 过期、CVE 漏洞、许可证检查 |
| 📖 生成 README | 根据代码自动生成专业 README |
| 📋 日志分析 | 错误模式识别、告警建议 |
| 💻 Shell 脚本生成 | 生产级 shell 脚本 |

### Git
| Skill | 用途 |
|-------|------|
| ✅ PR 审查 | blocker/suggestion/question 分级审查 |
| 📋 Commit Message | Conventional Commits 格式 |

### 数据库 / 设计
| Skill | 用途 |
|-------|------|
| 🚀 SQL 优化 | 索引 + 查询改写 + 执行计划分析 |
| 📜 数据库迁移脚本 | Up/Down 脚本，幂等、可回滚 |
| 🗄️ 数据建模 | ER 关系 + CREATE TABLE + 索引设计 |
| 🔌 API 设计 | RESTful 端点、JSON Schema、认证、限流 |

## .starcorerules（项目规则）

在项目根目录创建 `.starcorerules`（也兼容 `.cursorrules` 和 `CLAUDE.md`），Agent 会自动加载：

```
始终用中文回复
测试框架使用 vitest
不要引入新的第三方依赖
```

## 项目结构

```
StarCore/
├── app.go              # Go 后端入口 + Wails 绑定
├── main.go             # 应用入口
├── internal/
│   ├── agent/          # Agent 系统 + 工具
│   ├── ai/             # AI 对话服务 + Agent Loop
│   ├── context/        # 上下文构建 + 消息压缩
│   ├── files/          # 文件操作 + 搜索 + Diff
│   ├── git/            # Git 操作
│   ├── terminal/       # 终端管理
│   ├── provider/       # LLM 提供商 (OpenAI/Anthropic/Ollama)
│   ├── skill/          # Skills 系统
│   ├── lsp/            # LSP 语言服务器
│   ├── mcp/            # MCP 协议
│   ├── memory/         # 对话记忆持久化
│   └── watcher/        # 文件监听
├── frontend/
│   └── src/
│       ├── components/ # Svelte 组件
│       └── stores/     # 状态管理
└── build/              # 构建资源
```

## 技术栈

| 层 | 技术 |
|---|------|
| 后端 | Go 1.23 + Wails v2 |
| 前端 | Svelte 5 + CodeMirror 6 + Xterm.js + Tailwind CSS v4 |
| AI | OpenAI / Anthropic / DeepSeek / Ollama 兼容 API |
| 数据库 | SQLite (对话记忆) |
| LSP | gopls / typescript-language-server / pyright |

## License

MIT
