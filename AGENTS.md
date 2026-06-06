# StarCore IDE — Agent Guidelines

Native desktop IDE。使用 Go 1.23 + Wails v2 后端，Svelte 5 + CodeMirror 6 + Xterm.js + Tailwind CSS v4 前端。

## 构建与测试命令

### 开发模式
```bash
wails dev              # 开发模式（热重载）
npm run dev            # 前端开发服务器
```

### 构建命令
```bash
wails build            # 构建生产二进制文件
make build             # 构建（包含资源文件生成）
make build-windows     # 构建Windows可执行文件
make installer         # 创建NSIS安装器（需要makensis）
make clean             # 清理构建产物
make release           # 构建Windows发行版并打包
```

### 测试命令
```bash
go test ./... -cover   # 运行所有测试并显示覆盖率
go test ./... -v       # 详细模式运行所有测试
make test              # 运行内部包测试（相当于：go test ./internal/... -v）

# 运行特定包或测试
go test ./internal/context -v
go test ./internal/context -v -run TestBuildContextMessage
go test ./internal/ai -v
go test ./internal/provider -v
```

### 跳过的测试（平台限制）
以下测试因平台特性被跳过（`t.Skip`）：
- `TestSearchFiles` - 需要`chdir`；建议使用`wails dev`手动测试
- `TestGitStageAndCommit` - Windows cmd.exe `%%`转义与git `--format`冲突

## 项目架构

**入口点**: `main.go` 通过 `//go:embed` 嵌入 `frontend/dist`，调用 `NewApp()`。
**应用装配**: `app.go` 实例化所有服务，将 `*App` 绑定到Wails前端。
**前后端桥接**: `frontend/wailsjs/go/main/App.js`（自动生成的绑定）。后端通过 `wailsRuntime.EventsEmit` 发送事件；前端通过 `EventsOn` 监听。
**事件**: `app:first-run`, `skill:stream:*`, `terminal:*` 等。

### 内部包结构（`internal/`）
| 包 | 用途 |
|---------|---------|
| `provider/` | LLM提供商：OpenAI、Anthropic、Ollama（均实现`Provider`接口）。错误诊断在`diagnosis.go`。 |
| `agent/` | Agent注册表 + 工具系统。`tools/`目录下包含15个工具（读/写/编辑文件、执行命令、全局搜索/文件搜索、Git操作、网络获取、HTTP请求、技能工具、子代理）。 |
| `ai/` | Agent循环服务（`maxAgentLoops=100`, `maxToolResultChars=8000`）。 |
| `context/` | AI上下文构建器 — 附加项目结构、上下文文件、活动文件、选中代码。包含压缩功能。 |
| `memory/` | SQLite后端（使用`mattn/go-sqlite3`）。对话、知识库、令牌使用情况。 |
| `skill/` | 技能系统：内置 + 外部（从配置目录加载）。 |
| `lsp/` | gopls、typescript-language-server、pyright等语言服务器。 |
| `mcp/` | 模型上下文协议服务器管理。 |
| `terminal/` | PTY管理（通过Windows的`conpty`）。 |
| `watcher/` | 文件系统监视器。 |

## 关键Go依赖

- `github.com/wailsapp/wails/v2 v2.12.0`
- `github.com/mattn/go-sqlite3 v1.14.44` (SQLite + WAL模式 + busy timeout 5000)
- `github.com/UserExistsError/conpty v0.1.4` (Windows PTY)
- `golang.org/x/net v0.35.0` (网络库)

## 前端架构

- **Svelte 5** 使用rune（`$state`, `$derived`）组件 + 传统的`svelte/store`（可写/派生/获取）存储。
- **Tailwind CSS v4**: `style.css`中的`@import "tailwindcss"`。Vite插件：`@tailwindcss/vite`。主题使用的CSS自定义属性（`--bg-primary`, `--accent`等）。
- **CodeMirror 6** 语言支持：go、js/ts、json、html、css、md、python、rust、java、cpp、php、sql、xml、yaml。
- **无TypeScript** — JS文件中使用JSDoc进行类型提示。
- **状态管理**: `stores/`目录下的20个存储（ui、app、ai、provider、agent、skill、git、memory、theme等）。UI面板大小/可见性持久化到localStorage。
- **`svelte.config.js`** 设置`componentApi: 4`用于兼容性。`package.json`中对`@sveltejs/vite-plugin-svelte ^4.0.0-next.6`有覆盖配置。
- **窗口**: 无边框，默认1600×960（最小800×600），深色背景`#11111b`。

## 代码风格指南

### Go语言
- **缩进**: 使用制表符（tab）
- **导入分组**: 标准库 / 外部库 / 内部库
- **格式化**: 始终运行`go fmt`
- **错误处理**: 使用命名返回值或显式错误处理
- **命名约定**: 
  - 包名：小写单数名词
  - 接口名：方法名 + "er"（如`Provider`）
  - 变量名：小写驼峰
  - 常量名：大写蛇形

### JavaScript/Svelte/CSS
- **缩进**: 2个空格
- **行尾**: LF（Unix风格）
- **编码**: UTF-8
- **文件结尾**: 包含尾随换行符
- **空格**: 去除尾随空格（通过`.editorconfig`强制执行）
- **命名约定**:
  - 组件名：帕斯卡命名法（`MyComponent.svelte`）
  - 存储：小写驼峰
  - 常量：大写蛇形

### 配置文件
- **`.editorconfig`**: 强制执行编码、行尾和缩进规则
- **`jsconfig.json`**: 配置路径别名`@/*`
- **`svelte.config.js`**: Svelte配置
- **`vite.config.js`**: Vite + Tailwind CSS v4配置

## Agent工具系统

`internal/agent/tools/`目录下包含15个工具。除基本CRUD外的重要工具：

- `skill_tool.go` — 对话中执行技能
- `sub_agent.go` — 生成子代理进行并行任务
- `web_fetch.go` — 获取URL内容
- `http_request.go` — 执行任意HTTP请求
- `get_diagnostics.go` — 获取当前文件的LSP诊断信息
- `get_git_diff.go`, `git_commit.go` — Git集成
- `read_file.go`, `write_file.go`, `edit_file.go` — 文件操作
- `execute_command.go` — 执行命令
- `glob_files.go`, `search_files.go` — 文件搜索
- `get_context.go` — 获取上下文信息

## AI对话框常见错误调试指南

当AI对话框发送内容后模型回答报错时，常见原因和调试步骤：

### 常见错误类型

1. **认证错误（401/403）**
   - **症状**: "API密钥无效"、"unauthorized"、"api key"
   - **解决方法**: 检查Provider配置，确保API密钥正确

2. **频率限制（429）**
   - **症状**: "请求频率限制"、"rate limit"、"too many requests"
   - **解决方法**: 稍后重试，或升级API套餐

3. **上下文长度限制**
   - **症状**: "对话过长"、"context_length"、"token limit"
   - **解决方法**: 开始新对话，或减少对话历史

4. **服务端错误（500/502/503/504）**
   - **症状**: "AI服务暂时不可用"、"server error"
   - **解决方法**: 稍后重试，或切换提供商

5. **网络连接错误**
   - **症状**: "网络连接失败"、"network"、"timeout"、"connection refused"
   - **解决方法**: 检查网络设置、代理配置、防火墙规则

### 后端错误位置
- `internal/provider/diagnosis.go` - 错误诊断和分类
- `internal/provider/openai.go:202-204` - API密钥验证
- `internal/provider/openai.go:239-253` - 端点地址验证
- `internal/service.go:329-332` - Provider认证失败处理

### 前端错误处理
- `frontend/src/stores/ai.js:56-75` - `classifyError`错误分类函数
- `frontend/src/stores/ai.js:461-480` - 流事件错误处理
- 超时设置：流超时120秒，首块超时30秒

### 调试步骤
1. **检查Provider配置**
   - 验证API密钥是否正确配置
   - 检查端点地址格式（OpenAI、Anthropic、Ollama）
   - 确认模型名称有效

2. **检查网络环境**
   - 验证网络连接
   - 检查代理设置（如使用代理）
   - 确认防火墙未阻止API访问

3. **查看开发者控制台**
   - 打开开发者工具（F12）
   - 查看Console标签中的错误信息
   - 查看Network标签中的API请求状态

4. **检查localStorage**
   - 查看保存的Provider配置
   - 检查token使用情况

5. **测试不同Provider**
   - 切换不同的AI提供商测试
   - 尝试使用不同的模型

### 错误日志位置
- 开发者控制台（浏览器）
- 应用日志文件（如启用）
- 后端控制台输出（`wails dev`运行时）

## 杂项信息

- **无CI/CD**：没有GitHub Actions，构建流程通过`Makefile`管理
- **项目配置目录**: `os.UserConfigDir()/StarCore/`（提供商配置、技能、SQLite数据库、自定义模型）
- **`.trae/`和`.codeartsdoer/`**: 其他AI编码工具的产物 — 非项目配置
- **`wails generate`**: 添加新Go方法后重新生成`frontend/wailsjs/`
- **项目规则**: 支持`.starcorerules`、`.cursorrules`、`CLAUDE.md`文件，Agent会自动加载这些规则文件

## 重要注意事项

1. **代码规范**: 修改代码后务必运行`go fmt`和`go test`
2. **工具使用**: Agent使用的工具位于`internal/agent/tools/`，新增工具需实现`Tool`接口
3. **错误处理**: 所有错误都应通过`classifyError`或`DiagnoseError`进行友好分类
4. **事件通信**: 前后端通过Wails事件系统通信，事件名格式为`domain:action:detail`
5. **存储状态**: UI状态保存在localStorage中，应用重启后恢复
6. **国际化**: 错误消息支持中英文，通过错误信息关键词匹配语言

## 开发工作流

1. **启动开发环境**: `wails dev`（后端 + 前端热重载）
2. **运行测试**: `make test` 或 `go test ./internal/... -v`
3. **代码格式化**: `go fmt ./...`
4. **构建发布**: `make build-windows` + `make installer`
5. **调试**: 使用开发者控制台和日志输出

---

*最后更新: 2025年6月5日*