package debug

import (
	"fmt"
	"net"
	"net/rpc"
	"net/rpc/jsonrpc"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"
)

// EmitFunc is called to emit debug events to the frontend.
type EmitFunc func(event string, data interface{})

// Breakpoint represents a debug breakpoint.
type Breakpoint struct {
	ID        int    `json:"id"`
	File      string `json:"file"`
	Line      int    `json:"line"`
	Function  string `json:"function,omitempty"`
	Enabled   bool   `json:"enabled"`
	HitCount  int    `json:"hitCount"`
	Condition string `json:"condition,omitempty"`
}

// SessionState tracks the current debug state.
type SessionState struct {
	Status     string       `json:"status"` // running, stopped, exited
	Reason     string       `json:"reason"` // breakpoint, step, etc.
	File       string       `json:"file"`
	Line       int          `json:"line"`
	Expr       string       `json:"expr,omitempty"`
	Goroutines []Goroutine  `json:"goroutines"`
	Stack      []StackFrame `json:"stack"`
	Variables  []Variable   `json:"variables"`
}

// StackFrame represents a frame in the call stack.
type StackFrame struct {
	ID       int    `json:"id"`
	Function string `json:"function"`
	File     string `json:"file"`
	Line     int    `json:"line"`
	Package  string `json:"package"`
}

// Variable represents a debug variable.
type Variable struct {
	Name     string     `json:"name"`
	Value    string     `json:"value"`
	Type     string     `json:"type"`
	Children []Variable `json:"children,omitempty"`
}

// Goroutine represents a goroutine.
type Goroutine struct {
	ID    int    `json:"id"`
	Stack string `json:"stack"`
	State string `json:"state"`
}

// DebugSession represents an active debugging session.
type DebugSession struct {
	ID          string
	ProgramPath string
	Port        int
	Cmd         *exec.Cmd
	Client      *rpc.Client
	Breakpoints []Breakpoint
	State       SessionState
	mu          sync.Mutex
}

// Manager manages debug sessions.
type Manager struct {
	mu       sync.RWMutex
	sessions map[string]*DebugSession
	nextPort int
	emit     EmitFunc
}

// NewManager creates a new debug manager.
func NewManager(emit EmitFunc) *Manager {
	return &Manager{
		sessions: make(map[string]*DebugSession),
		nextPort: 38697,
		emit:     emit,
	}
}

// findAvailablePort finds an available port starting from the given port.
func (m *Manager) findAvailablePort() (int, error) {
	for i := 0; i < 100; i++ {
		port := m.nextPort
		m.nextPort++
		if m.nextPort > 65535 {
			m.nextPort = 38697
		}

		addr := fmt.Sprintf("127.0.0.1:%d", port)
		ln, err := net.Listen("tcp", addr)
		if err == nil {
			ln.Close()
			return port, nil
		}
	}
	return 0, fmt.Errorf("no available port found")
}

// isDlvInstalled checks if dlv is available on the system.
func isDlvInstalled() bool {
	path, err := exec.LookPath("dlv")
	if err != nil {
		return false
	}
	_, err = os.Stat(path)
	return err == nil
}

// Start starts a new debug session for the given program.
func (m *Manager) Start(programPath string, args []string) (*DebugSession, error) {
	if !isDlvInstalled() {
		return nil, fmt.Errorf("dlv (Delve debugger) is not installed or not in PATH; install with: go install github.com/go-delve/delve/cmd/dlv@latest")
	}

	absPath, err := filepath.Abs(programPath)
	if err != nil {
		return nil, fmt.Errorf("invalid program path: %w", err)
	}

	if _, err := os.Stat(absPath); err != nil {
		return nil, fmt.Errorf("program not found: %s", absPath)
	}

	port, err := m.findAvailablePort()
	if err != nil {
		return nil, err
	}

	listenAddr := fmt.Sprintf("127.0.0.1:%d", port)

	dlvArgs := []string{"debug", "--headless", "--api-version=2", "--listen=" + listenAddr}
	dlvArgs = append(dlvArgs, absPath)
	if len(args) > 0 {
		dlvArgs = append(dlvArgs, "--")
		dlvArgs = append(dlvArgs, args...)
	}

	cmd := exec.Command("dlv", dlvArgs...)
	cmd.Dir = filepath.Dir(absPath)

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("failed to start dlv: %w", err)
	}

	// Wait for dlv to start listening
	var client *rpc.Client
	for i := 0; i < 50; i++ {
		time.Sleep(100 * time.Millisecond)
		conn, err := net.DialTimeout("tcp", listenAddr, 2*time.Second)
		if err == nil {
			client = jsonrpc.NewClient(conn)
			break
		}
		if cmd.Process != nil && !isProcessRunning(cmd.Process.Pid) {
			break
		}
	}

	if client == nil {
		cmd.Process.Kill()
		cmd.Wait()
		return nil, fmt.Errorf("failed to connect to dlv at %s", listenAddr)
	}

	sessionID := fmt.Sprintf("debug_%d_%d", time.Now().UnixMilli(), port)

	session := &DebugSession{
		ID:          sessionID,
		ProgramPath: absPath,
		Port:        port,
		Cmd:         cmd,
		Client:      client,
		Breakpoints: make([]Breakpoint, 0),
		State:       SessionState{Status: "running"},
	}

	m.mu.Lock()
	m.sessions[sessionID] = session
	m.mu.Unlock()

	// Wait for initial stop
	go m.waitForStop(sessionID)

	m.emit("debug:session-started", map[string]interface{}{
		"sessionId": sessionID,
		"program":   absPath,
		"port":      port,
	})

	return session, nil
}

// isProcessRunning checks if a process is still running.
func isProcessRunning(pid int) bool {
	p, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	err = p.Signal(syscall.Signal(0))
	return err == nil
}

// waitForStop waits for the debugger to stop and emits events.
func (m *Manager) waitForStop(sessionID string) {
	session := m.Get(sessionID)
	if session == nil {
		return
	}

	// Poll for state changes
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	for range ticker.C {
		state, err := m.GetState(sessionID)
		if err != nil {
			continue
		}

		if state.Status == "stopped" {
			m.emit("debug:state-changed", map[string]interface{}{
				"sessionId": sessionID,
				"state":     state,
			})

			if state.Reason == "breakpoint" {
				m.emit("debug:breakpoint-hit", map[string]interface{}{
					"sessionId": sessionID,
					"state":     state,
				})
			}
			break
		}

		if state.Status == "exited" {
			m.emit("debug:session-ended", map[string]interface{}{
				"sessionId": sessionID,
				"reason":    "exited",
			})
			return
		}
	}
}

// Get returns a session by ID.
func (m *Manager) Get(sessionID string) *DebugSession {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.sessions[sessionID]
}

// Stop stops a debug session.
func (m *Manager) Stop(sessionID string) error {
	session := m.Get(sessionID)
	if session == nil {
		return fmt.Errorf("session not found: %s", sessionID)
	}

	session.mu.Lock()
	defer session.mu.Unlock()

	if session.Client != nil {
		session.Client.Close()
	}

	if session.Cmd != nil && session.Cmd.Process != nil {
		session.Cmd.Process.Kill()
		session.Cmd.Wait()
	}

	m.mu.Lock()
	delete(m.sessions, sessionID)
	m.mu.Unlock()

	m.emit("debug:session-ended", map[string]interface{}{
		"sessionId": sessionID,
		"reason":    "stopped",
	})

	return nil
}

// List returns all active sessions.
func (m *Manager) List() []map[string]interface{} {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]map[string]interface{}, 0, len(m.sessions))
	for _, s := range m.sessions {
		result = append(result, map[string]interface{}{
			"id":          s.ID,
			"program":     s.ProgramPath,
			"port":        s.Port,
			"status":      s.State.Status,
			"breakpoints": len(s.Breakpoints),
		})
	}
	return result
}

// AddBreakpoint sets a breakpoint at the specified file and line, with optional condition.
func (m *Manager) AddBreakpoint(sessionID, file string, line int, condition string) (*Breakpoint, error) {
	session := m.Get(sessionID)
	if session == nil {
		return nil, fmt.Errorf("session not found: %s", sessionID)
	}

	args := map[string]interface{}{
		"file": file,
		"line": line,
	}
	if condition != "" {
		args["condition"] = condition
	}

	var result Breakpoint
	err := session.Client.Call("Debugger.CreateBreakpoint", args, &result)
	if err != nil {
		return nil, fmt.Errorf("failed to create breakpoint: %w", err)
	}

	if condition != "" {
		result.Condition = condition
	}

	session.mu.Lock()
	session.Breakpoints = append(session.Breakpoints, result)
	session.mu.Unlock()

	return &result, nil
}

// AddBreakpointByFunc sets a breakpoint at the specified function.
func (m *Manager) AddBreakpointByFunc(sessionID, function string) (*Breakpoint, error) {
	session := m.Get(sessionID)
	if session == nil {
		return nil, fmt.Errorf("session not found: %s", sessionID)
	}

	var result Breakpoint
	err := session.Client.Call("Debugger.CreateBreakpoint", map[string]interface{}{
		"functionName": function,
	}, &result)
	if err != nil {
		return nil, fmt.Errorf("failed to create breakpoint: %w", err)
	}

	session.mu.Lock()
	session.Breakpoints = append(session.Breakpoints, result)
	session.mu.Unlock()

	return &result, nil
}

// RemoveBreakpoint removes a breakpoint by ID.
func (m *Manager) RemoveBreakpoint(sessionID string, bpID int) error {
	session := m.Get(sessionID)
	if session == nil {
		return fmt.Errorf("session not found: %s", sessionID)
	}

	err := session.Client.Call("Debugger.ClearBreakpoint", bpID, nil)
	if err != nil {
		return fmt.Errorf("failed to remove breakpoint: %w", err)
	}

	session.mu.Lock()
	newBps := make([]Breakpoint, 0, len(session.Breakpoints))
	for _, bp := range session.Breakpoints {
		if bp.ID != bpID {
			newBps = append(newBps, bp)
		}
	}
	session.Breakpoints = newBps
	session.mu.Unlock()

	return nil
}

// ListBreakpoints returns all breakpoints for a session.
func (m *Manager) ListBreakpoints(sessionID string) ([]Breakpoint, error) {
	session := m.Get(sessionID)
	if session == nil {
		return nil, fmt.Errorf("session not found: %s", sessionID)
	}

	var bps []Breakpoint
	err := session.Client.Call("Debugger.ListBreakpoints", false, &bps)
	if err != nil {
		return nil, fmt.Errorf("failed to list breakpoints: %w", err)
	}

	return bps, nil
}

// Continue resumes program execution.
func (m *Manager) Continue(sessionID string) error {
	session := m.Get(sessionID)
	if session == nil {
		return fmt.Errorf("session not found: %s", sessionID)
	}

	var state SessionState
	err := session.Client.Call("Debugger.Command", map[string]interface{}{
		"name":     "continue",
		"threadId": 0,
	}, &state)
	if err != nil {
		return fmt.Errorf("failed to continue: %w", err)
	}

	m.emit("debug:state-changed", map[string]interface{}{
		"sessionId": sessionID,
		"state":     state,
	})

	go m.waitForStop(sessionID)

	return nil
}

// StepOver steps over the current line.
func (m *Manager) StepOver(sessionID string) error {
	session := m.Get(sessionID)
	if session == nil {
		return fmt.Errorf("session not found: %s", sessionID)
	}

	var state SessionState
	err := session.Client.Call("Debugger.Command", map[string]interface{}{
		"name":     "next",
		"threadId": 0,
	}, &state)
	if err != nil {
		return fmt.Errorf("failed to step over: %w", err)
	}

	m.emit("debug:state-changed", map[string]interface{}{
		"sessionId": sessionID,
		"state":     state,
	})

	go m.waitForStop(sessionID)

	return nil
}

// StepIn steps into the current function.
func (m *Manager) StepIn(sessionID string) error {
	session := m.Get(sessionID)
	if session == nil {
		return fmt.Errorf("session not found: %s", sessionID)
	}

	var state SessionState
	err := session.Client.Call("Debugger.Command", map[string]interface{}{
		"name":     "step",
		"threadId": 0,
	}, &state)
	if err != nil {
		return fmt.Errorf("failed to step in: %w", err)
	}

	m.emit("debug:state-changed", map[string]interface{}{
		"sessionId": sessionID,
		"state":     state,
	})

	go m.waitForStop(sessionID)

	return nil
}

// StepOut steps out of the current function.
func (m *Manager) StepOut(sessionID string) error {
	session := m.Get(sessionID)
	if session == nil {
		return fmt.Errorf("session not found: %s", sessionID)
	}

	var state SessionState
	err := session.Client.Call("Debugger.Command", map[string]interface{}{
		"name":     "stepOut",
		"threadId": 0,
	}, &state)
	if err != nil {
		return fmt.Errorf("failed to step out: %w", err)
	}

	m.emit("debug:state-changed", map[string]interface{}{
		"sessionId": sessionID,
		"state":     state,
	})

	go m.waitForStop(sessionID)

	return nil
}

// GetState returns the current debug state.
func (m *Manager) GetState(sessionID string) (*SessionState, error) {
	session := m.Get(sessionID)
	if session == nil {
		return nil, fmt.Errorf("session not found: %s", sessionID)
	}

	var apiState struct {
		Running bool `json:"Running"`
		Current *struct {
			GoroutineID int `json:"goroutineID"`
			Location    struct {
				File     string `json:"file"`
				Line     int    `json:"line"`
				Function struct {
					Name string `json:"name"`
				} `json:"function"`
			} `json:"location"`
			Stack []struct {
				Location struct {
					File     string `json:"file"`
					Line     int    `json:"line"`
					Function struct {
						Name        string `json:"name"`
						PackageName string `json:"packageName"`
					} `json:"function"`
				} `json:"location"`
			} `json:"stack"`
		} `json:"CurrentThread"`
		Exited   bool `json:"exited"`
		ExitCode int  `json:"exitCode"`
	}

	err := session.Client.Call("Debugger.State", 0, &apiState)
	if err != nil {
		// If we can't get state, dlv may have exited
		if !isProcessRunning(session.Cmd.Process.Pid) {
			return &SessionState{Status: "exited"}, nil
		}
		return nil, fmt.Errorf("failed to get state: %w", err)
	}

	state := &SessionState{
		Goroutines: make([]Goroutine, 0),
		Stack:      make([]StackFrame, 0),
		Variables:  make([]Variable, 0),
	}

	if apiState.Exited {
		state.Status = "exited"
		return state, nil
	}

	if apiState.Running {
		state.Status = "running"
		return state, nil
	}

	state.Status = "stopped"

	if apiState.Current != nil {
		state.File = apiState.Current.Location.File
		state.Line = apiState.Current.Location.Line

		// Parse stack frames
		for i, frame := range apiState.Current.Stack {
			state.Stack = append(state.Stack, StackFrame{
				ID:       i,
				Function: frame.Location.Function.Name,
				File:     frame.Location.File,
				Line:     frame.Location.Line,
				Package:  frame.Location.Function.PackageName,
			})
		}

		// Get goroutines
		var goroutines []struct {
			ID    int    `json:"id"`
			State string `json:"state"`
			Stack []struct {
				Location struct {
					Function struct {
						Name string `json:"name"`
					} `json:"function"`
				} `json:"location"`
			} `json:"stack"`
		}
		err = session.Client.Call("Debugger.ListGoroutines", 0, &goroutines)
		if err == nil {
			for _, g := range goroutines {
				stackStr := ""
				if len(g.Stack) > 0 {
					stackStr = g.Stack[0].Location.Function.Name
				}
				state.Goroutines = append(state.Goroutines, Goroutine{
					ID:    g.ID,
					State: g.State,
					Stack: stackStr,
				})
			}
		}

		// Get local variables if stopped at a location
		if len(state.Stack) > 0 {
			var locals []struct {
				Name  string `json:"name"`
				Value string `json:"value"`
				Type  string `json:"type"`
			}
			err = session.Client.Call("Debugger.ListLocalVariables", map[string]interface{}{
				"scope": map[string]interface{}{
					"goroutineID": apiState.Current.GoroutineID,
					"frame":       0,
				},
			}, &locals)
			if err == nil {
				for _, v := range locals {
					state.Variables = append(state.Variables, Variable{
						Name:  v.Name,
						Value: v.Value,
						Type:  v.Type,
					})
				}
			}

			// Also get function arguments
			var args []struct {
				Name  string `json:"name"`
				Value string `json:"value"`
				Type  string `json:"type"`
			}
			err = session.Client.Call("Debugger.ListFunctionArgs", map[string]interface{}{
				"scope": map[string]interface{}{
					"goroutineID": apiState.Current.GoroutineID,
					"frame":       0,
				},
			}, &args)
			if err == nil {
				for _, v := range args {
					state.Variables = append(state.Variables, Variable{
						Name:  v.Name,
						Value: v.Value,
						Type:  v.Type,
					})
				}
			}
		}
	}

	return state, nil
}

// GetVariable evaluates an expression and returns its value.
func (m *Manager) GetVariable(sessionID string, frameID int, expr string) (*Variable, error) {
	session := m.Get(sessionID)
	if session == nil {
		return nil, fmt.Errorf("session not found: %s", sessionID)
	}

	var result struct {
		Name     string `json:"name"`
		Value    string `json:"value"`
		Type     string `json:"type"`
		Children []struct {
			Name  string `json:"name"`
			Value string `json:"value"`
			Type  string `json:"type"`
		} `json:"children"`
	}

	err := session.Client.Call("Debugger.EvalVariable", map[string]interface{}{
		"expr": expr,
		"scope": map[string]interface{}{
			"goroutineID": -1,
			"frame":       frameID,
		},
	}, &result)
	if err != nil {
		return nil, fmt.Errorf("failed to evaluate expression: %w", err)
	}

	variable := &Variable{
		Name:     result.Name,
		Value:    result.Value,
		Type:     result.Type,
		Children: make([]Variable, 0),
	}

	for _, child := range result.Children {
		variable.Children = append(variable.Children, Variable{
			Name:  child.Name,
			Value: child.Value,
			Type:  child.Type,
		})
	}

	return variable, nil
}

// Restart restarts the debug session.
func (m *Manager) Restart(sessionID string) error {
	session := m.Get(sessionID)
	if session == nil {
		return fmt.Errorf("session not found: %s", sessionID)
	}

	var state SessionState
	err := session.Client.Call("Debugger.Restart", false, &state)
	if err != nil {
		return fmt.Errorf("failed to restart: %w", err)
	}

	m.emit("debug:state-changed", map[string]interface{}{
		"sessionId": sessionID,
		"state":     state,
	})

	return nil
}

// Detach detaches from the debug session without killing the program.
func (m *Manager) Detach(sessionID string) error {
	session := m.Get(sessionID)
	if session == nil {
		return fmt.Errorf("session not found: %s", sessionID)
	}

	session.mu.Lock()
	defer session.mu.Unlock()

	if session.Client != nil {
		session.Client.Close()
	}

	m.mu.Lock()
	delete(m.sessions, sessionID)
	m.mu.Unlock()

	m.emit("debug:session-ended", map[string]interface{}{
		"sessionId": sessionID,
		"reason":    "detached",
	})

	return nil
}

// StopAll stops all active debug sessions.
func (m *Manager) StopAll() {
	m.mu.RLock()
	ids := make([]string, 0, len(m.sessions))
	for id := range m.sessions {
		ids = append(ids, id)
	}
	m.mu.RUnlock()

	for _, id := range ids {
		m.Stop(id)
	}
}

// ConsoleResult represents the result of a debug console command.
type ConsoleResult struct {
	Output string `json:"output"`
	Error  string `json:"error,omitempty"`
}

// ConsoleExecute executes a debug console command (expression evaluation, dlv command).
func (m *Manager) ConsoleExecute(sessionID, expr string) (*ConsoleResult, error) {
	session := m.Get(sessionID)
	if session == nil {
		return nil, fmt.Errorf("session not found: %s", sessionID)
	}

	// Try to evaluate as expression first
	var evalResult struct {
		Name     string `json:"name"`
		Value    string `json:"value"`
		Type     string `json:"type"`
		Children []struct {
			Name  string `json:"name"`
			Value string `json:"value"`
			Type  string `json:"type"`
		} `json:"children"`
	}

	err := session.Client.Call("Debugger.EvalVariable", map[string]interface{}{
		"expr": expr,
		"scope": map[string]interface{}{
			"goroutineID": -1,
			"frame":       0,
		},
	}, &evalResult)

	if err == nil {
		output := fmt.Sprintf("%s = %s (%s)", evalResult.Name, evalResult.Value, evalResult.Type)
		for _, child := range evalResult.Children {
			output += fmt.Sprintf("\n  .%s = %s (%s)", child.Name, child.Value, child.Type)
		}
		return &ConsoleResult{Output: output}, nil
	}

	// If expression eval failed, try as a raw dlv command
	var cmdResult string
	err = session.Client.Call("Debugger.Command", map[string]interface{}{
		"name": expr,
	}, &cmdResult)
	if err == nil {
		return &ConsoleResult{Output: cmdResult}, nil
	}

	return &ConsoleResult{Error: fmt.Sprintf("Error: %v", err)}, nil
}

// GetVersion returns the dlv version string.
func GetVersion() string {
	out, err := exec.Command("dlv", "version").CombinedOutput()
	if err != nil {
		return "unknown"
	}
	return strings.TrimSpace(string(out))
}

// CheckDlvInstalled checks if dlv is available and returns its version.
func CheckDlvInstalled() (bool, string) {
	if !isDlvInstalled() {
		return false, ""
	}
	return true, GetVersion()
}
