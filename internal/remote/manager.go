package remote

import (
	"context"
	"fmt"
	"sync"
	"time"
)

type ConnectionType string

const (
	TypeSSH       ConnectionType = "ssh"
	TypeContainer ConnectionType = "container"
)

type ConnectionStatus string

const (
	StatusDisconnected ConnectionStatus = "disconnected"
	StatusConnecting   ConnectionStatus = "connecting"
	StatusConnected    ConnectionStatus = "connected"
	StatusError        ConnectionStatus = "error"
)

type Connection struct {
	ID          string           `json:"id"`
	Name        string           `json:"name"`
	Type        ConnectionType   `json:"type"`
	Host        string           `json:"host"`
	Port        int              `json:"port"`
	User        string           `json:"user"`
	Container   string           `json:"container,omitempty"`
	WorkDir     string           `json:"workDir"`
	Status      ConnectionStatus `json:"status"`
	LastError   string           `json:"lastError,omitempty"`
	ConnectedAt time.Time        `json:"connectedAt,omitempty"`
}

type Manager struct {
	mu          sync.RWMutex
	connections map[string]*Connection
}

func NewManager() *Manager {
	return &Manager{
		connections: make(map[string]*Connection),
	}
}

func (m *Manager) AddConnection(conn Connection) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if conn.ID == "" {
		conn.ID = fmt.Sprintf("%s-%d", conn.Type, time.Now().UnixMilli())
	}
	if _, exists := m.connections[conn.ID]; exists {
		return fmt.Errorf("connection %s already exists", conn.ID)
	}
	conn.Status = StatusDisconnected
	m.connections[conn.ID] = &conn
	return nil
}

func (m *Manager) RemoveConnection(id string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.connections, id)
}

func (m *Manager) ListConnections() []Connection {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]Connection, 0, len(m.connections))
	for _, c := range m.connections {
		result = append(result, *c)
	}
	return result
}

func (m *Manager) GetConnection(id string) *Connection {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.connections[id]
}

func (m *Manager) Connect(ctx context.Context, id string) error {
	m.mu.Lock()
	conn, ok := m.connections[id]
	if !ok {
		m.mu.Unlock()
		return fmt.Errorf("connection %s not found", id)
	}
	conn.Status = StatusConnecting
	m.mu.Unlock()

	switch conn.Type {
	case TypeSSH:
		err := m.connectSSH(ctx, conn)
		if err != nil {
			m.mu.Lock()
			conn.Status = StatusError
			conn.LastError = err.Error()
			m.mu.Unlock()
			return err
		}
	case TypeContainer:
		err := m.connectContainer(ctx, conn)
		if err != nil {
			m.mu.Lock()
			conn.Status = StatusError
			conn.LastError = err.Error()
			m.mu.Unlock()
			return err
		}
	default:
		m.mu.Lock()
		conn.Status = StatusError
		conn.LastError = fmt.Sprintf("unsupported connection type: %s", conn.Type)
		m.mu.Unlock()
		return fmt.Errorf("unsupported connection type: %s", conn.Type)
	}

	m.mu.Lock()
	conn.Status = StatusConnected
	conn.ConnectedAt = time.Now()
	conn.LastError = ""
	m.mu.Unlock()
	return nil
}

func (m *Manager) Disconnect(id string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	conn, ok := m.connections[id]
	if !ok {
		return
	}
	conn.Status = StatusDisconnected
	conn.ConnectedAt = time.Time{}
}

func (m *Manager) connectSSH(_ context.Context, conn *Connection) error {
	if conn.Host == "" {
		return fmt.Errorf("SSH host is required")
	}
	if conn.User == "" {
		conn.User = "root"
	}
	if conn.Port == 0 {
		conn.Port = 22
	}
	return nil
}

func (m *Manager) connectContainer(_ context.Context, conn *Connection) error {
	if conn.Container == "" {
		return fmt.Errorf("container name/id is required")
	}
	return nil
}
