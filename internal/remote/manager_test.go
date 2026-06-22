package remote

import (
	"context"
	"testing"
)

func TestAddConnection(t *testing.T) {
	m := NewManager()
	err := m.AddConnection(Connection{
		ID:   "ssh-1",
		Name: "dev-server",
		Type: TypeSSH,
		Host: "dev.example.com",
		Port: 22,
		User: "root",
	})
	if err != nil {
		t.Fatal(err)
	}

	conn := m.GetConnection("ssh-1")
	if conn == nil {
		t.Fatal("expected connection to exist")
	}
	if conn.Status != StatusDisconnected {
		t.Error("new connection should be disconnected")
	}
}

func TestAddDuplicate(t *testing.T) {
	m := NewManager()
	m.AddConnection(Connection{ID: "ssh-1", Type: TypeSSH, Host: "host"})
	err := m.AddConnection(Connection{ID: "ssh-1", Type: TypeSSH, Host: "host2"})
	if err == nil {
		t.Error("expected error for duplicate")
	}
}

func TestRemoveConnection(t *testing.T) {
	m := NewManager()
	m.AddConnection(Connection{ID: "ssh-1", Type: TypeSSH, Host: "host"})
	m.RemoveConnection("ssh-1")
	if m.GetConnection("ssh-1") != nil {
		t.Error("connection should be removed")
	}
}

func TestConnectSSH(t *testing.T) {
	m := NewManager()
	m.AddConnection(Connection{
		ID:   "ssh-1",
		Type: TypeSSH,
		Host: "dev.example.com",
		Port: 22,
		User: "root",
	})

	err := m.Connect(context.Background(), "ssh-1")
	if err != nil {
		t.Fatal(err)
	}

	conn := m.GetConnection("ssh-1")
	if conn.Status != StatusConnected {
		t.Errorf("expected connected, got %s", conn.Status)
	}
}

func TestConnectSSHNoHost(t *testing.T) {
	m := NewManager()
	m.AddConnection(Connection{ID: "ssh-1", Type: TypeSSH})

	err := m.Connect(context.Background(), "ssh-1")
	if err == nil {
		t.Error("expected error for missing host")
	}

	conn := m.GetConnection("ssh-1")
	if conn.Status != StatusError {
		t.Errorf("expected error status, got %s", conn.Status)
	}
}

func TestConnectContainer(t *testing.T) {
	m := NewManager()
	m.AddConnection(Connection{
		ID:        "ctr-1",
		Type:      TypeContainer,
		Container: "my-container",
	})

	err := m.Connect(context.Background(), "ctr-1")
	if err != nil {
		t.Fatal(err)
	}

	conn := m.GetConnection("ctr-1")
	if conn.Status != StatusConnected {
		t.Errorf("expected connected, got %s", conn.Status)
	}
}

func TestDisconnect(t *testing.T) {
	m := NewManager()
	m.AddConnection(Connection{ID: "ssh-1", Type: TypeSSH, Host: "host"})
	m.Connect(context.Background(), "ssh-1")
	m.Disconnect("ssh-1")

	conn := m.GetConnection("ssh-1")
	if conn.Status != StatusDisconnected {
		t.Errorf("expected disconnected, got %s", conn.Status)
	}
}

func TestListConnections(t *testing.T) {
	m := NewManager()
	m.AddConnection(Connection{ID: "ssh-1", Type: TypeSSH, Host: "host1"})
	m.AddConnection(Connection{ID: "ctr-1", Type: TypeContainer, Container: "ctr"})

	list := m.ListConnections()
	if len(list) != 2 {
		t.Fatalf("expected 2 connections, got %d", len(list))
	}
}
