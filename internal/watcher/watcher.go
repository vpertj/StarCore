package watcher

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	wailsRuntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

type FileChange struct {
	Path      string `json:"path"`
	EventType string `json:"eventType"` // "created", "modified", "deleted"
}

type Watcher struct {
	ctx       context.Context
	rootPath  string
	mu        sync.Mutex
	lastState map[string]time.Time
	stopCh    chan struct{}
}

var DefaultIgnoreDirs = map[string]bool{
	"node_modules": true, ".git": true, "dist": true, "build": true, "out": true,
	"__pycache__": true, ".next": true, ".nuxt": true, "vendor": true,
	".venv": true, "venv": true, ".tox": true, ".mypy_cache": true,
	"target": true, "bin": true, "obj": true, ".idea": true, ".vscode": true,
	".cache": true, ".gradle": true, "coverage": true, ".pytest_cache": true,
}

func New(rootPath string) *Watcher {
	return &Watcher{
		rootPath:  rootPath,
		lastState: make(map[string]time.Time),
		stopCh:    make(chan struct{}),
	}
}

func (w *Watcher) SetContext(ctx context.Context) {
	w.ctx = ctx
}

func (w *Watcher) Scan() map[string]time.Time {
	state := make(map[string]time.Time)
	filepath.Walk(w.rootPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if info.IsDir() {
			if DefaultIgnoreDirs[info.Name()] || strings.HasPrefix(info.Name(), ".") {
				return filepath.SkipDir
			}
			return nil
		}
		state[path] = info.ModTime()
		return nil
	})
	return state
}

func (w *Watcher) Start(interval time.Duration) {
	w.lastState = w.Scan()

	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for {
			select {
			case <-w.stopCh:
				return
			case <-ticker.C:
				w.checkChanges()
			}
		}
	}()
}

func (w *Watcher) checkChanges() {
	current := w.Scan()

	w.mu.Lock()
	prev := w.lastState
	w.lastState = current
	w.mu.Unlock()

	// Detect new and modified files
	for path, mtime := range current {
		if prevMtime, ok := prev[path]; !ok {
			w.emit(path, "created")
		} else if mtime.After(prevMtime) {
			w.emit(path, "modified")
		}
	}

	// Detect deleted files
	for path := range prev {
		if _, ok := current[path]; !ok {
			w.emit(path, "deleted")
		}
	}
}

func (w *Watcher) emit(path, eventType string) {
	if w.ctx == nil {
		return
	}
	change := FileChange{
		Path:      path,
		EventType: eventType,
	}
	wailsRuntime.EventsEmit(w.ctx, "file:change", change)
}

func (w *Watcher) Stop() {
	close(w.stopCh)
}
