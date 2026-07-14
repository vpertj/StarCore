# StarCore IDE

**AI-Native Desktop IDE** — Built with Go + Svelte 5, powered by a multi-agent architecture with 21 tools, 10 agent roles, and autonomous task execution.

[![Build](https://github.com/vpertj/StarCore/actions/workflows/ci.yml/badge.svg)](https://github.com/vpertj/StarCore/actions)
[![Release](https://github.com/vpertj/StarCore/actions/workflows/release.yml/badge.svg)](https://github.com/vpertj/StarCore/actions)
[![License](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)

---

## Table of Contents

- [Highlights](#highlights)
- [Architecture](#architecture)
- [Agent Orchestration](#agent-orchestration)
- [Tech Stack](#tech-stack)
- [Quick Start](#quick-start)
- [Configuration](#configuration)
- [Skills System](#skills-system)
- [Project Structure](#project-structure)
- [License](#license)

---

## Highlights

- **Multi-Agent System** — 10 specialized agent roles (Universal, Frontend/Backend Architect, DevOps, QA, PM...) with capability-based routing
- **21 Built-in Tools** — File CRUD, command execution, code search, Git operations, HTTP requests, LSP diagnostics, sub-agents, and more
- **Autonomous Task Execution** — Agent loop with up to 80 iterations, automatic tool calling, error recovery, and anti-drift mechanisms
- **Context-Aware** — Project structure analysis, dependency graph, RAG semantic search, knowledge base, and smart context compression
- **Multi-Model** — OpenAI, Anthropic, DeepSeek, Ollama (local) with automatic failover and cost tracking
- **Skills System** — 24+ built-in skills for code review, testing, security audit, SQL optimization, API design...
- **Full IDE** — CodeMirror 6 editor, Xterm.js terminal, Git panel, file explorer, LSP support
- **Cross-Platform** — Windows, macOS, Linux

---

## Architecture

```
┌─────────────────────────────────────────────────────────────────┐
│                        Frontend (Svelte 5)                       │
│  ┌──────────┐  ┌──────────┐  ┌──────────┐  ┌────────────────┐  │
│  │ AIPanel  │  │ CodeEdit │  │ Terminal │  │ Git/File Panel │  │
│  └────┬─────┘  └──────────┘  └──────────┘  └────────────────┘  │
│       │ Wails Event System                                       │
├───────┼─────────────────────────────────────────────────────────┤
│       │                 Backend (Go 1.23)                        │
│  ┌────┴─────────────────────────────────────────────────────┐   │
│  │                    AI Service Layer                        │   │
│  │  ┌──────────┐  ┌──────────┐  ┌──────────┐  ┌──────────┐  │   │
│  │  │  Agent   │  │ Context  │  │ Provider │  │  Memory  │  │   │
│  │  │  Loop    │  │ Builder  │  │ Manager  │  │  Store   │  │   │
│  │  └────┬─────┘  └──────────┘  └──────────┘  └──────────┘  │   │
│  │       │                                                     │   │
│  │  ┌────┴─────────────────────────────────────────────────┐  │   │
│  │  │              Tool Execution Layer                     │  │   │
│  │  │  21 Tools │ Sub-Agents │ Skills │ LSP │ MCP         │  │   │
│  │  └─────────────────────────────────────────────────────┘  │   │
│  └───────────────────────────────────────────────────────────┘   │
└─────────────────────────────────────────────────────────────────┘
```

---

## Agent Orchestration

### Agent Loop (Core Engine)

The agent loop is the heart of StarCore's autonomous coding capability. It manages the entire lifecycle of an AI task:

```
User Request
    │
    ▼
┌─────────────────┐
│ Intent Classifier│ ──→ Determines task type (code_edit, debug, refactor, review...)
└────────┬────────┘
         │
         ▼
┌─────────────────┐
│  Agent Router   │ ──→ Selects optimal agent based on intent + capabilities
└────────┬────────┘
         │
         ▼
┌─────────────────────────────────────────────────────────────┐
│                    Agent Loop (max 80 iterations)            │
│                                                              │
│  ┌─── For each iteration ─────────────────────────────────┐ │
│  │                                                         │ │
│  │  1. Build Context (stable prefix + dynamic suffix)      │ │
│  │     ├─ Git status + Code structure analysis             │ │
│  │     ├─ Project rules (.starcorerules)                   │ │
│  │     ├─ Knowledge base + RAG results                     │ │
│  │     ├─ Active file + Selected code                      │ │
│  │     └─ Conversation history (compressed if needed)      │ │
│  │                                                         │ │
│  │  2. Inject Loop State                                   │ │
│  │     ├─ Todo list + Progress percentage                  │ │
│  │     ├─ Files touched this session                       │ │
│  │     ├─ Key decisions (max 5, FIFO)                      │ │
│  │     └─ Anti-drift: re-inject original goal every 10 rnd │ │
│  │                                                         │ │
│  │  3. Call LLM (with retry + circuit breaker)             │ │
│  │     ├─ Streaming response                               │ │
│  │     ├─ Repetition detection (interrupt if looping)      │ │
│  │     └─ Tool call parsing (function calling + text)      │ │
│  │                                                         │ │
│  │  4. Execute Tools (parallel goroutines)                 │ │
│  │     ├─ Timeout: 60s (6min for approval-required)        │ │
│  │     ├─ Result truncation (dynamic budget, max 12K)      │ │
│  │     ├─ Auto-verify (build mode: go test, npm build...)  │ │
│  │     └─ Syntax check (go fmt, py_compile, tsc)           │ │
│  │                                                         │ │
│  │  5. Safety Checks                                       │ │
│  │     ├─ Exact repeat detection (same tool calls)         │ │
│  │     ├─ Semantic repeat detection (80% similarity)       │ │
│  │     ├─ Stagnation detection (5 rounds no progress)      │ │
│  │     ├─ File modification rate limit (10 per file)       │ │
│  │     └─ Loop limit warning (3 rounds before max)         │ │
│  │                                                         │ │
│  │  6. Nudge (if no tools called)                          │ │
│  │     ├─ Dynamic nudge count (2-10 based on complexity)   │ │
│  │     ├─ Include original goal + files modified            │ │
│  │     ├─ Tool routing suggestion                          │ │
│  │     └─ After 2 nudges: suggest model switch             │ │
│  │                                                         │ │
│  └─── Next iteration ─────────────────────────────────────┘ │
│                                                              │
│  Auto-continue: +20 rounds (max 3 times) when limit reached  │
└──────────────────────────────────────────────────────────────┘
```

### Agent Roles (10 Specialized Agents)

| Agent | Icon | Capabilities | Best For |
|-------|------|-------------|----------|
| Universal Assistant | ⚡ | All tools, all intents | General coding tasks |
| Frontend Architect | 🌐 | read/write/edit, search | React/Vue/Svelte/Angular |
| Backend Architect | ⚙️ | read/write/edit, search, execute | Go/Node/Python/Java |
| UI Designer | 🎨 | read/write/edit | Design systems, CSS, layouts |
| DevOps Engineer | 🚀 | read/write/edit, execute | Docker, K8s, CI/CD, deploy |
| Performance Expert | 📊 | read/write/edit, search, execute | Optimization, profiling |
| API Test Engineer | 🧪 | read/write/edit, execute | Testing, mocking, coverage |
| Compliance Checker | 🛡️ | read, search | Security audit, code review |
| Product Manager | 📋 | read/write/edit | Requirements, PRD, planning |
| AI Integration Engineer | 🤖 | read/write/edit | LLM integration, RAG, agents |

### Tool System (21 Built-in Tools)

```
File Operations          Execution & Search        Git & Network
─────────────           ──────────────           ─────────────
read_file               execute_command          get_git_diff
write_file              search_files             git_commit
edit_file               glob_files               git_pull
multi_edit              list_directory           git_push
create_directory        get_diagnostics
delete_file             web_fetch
move_file               http_request

Workflow & Meta
──────────────
todo_write              skill (execute skills)
ask_user                sub_agent (parallel tasks)
```

### Context Engineering

StarCore uses a sophisticated context management system:

1. **Stable Prefix** (cacheable across requests):
   - Git context (branch, recent commits, diff stats)
   - Code structure analysis (functions, types, imports)
   - Project rules (.starcorerules)
   - Project structure tree
   - Knowledge base entries
   - RAG semantic search results

2. **Dynamic_suffix** (varies per request):
   - Context files (user-selected)
   - Active file content
   - Selected code

3. **Smart Compression**:
   - Token estimation (provider-specific CJK/ASCII ratios)
   - AI-powered summarization when context exceeds 80% of window
   - Message pruning (keep system prefix + suffix + recent 60 messages)
   - Summary persistence to SQLite

4. **Deduplication**:
   - Path normalization + content hash
   - Containment detection (remove files fully contained in others)

### Safety Mechanisms

| Mechanism | Description |
|-----------|-------------|
| **Circuit Breaker** | Opens after 10 consecutive failures, auto-recovers after 60s |
| **Repetition Detection** | 4-layer: exact line, sentence, prefix (5 chars), 8-gram sliding window |
| **Stagnation Detection** | Alerts after 5 rounds without progress |
| **Anti-Drift** | Re-injects original goal every 10 rounds |
| **File Rate Limit** | Warns at 5 modifications, blocks at 10 per file |
| **Tool Error Classification** | 5 categories: retryable, needs LLM, fatal, permission, syntax |
| **Consecutive Failure Tracking** | Per-tool failure counting with strategy change prompt |
| **Sandbox** | Path traversal detection, command validation, SSRF protection |

---

## Tech Stack

| Layer | Technology | Purpose |
|-------|-----------|---------|
| **Backend** | Go 1.23 + Wails v2 v2.12.0 | Desktop app framework |
| **Frontend** | Svelte 5 (runes) + Tailwind CSS v4 | UI framework |
| **Editor** | CodeMirror 6 | Code editing with syntax highlighting |
| **Terminal** | Xterm.js | Integrated terminal |
| **AI** | OpenAI / Anthropic / DeepSeek / Ollama | LLM providers |
| **Database** | SQLite (mattn/go-sqlite3) | Conversations, knowledge, tokens |
| **LSP** | gopls / typescript-language-server / pyright | Code intelligence |
| **Build** | esbuild (via Vite) + Wails | Bundling and native compilation |

---

## Quick Start

### Prerequisites
- Go 1.23+
- Node.js 18+
- Wails CLI: `go install github.com/wailsapp/wails/v2/cmd/wails@latest`

### Development
```bash
git clone https://github.com/vpertj/StarCore.git
cd StarCore
wails dev
```

### Build
```bash
# Windows
wails build -platform windows -arch amd64

# macOS
wails build -platform darwin -arch arm64

# Linux
wails build -platform linux -arch amd64
```

Or use Makefile:
```bash
make build-windows   # Build Windows exe
make test            # Run tests
make clean           # Clean artifacts
```

---

## Configuration

### AI Providers

| Provider | Default Endpoint | API Key |
|----------|-----------------|---------|
| OpenAI | `https://api.openai.com/v1` | [Get Key](https://platform.openai.com) |
| Anthropic | `https://api.anthropic.com/v1` | [Get Key](https://console.anthropic.com) |
| DeepSeek | `https://api.deepseek.com/v1` | [Get Key](https://platform.deepseek.com) |
| Ollama | `http://localhost:11434` | Not needed (local) |

**Free tier**: Install [Ollama](https://ollama.com), run `ollama pull qwen2.5-coder:7b`, add as provider in StarCore.

### Project Rules

Create `.starcorerules` in project root (also supports `.cursorrules` and `CLAUDE.md`):

```markdown
Always reply in Chinese
Use vitest for testing
Don't introduce new third-party dependencies
Follow the existing code style
```

---

## Skills System

24+ built-in skills organized by category:

| Category | Skills |
|----------|--------|
| **Code** | Generate tests, Code review, Refactor, Debug analysis, Security check, Performance analysis, Error handling |
| **Project** | Project init, Dependency audit, Generate README, Log analysis, Shell script |
| **Git** | PR review, Commit message |
| **Database** | SQL optimization, Migration scripts, Data modeling, API design |

Trigger in chat with `/skill-name` or let the agent auto-invoke.

---

## Project Structure

```
StarCore/
├── app.go                    # Wails app setup + bindings
├── main.go                   # Entry point
├── internal/
│   ├── agent/                # Agent system
│   │   ├── tools/            # 21 tool implementations
│   │   │   ├── builtins.go   # Tool registry
│   │   │   ├── sub_agent.go  # Parallel sub-agent execution
│   │   │   └── loop_state.go # Cross-iteration state
│   │   ├── tool_router.go    # Intent-based tool suggestions
│   │   └── intent.go         # 10-type intent classifier
│   ├── ai/
│   │   ├── service.go        # Agent loop + streaming (1800+ lines)
│   │   ├── truncate.go       # Smart result truncation
│   │   └── task_router.go    # Task complexity evaluation
│   ├── context/
│   │   ├── builder.go        # Context message construction
│   │   ├── dedup.go          # File deduplication (3-layer)
│   │   └── auto_suggest.go   # Auto context file recommendation
│   ├── provider/             # LLM provider implementations
│   ├── memory/               # SQLite persistence
│   ├── skill/                # Skills system
│   ├── lsp/                  # Language server protocol
│   ├── mcp/                  # Model Context Protocol
│   ├── terminal/             # PTY management
│   ├── git/                  # Git operations
│   ├── files/                # File operations + search
│   ├── watcher/              # File system watcher
│   └── sandbox/              # Security sandbox
├── frontend/
│   └── src/
│       ├── components/       # Svelte 5 components
│       └── stores/           # State management
├── .github/workflows/        # CI/CD (build + release)
├── build/                    # Build resources (icons, NSIS)
├── Makefile                  # Build automation
├── wails.json                # Wails configuration
└── README.md                 # This file
```

---

## License

MIT
