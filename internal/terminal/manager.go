package terminal

import (
	"fmt"
	"io"
	"os/exec"
	"runtime"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/UserExistsError/conpty"
)

const readBufSize = 4096

// EmitFunc is called to emit terminal output to the frontend.
type EmitFunc func(event string, data interface{})

// Session represents an active terminal session.
type Session struct {
	ID        string
	Pty       *conpty.ConPty
	Done      chan struct{}
	Created   time.Time
	connected bool
	mu        sync.Mutex
	CWD       string
	buffer    []string
}

// Manager manages terminal sessions and their PTY processes.
type Manager struct {
	mu       sync.Mutex
	sessions map[string]*Session
	idx      int
	emitFn   EmitFunc
	ctxDone  <-chan struct{}
}

// NewManager creates a new terminal manager.
func NewManager(emitFn EmitFunc, ctxDone <-chan struct{}) *Manager {
	return &Manager{
		sessions: make(map[string]*Session),
		emitFn:   emitFn,
		ctxDone:  ctxDone,
	}
}

// New creates a new terminal session.
func (m *Manager) New(cwd string) (string, error) {
	m.mu.Lock()
	m.idx++
	id := fmt.Sprintf("term_%d", m.idx)
	m.mu.Unlock()

	var cmdLine string
	if runtime.GOOS == "windows" {
		shellPath, err := exec.LookPath("powershell.exe")
		if err != nil {
			return "", fmt.Errorf("powershell.exe not found: %w", err)
		}
		cmdLine = shellPath + " -NoLogo"
	} else {
		shellPath, err := exec.LookPath("sh")
		if err != nil {
			return "", fmt.Errorf("sh not found: %w", err)
		}
		cmdLine = shellPath
	}

	opts := []conpty.ConPtyOption{
		conpty.ConPtyDimensions(120, 30),
	}
	if cwd != "" {
		opts = append(opts, conpty.ConPtyWorkDir(cwd))
	}

	cpty, err := conpty.Start(cmdLine, opts...)
	if err != nil {
		return "", fmt.Errorf("conpty start: %w", err)
	}

	session := &Session{
		ID:      id,
		Pty:     cpty,
		Done:    make(chan struct{}),
		Created: time.Now(),
		CWD:     cwd,
	}

	m.mu.Lock()
	m.sessions[id] = session
	m.mu.Unlock()

	go m.readOutput(session)

	return id, nil
}

// readOutput reads PTY output and emits it as events.
func (m *Manager) readOutput(session *Session) {
	cpty := session.Pty
	id := session.ID
	buf := make([]byte, readBufSize)
	var carry []byte

	emit := func(output string) {
		m.emitFn("terminal:output:"+id, output)
	}

	emitExit := func() {
		m.emitFn("terminal:exit:"+id, nil)
	}

	for {
		select {
		case <-m.ctxDone:
			return
		default:
		}
		n, err := cpty.Read(buf)
		if n > 0 {
			var data []byte
			if len(carry) > 0 {
				data = make([]byte, len(carry)+n)
				copy(data, carry)
				copy(data[len(carry):], buf[:n])
				carry = nil
			} else {
				data = buf[:n]
			}

			validEnd := len(data)
			for validEnd > 0 && !utf8.Valid(data[:validEnd]) {
				validEnd--
			}
			if validEnd < len(data) {
				carry = make([]byte, len(data)-validEnd)
				copy(carry, data[validEnd:])
				if len(carry) > 4 {
					carry = nil
				}
			}
			if validEnd > 0 {
				output := string(data[:validEnd])
				session.mu.Lock()
				if session.connected {
					session.mu.Unlock()
					emit(output)
				} else {
					session.buffer = append(session.buffer, output)
					if len(session.buffer) > 200 {
						session.buffer = session.buffer[len(session.buffer)-200:]
					}
					session.mu.Unlock()
				}
			}
		}
		if err != nil {
			if err != io.EOF {
				session.mu.Lock()
				if session.connected {
					session.mu.Unlock()
					emit("\r\n\x1b[90m[进程异常退出]\x1b[0m\r\n")
				} else {
					session.mu.Unlock()
				}
			}
			if len(carry) > 0 {
				emit(string(carry))
			}
			break
		}
	}
	close(session.Done)
	emitExit()
}

// Connect connects the frontend to a buffered session and flushes the buffer.
func (m *Manager) Connect(id string) error {
	m.mu.Lock()
	sess := m.sessions[id]
	if sess == nil || sess.Pty == nil {
		m.mu.Unlock()
		return fmt.Errorf("terminal not found: %s", id)
	}
	m.mu.Unlock()

	sess.mu.Lock()
	defer sess.mu.Unlock()
	if sess.connected {
		return nil
	}
	sess.connected = true
	for _, output := range sess.buffer {
		m.emitFn("terminal:output:"+id, output)
	}
	sess.buffer = nil
	return nil
}

// Write writes data to a session's PTY.
func (m *Manager) Write(id string, data string) error {
	m.mu.Lock()
	sess := m.sessions[id]
	m.mu.Unlock()

	if sess == nil || sess.Pty == nil {
		return fmt.Errorf("terminal not found: %s", id)
	}

	_, err := sess.Pty.Write([]byte(data))
	return err
}

// Resize resizes a session's PTY dimensions.
func (m *Manager) Resize(id string, cols int, rows int) error {
	m.mu.Lock()
	sess := m.sessions[id]
	m.mu.Unlock()

	if sess == nil || sess.Pty == nil {
		return nil
	}
	return sess.Pty.Resize(cols, rows)
}

// Kill closes and removes a terminal session.
func (m *Manager) Kill(id string) error {
	m.mu.Lock()
	sess, ok := m.sessions[id]
	if ok {
		delete(m.sessions, id)
	}
	m.mu.Unlock()

	if !ok || sess == nil || sess.Pty == nil {
		return nil
	}

	return sess.Pty.Close()
}

// List returns all active terminal sessions.
func (m *Manager) List() []map[string]interface{} {
	m.mu.Lock()
	defer m.mu.Unlock()

	result := make([]map[string]interface{}, 0, len(m.sessions))
	for id, sess := range m.sessions {
		result = append(result, map[string]interface{}{
			"id":      id,
			"created": sess.Created.Format(time.RFC3339),
		})
	}
	return result
}
