# StarCore IDE Specification

## 1. Overview
StarCore IDE is a native desktop integrated development environment that combines the power of **Go**, **Wails**, and **Svelte 5** to deliver an AI‑assisted coding experience.  It is positioned as a competitor to the **Trae IDE** (React/Electron) and **Zed IDE** (Rust/GPUI).  The IDE provides a modern, responsive UI, AI code‑completion, project management, and extensibility through plugins.

## 2. Goals
1. **Feature‑complete IDE** – project explorer, code editor, terminal, AI chat, settings, and plugin system.
2. **Cross‑platform binaries** – Windows `.exe` and macOS `.dmg` with a single codebase.
3. **Responsive UI** – works on both desktop (large screens) and laptop/tablet (small screens).
4. **Performance** – start‑up < 500 ms, UI latency < 50 ms, memory footprint < 150 MB.
5. **Extensible** – plugin architecture based on Go modules; plugins can add menus, commands, or UI panels.
6. **AI integration** – a built‑in chat pane that talks to a configurable LLM endpoint.

## 3. Tech Stack
| Layer | Technology | Reason |
|-------|------------|--------|
| **Backend** | **Go 1.22** + **Wails 3** | Strong concurrency, compiled binaries, native webview bridge.
| **Frontend** | **Svelte 5** + **shadcn‑svelte** + **Tailwind 4** | Declarative UI, minimal runtime, easy theming, component library.
| **Editor** | **CodeMirror 6** (Svelte wrapper) | Mature, extensible, supports LSP and syntax highlighting.
| **Terminal** | **Xterm.js 5** | Web‑based terminal that integrates with the OS PTY.
| **Icons** | **Lucide Icons** | Consistent open‑source icon set.
| **State Management** | **Svelte stores** | Simple reactive store pattern aligns with Svelte design.
| **Styling** | **Tailwind CSS** (utility‑first) + **shadcn‑svelte** components | Rapid UI development, themable, responsive.
| **AI Client** | **Fetch API** (TS) – configurable endpoint | Keeps backend agnostic; user can point to any LLM service.
| **Build** | **Vite** (for Svelte) + **Wails** bundler | Fast dev HMR, production bundling to native binary.

## 4. Architecture Overview
```
+-------------------+          +------------------+
|  Go (Wails)       |  <--->   |  Svelte UI (Vite) |
|  - Core services  |          |  - shadcn-svelte |
|  - Plugin loader  |          |  - CodeMirror    |
|  - IPC bridge    |          +------------------+
+-------------------+
       ^   ^
       |   |
   Native OS   WebView (Chromium)
```
* The Go process runs the application logic, loads plugins, and provides an IPC bridge via Wails.
* The Svelte UI runs inside the Chromium webview. UI components communicate with Go through the Wails‑generated `window.backend` object.
* Plugins are Go modules that can register new menus, commands, or UI panels.

## 5. UI Design
- **Layout** – Split‑view with a left **Project Explorer**, centre **Editor**, right **Side Panel** (Terminal, AI chat, or extensions).
- **Responsive breakpoints** – Collapse the explorer into a drawer on < 640 px width.
- **Theming** – Light/Dark mode using Tailwind CSS variables; user can select custom accent colors.
- **Components** – All UI built from **shadcn‑svelte** primitives (Button, Dialog, Tabs, etc.) to ensure consistency with the design system.
- **Editor** – CodeMirror 6 with syntax highlighting, LSP support, and inline AI suggestions.
- **Terminal** – Xterm.js embedded, connected to the OS PTY via the Go backend.
- **AI Chat** – Persistent chat pane; user can send code snippets, receive completions, and insert them directly into the editor.

## 6. Core Features
1. **Project Management** – Open folders, watch file system, display tree, context menu actions (new file, rename, delete).
2. **Code Editing** – Syntax highlighting, auto‑completion, LSP integration, multi‑cursor support.
3. **AI Assistant** – Configurable endpoint (OpenAI, Azure, Ollama); streaming response; insert suggestion.
4. **Integrated Terminal** – Shell access, configurable font & colors.
5. **Settings** – JSON/YAML config file, UI for theme, AI endpoint, key bindings.
6. **Plugin System** – `plugins/` folder; each plugin is a Go module exporting a `Register(*wails.Runtime)` function.
7. **Search** – Global file search using ripgrep (`rg`), UI results list.
8. **Version Control** – Basic Git status view; commit, branch, and push UI (via `git` CLI).

## 7. Non‑functional Requirements
- **Performance**: UI latency < 50 ms for interactions; start‑up time < 500 ms.
- **Memory**: < 150 MB on idle, < 300 MB with a project open.
- **Security**: All external network calls (AI endpoint) must be optional and configurable; no hard‑coded secrets.
- **Accessibility**: Keyboard navigation, ARIA labels, high‑contrast theme.
- **Internationalization**: UI strings stored in JSON files for future i18n.

## 8. Milestones & Phases
| Phase | Scope | Deliverables |
|-------|-------|--------------|
| **Phase 1 – Core** | Project explorer, basic editor, terminal, settings UI. | Working binary that opens a folder, edits files, and runs a terminal. |
| **Phase 2 – AI** | AI chat pane, streaming completions, insert suggestion. | AI integration with configurable endpoint; unit tests for request handling. |
| **Phase 3 – Plugins** | Plugin loader, sample plugin, UI extension points. | Documentation for plugin development, sample "Hello World" plugin. |
| **Phase 4 – Polish** | Theming, responsive layout, performance tuning, CI/CD pipeline. | Dark/light mode toggle, mobile‑friendly drawer, automated tests pass on CI. |
| **Phase 5 – Release** | Build installers for Windows `.exe` and macOS `.dmg`; versioning, changelog. | Publish to GitHub releases, CI creates signed binaries. |

## 9. Development Workflow
1. **Repository** – `main` branch protected; feature branches follow `feat/<name>`.
2. **Testing** – TDD for all new code. Go tests (`go test ./...`) and Svelte tests (`npm test`).
3. **Commit Style** – Conventional Commits (`feat:`, `fix:`, `docs:`). Frequent small commits.
4. **CI** – GitHub Actions run `go test`, `npm test`, lint, and build binaries on push.
5. **Code Review** – Pull request must pass CI and have at least one review before merge.

## 10. Open Questions
- Which LLM provider(s) will be the default? (OpenAI vs Ollama)
- How granular should the plugin API be? (menu only vs full UI injection)
- Do we need built‑in debugging support (DAP) in Phase 2?

---
*Document created to guide the implementation of StarCore IDE using Go, Wails, and Svelte 5.*