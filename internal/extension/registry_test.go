package extension

import (
	"os"
	"path/filepath"
	"testing"
)

func TestRegister(t *testing.T) {
	r := NewRegistry(t.TempDir())
	err := r.Register(Extension{ID: "test-ext", Name: "Test", Version: "1.0.0"})
	if err != nil {
		t.Fatal(err)
	}

	ext := r.Get("test-ext")
	if ext == nil {
		t.Fatal("expected extension to be registered")
	}
	if !ext.Enabled {
		t.Error("extension should be enabled by default")
	}
}

func TestRegisterDuplicate(t *testing.T) {
	r := NewRegistry(t.TempDir())
	r.Register(Extension{ID: "test-ext"})
	err := r.Register(Extension{ID: "test-ext"})
	if err == nil {
		t.Error("expected error for duplicate registration")
	}
}

func TestRegisterEmptyID(t *testing.T) {
	r := NewRegistry(t.TempDir())
	err := r.Register(Extension{})
	if err == nil {
		t.Error("expected error for empty ID")
	}
}

func TestSetEnabled(t *testing.T) {
	r := NewRegistry(t.TempDir())
	r.Register(Extension{ID: "test-ext"})

	r.SetEnabled("test-ext", false)
	ext := r.Get("test-ext")
	if ext.Enabled {
		t.Error("extension should be disabled")
	}

	enabled := r.ListEnabled()
	if len(enabled) != 0 {
		t.Error("expected no enabled extensions")
	}
}

func TestGetCommands(t *testing.T) {
	r := NewRegistry(t.TempDir())
	r.Register(Extension{
		ID: "test-ext",
		Commands: []CommandContribution{
			{ID: "cmd1", Label: "Command 1"},
			{ID: "cmd2", Label: "Command 2"},
		},
	})

	cmds := r.GetCommands()
	if len(cmds) != 2 {
		t.Fatalf("expected 2 commands, got %d", len(cmds))
	}
}

func TestLoadFromDir(t *testing.T) {
	dir := t.TempDir()
	extDir := filepath.Join(dir, "my-ext")
	os.MkdirAll(extDir, 0755)

	manifest := `{"id":"my-ext","name":"My Extension","version":"1.0.0"}`
	os.WriteFile(filepath.Join(extDir, "extension.json"), []byte(manifest), 0644)

	r := NewRegistry(t.TempDir())
	err := r.LoadFromDir(dir)
	if err != nil {
		t.Fatal(err)
	}

	ext := r.Get("my-ext")
	if ext == nil {
		t.Fatal("expected extension loaded from dir")
	}
}

func TestSaveLoadConfig(t *testing.T) {
	dir := t.TempDir()
	r := NewRegistry(dir)
	r.Register(Extension{ID: "test-ext", Name: "Test"})

	if err := r.SaveConfig(); err != nil {
		t.Fatal(err)
	}

	r2 := NewRegistry(dir)
	if err := r2.LoadConfig(); err != nil {
		t.Fatal(err)
	}

	ext := r2.Get("test-ext")
	if ext == nil {
		t.Fatal("expected extension loaded from config")
	}
}
