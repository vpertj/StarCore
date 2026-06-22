# Multi-Project Support Implementation Plan

> **For agentic workers:** REQUIRED: Use $subagent-driven-development (if subagents available) or $executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Allow the app to track multiple open projects simultaneously, with per-project state for file watchers, terminals, and sandbox configs.

**Architecture:** Extend existing single-project structures to maps keyed by project path. Maintain backward compatibility by treating the first opened project as active. Frontend stores will track open projects and filter terminals accordingly.

**Tech Stack:** Go 1.23, Wails v2, Svelte 5 stores, JavaScript.

---

## File Structure

- Modify: `internal/terminal/manager.go` — add ProjectPath to Session, new methods
- Modify: `app.go` — per-project watchers, sandbox, active project, new methods
- Modify: `frontend/src/stores/app.js` — openProjects store, switch/close functions
- Modify: `frontend/src/stores/terminal.js` — terminal tabs per project, filter by active
- Create: `internal/terminal/manager_test.go` — unit tests for new methods
- Create: `app_test.go` — unit tests for new App methods (optional)

---

## Task 1: Extend terminal manager with project awareness

**Files:**
- Modify: `internal/terminal/manager.go`
- Create: `internal/terminal/manager_test.go`

- [ ] **Step 1: Add ProjectPath field to Session struct**

```go
type Session struct {
    ID          string
    Pty         *conpty.ConPty
    Done        chan struct{}
    Created     time.Time
    connected   bool
    mu          sync.Mutex
    CWD         string
    ProjectPath string
    buffer      []string
}
```

- [ ] **Step 2: Update NewManager to accept projectPath parameter**

Change `New(cwd string)` signature to `New(cwd string, projectPath string)`. Store projectPath in session.

- [ ] **Step 3: Add ListByProject method**

```go
func (m *Manager) ListByProject(projectPath string) []map[string]interface{} {
    m.mu.Lock()
    defer m.mu.Unlock()
    result := make([]map[string]interface{}, 0)
    for _, sess := range m.sessions {
        if sess.ProjectPath == projectPath {
            result = append(result, map[string]interface{}{
                "id":      sess.ID,
                "created": sess.Created.Format(time.RFC3339),
            })
        }
    }
    return result
}
```

- [ ] **Step 4: Add KillByProject method**

```go
func (m *Manager) KillByProject(projectPath string) error {
    m.mu.Lock()
    toKill := make([]string, 0)
    for id, sess := range m.sessions {
        if sess.ProjectPath == projectPath {
            toKill = append(toKill, id)
        }
    }
    m.mu.Unlock()
    for _, id := range toKill {
        if err := m.Kill(id); err != nil {
            return err
        }
    }
    return nil
}
```

- [ ] **Step 5: Update all callers of New() to pass projectPath**

In `app.go` `NewTerminal` and `StartTerminal` and `SetProjectPath` (where terminals may be created). Need to adjust signatures.

- [ ] **Step 6: Write unit tests for new methods**

Create test file `manager_test.go` that tests ListByProject, KillByProject.

- [ ] **Step 7: Run tests**

Run: `go test ./internal/terminal -v`

---

## Task 2: Extend App struct for per-project state

**Files:**
- Modify: `app.go`

- [ ] **Step 1: Add fields to App struct**

```go
type App struct {
    // existing...
    fileWatchers    map[string]*watcher.Watcher
    sandboxConfigs  map[string]*sandbox.Config
    activeProject   string
    openProjects    []string
}
```

Initialize maps in `NewApp`.

- [ ] **Step 2: Modify SetProjectPath to keep old watchers**

Instead of destroying the single watcher, add to map and start new watcher. Keep old watchers alive.

- [ ] **Step 3: Add SwitchProject method**

```go
func (a *App) SwitchProject(path string) {
    a.activeProject = path
    // Update UI-related services (verify, LSP root, workspace active)
    a.verifySvc.SetProjectDir(path)
    a.lspMgr.SetRootPath(path)
    a.workspaceMgr.SetActive(path)
    // Emit event for frontend
    a.emit("project:switched", map[string]interface{}{"path": path})
}
```

- [ ] **Step 4: Add GetOpenProjects method**

```go
func (a *App) GetOpenProjects() []string {
    return a.openProjects
}
```

- [ ] **Step 5: Add CloseProject method**

Stops watcher, kills terminals for that project, removes from maps.

- [ ] **Step 6: Update startup to restore open projects**

Persist open projects list in session state? For now, store in memory only.

- [ ] **Step 7: Write unit tests for new App methods**

Optional: create app_test.go with mock services.

---

## Task 3: Frontend multi-project state

**Files:**
- Modify: `frontend/src/stores/app.js`

- [ ] **Step 1: Add openProjects store**

```js
export const openProjects = writable(/** @type {string[]} */ ([]))
```

Persist to localStorage.

- [ ] **Step 2: Update openProjectPath to add to openProjects**

When a project is opened, add to openProjects if not already present.

- [ ] **Step 3: Add switchProject function**

```js
export function switchProject(path) {
    currentProject.set(path)
    if (window.backend.SwitchProject) {
        window.backend.SwitchProject(path)
    }
}
```

- [ ] **Step 4: Add closeProject function**

```js
export function closeProject(path) {
    openProjects.update(list => list.filter(p => p !== path))
    if (window.backend.CloseProject) {
        window.backend.CloseProject(path)
    }
    // If closing active project, switch to another open project or null
    if (get(currentProject) === path) {
        const remaining = get(openProjects)
        currentProject.set(remaining.length > 0 ? remaining[0] : null)
    }
}
```

- [ ] **Step 5: Update recentProjects subscription to sync openProjects**

Maybe openProjects is separate.

- [ ] **Step 6: Test UI flow manually**

Open multiple projects, switch between them, close one.

---

## Task 4: Terminal store project filtering

**Files:**
- Modify: `frontend/src/stores/terminal.js`

- [ ] **Step 1: Add projectPath to TerminalTab typedef**

```js
/**
 * @typedef {{ id: string, title: string, status: TerminalStatus, exitCode: number|null, projectPath: string|null }} TerminalTab
 */
```

- [ ] **Step 2: Update createTerminalTab to store projectPath**

```js
const projectPath = get(currentProject) || null
// after creating tab
const tab = { id, title: `Terminal ${terminalCounter++}`, status: 'running', exitCode: null, projectPath }
```

- [ ] **Step 3: Add derived store for filtered terminals**

```js
import { derived } from 'svelte/store'
export const filteredTerminalTabs = derived(
    [terminalTabs, currentProject],
    ([$terminalTabs, $currentProject]) => {
        if (!$currentProject) return $terminalTabs
        return $terminalTabs.filter(t => t.projectPath === $currentProject)
    }
)
```

- [ ] **Step 4: Update UI components to use filteredTerminalTabs**

Need to find where terminal tabs are rendered. Search for terminalTabs usage.

- [ ] **Step 5: Update ensureDefaultTerminal to consider project**

If no terminal for current project, create one.

- [ ] **Step 6: Update restartTerminal to preserve projectPath**

Already uses currentProject.

- [ ] **Step 7: Test terminal switching**

Switch projects, verify terminals filter correctly.

---

## Task 5: Integration and verification

- [ ] **Step 1: Run go build**

Run: `go build ./...`

- [ ] **Step 2: Run frontend build**

Run: `cd frontend && npx vite build`

- [ ] **Step 3: Run all tests**

Run: `go test ./... -count=1`

- [ ] **Step 4: Manual testing**

Start with `wails dev`, open multiple projects, verify terminals per project, file watchers per project, sandbox configs per project.

---

## Commit Strategy

- Commit after each task completion
- Use conventional commits: `feat(terminal): add project-aware methods`, `feat(app): add multi-project state`, etc.

---

## Notes

- Backward compatibility: existing single-project flow works because openProjects will contain one project, activeProject defaults to that.
- Sandbox configs: currently `agentTools.SandboxConfig` is a global variable. Need to make it per-project. Might need to change `sandbox.DefaultConfig(path)` call in SetProjectPath to store per project and set global when switching.
- LSP and workspace managers already support multiple roots; we just need to keep them in sync.

---

*Plan created 2026-06-22*