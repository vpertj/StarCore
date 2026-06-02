# StarCore IDE — Agent Guidelines

Native desktop IDE. Go 1.23 + Wails v2 backend, Svelte 5 + CodeMirror 6 + Xterm.js + Tailwind CSS v4 frontend.

## Build & Test

```bash
wails dev              # dev mode (hot reload)
wails build            # production binary
make test              # go test ./internal/... -v
go test ./... -cover   # coverage
go test ./internal/context -v -run TestBuildContextMessage
```

Some tests are **skipped** (`t.Skip`) due to platform quirks:
- `TestSearchFiles` — requires `chdir`; test manually with `wails dev`
- `TestGitStageAndCommit` — Windows cmd.exe `%%` escaping conflicts with git `--format`

## Architecture

**Entrypoint**: `main.go` embeds `frontend/dist` via `//go:embed`, calls `NewApp()`.
**App wiring**: `app.go` instantiates all services, binds `*App` to Wails frontend.
**Frontend-backend bridge**: `frontend/wailsjs/go/main/App.js` (auto-generated bindings). Backend emits events via `wailsRuntime.EventsEmit`; frontend listens via `EventsOn`.
**Events**: `app:first-run`, `skill:stream:*`, `terminal:*`, etc.

**Internal packages** (`internal/`):
| Package | Purpose |
|---------|---------|
| `provider/` | LLM providers: OpenAI, Anthropic, Ollama (all implement `Provider` interface). Error diagnosis in `diagnosis.go`. |
| `agent/` | Agent registry + tool system. 15 tools in `tools/` (read/write/edit_file, execute_command, glob/search_files, git ops, web_fetch, http_request, skill_tool, sub_agent). |
| `ai/` | Agent loop service (`maxAgentLoops=100`, `maxToolResultChars=8000`). |
| `context/` | AI context builder — attaches project structure, context files, active file, selected code. Has compression. |
| `memory/` | SQLite-backed via `mattn/go-sqlite3`. Conversations, knowledge base, token usage. |
| `skill/` | Skill system: built-in + external (loaded from config dir). |
| `lsp/` | gopls, typescript-language-server, pyright. |
| `mcp/` | Model Context Protocol server management. |
| `terminal/` | PTY management via `conpty` (Windows). |
| `watcher/` | File system watcher. |

## Key Go Dependencies

- `github.com/wailsapp/wails/v2 v2.12.0`
- `github.com/mattn/go-sqlite3 v1.14.44` (SQLite + WAL mode + busy timeout 5000)
- `github.com/UserExistsError/conpty v0.1.4` (Windows PTY)

## Frontend

- **Svelte 5** with runes (`$state`, `$derived`) in components + traditional `svelte/store` (writable/derived/get) in stores.
- **Tailwind CSS v4**: `@import "tailwindcss"` in `style.css`. Vite plugin: `@tailwindcss/vite`. CSS custom properties for theming (`--bg-primary`, `--accent`, etc.).
- **CodeMirror 6** languages: go, js/ts, json, html, css, md, python, rust, java, cpp, php, sql, xml, yaml.
- **No TypeScript** — JSDoc for type hints in JS files.
- **State management**: 20 stores in `stores/` (ui, app, ai, provider, agent, skill, git, memory, theme, etc.). UI panels persist size/visibility to localStorage.
- **`svelte.config.js`** sets `componentApi: 4` for compatibility. `package.json` has `overrides` for `@sveltejs/vite-plugin-svelte ^4.0.0-next.6`.
- Window frameless, 1600×960 default (min 800×600), dark bg `#11111b`.

## Style

- **Go**: tab indentation, group imports (stdlib / external / internal), `go fmt`.
- **JS/Svelte/CSS**: 2-space indentation, LF line endings, UTF-8, trailing newline, trim trailing whitespace (`.editorconfig`).

## Agent Tools (what the AI agent uses)

15 tools in `internal/agent/tools/`. Notable ones beyond basic CRUD:
- `skill_tool.go` — execute a skill mid-conversation
- `sub_agent.go` — spawn sub-agent for parallel tasks
- `web_fetch.go` — fetch URLs
- `http_request.go` — arbitrary HTTP requests
- `get_diagnostics.go` — LSP diagnostics for current file
- `get_git_diff.go`, `git_commit.go` — git integration

## Misc

- No CI/CD, no GitHub Actions, no `.github/`. Build pipeline is `Makefile`.
- Project config dir: `os.UserConfigDir()/StarCore/` (provider configs, skills, SQLite db, custom models).
- `.trae/` and `.codeartsdoer/` are artifacts from other AI coding tools — not project config.
- `wails generate` re-generates `frontend/wailsjs/` after adding new Go methods.
