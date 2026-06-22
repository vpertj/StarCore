package workspace

import (
	"path/filepath"
	"testing"
)

func TestAddRoot(t *testing.T) {
	m := NewManager()
	m.AddRoot(filepath.Join("home", "user", "project1"))
	m.AddRoot(filepath.Join("home", "user", "project2"))

	roots := m.Roots()
	if len(roots) != 2 {
		t.Fatalf("expected 2 roots, got %d", len(roots))
	}
	if !roots[0].Active {
		t.Error("first root should be active")
	}
	if roots[1].Active {
		t.Error("second root should not be active")
	}
}

func TestAddRootDuplicate(t *testing.T) {
	m := NewManager()
	p := filepath.Join("home", "user", "project1")
	m.AddRoot(p)
	m.AddRoot(p)

	if len(m.Roots()) != 1 {
		t.Fatalf("expected 1 root, got %d", len(m.Roots()))
	}
}

func TestRemoveRoot(t *testing.T) {
	m := NewManager()
	p1 := filepath.Join("home", "user", "project1")
	p2 := filepath.Join("home", "user", "project2")
	m.AddRoot(p1)
	m.AddRoot(p2)
	m.RemoveRoot(p1)

	roots := m.Roots()
	if len(roots) != 1 {
		t.Fatalf("expected 1 root, got %d", len(roots))
	}
	if !roots[0].Active {
		t.Error("remaining root should become active")
	}
}

func TestSetActive(t *testing.T) {
	m := NewManager()
	p1 := filepath.Join("home", "user", "project1")
	p2 := filepath.Join("home", "user", "project2")
	m.AddRoot(p1)
	m.AddRoot(p2)
	m.SetActive(p2)

	r := m.ActiveRoot()
	if r == nil {
		t.Fatal("expected non-nil active root")
	}
	if !r.Active {
		t.Error("root should be active")
	}
}

func TestActivePath(t *testing.T) {
	m := NewManager()
	if m.ActivePath() != "" {
		t.Error("empty manager should return empty path")
	}
	p := filepath.Join("home", "user", "project1")
	m.AddRoot(p)
	if m.ActivePath() == "" {
		t.Error("expected non-empty path after adding root")
	}
}

func TestFindRootForPath(t *testing.T) {
	m := NewManager()
	p1 := filepath.Join("home", "user", "project1")
	p2 := filepath.Join("home", "user", "project2")
	m.AddRoot(p1)
	m.AddRoot(p2)

	r := m.FindRootForPath(filepath.Join("home", "user", "project1", "src", "main.go"))
	if r == nil {
		t.Fatal("expected non-nil root")
	}
}
