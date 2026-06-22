package watcher

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"

	wailsRuntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

type FileChange struct {
	Path      string `json:"path"`
	EventType string `json:"eventType"`
}

type Watcher struct {
	ctx         context.Context
	rootPath    string
	mu          sync.Mutex
	lastState   map[string]time.Time
	stopCh      chan struct{}
	fsw         *fsnotify.Watcher
	useFSNotify bool
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

func (w *Watcher) Start(interval time.Duration) {
	fsw, err := fsnotify.NewWatcher()
	if err == nil {
		w.fsw = fsw
		w.useFSNotify = true
		w.startFSNotify()
		return
	}
	w.useFSNotify = false
	w.startPolling(interval)
}

func (w *Watcher) startFSNotify() {
	if w.fsw == nil || w.rootPath == "" {
		return
	}

	err := w.fsw.Add(w.rootPath)
	if err != nil {
		w.startPolling(2 * time.Second)
		return
	}

	go func() {
		var debounceTimer *time.Timer
		var debounceMu sync.Mutex
		pendingEvents := make(map[string]string)

		flushEvents := func() {
			debounceMu.Lock()
			events := make(map[string]string)
			for k, v := range pendingEvents {
				events[k] = v
			}
			pendingEvents = make(map[string]string)
			debounceMu.Unlock()

			for path, eventType := range events {
				w.emit(path, eventType)
			}
		}

		for {
			select {
			case <-w.stopCh:
				w.fsw.Close()
				return
			case event, ok := <-w.fsw.Events:
				if !ok {
					return
				}
				if w.shouldIgnore(event.Name) {
					continue
				}
				eventType := "modified"
				if event.Op&fsnotify.Create != 0 {
					eventType = "created"
				} else if event.Op&fsnotify.Remove != 0 || event.Op&fsnotify.Rename != 0 {
					eventType = "deleted"
				}

				debounceMu.Lock()
				pendingEvents[event.Name] = eventType
				debounceMu.Unlock()

				if debounceTimer != nil {
					debounceTimer.Reset(50 * time.Millisecond)
				} else {
					debounceTimer = time.AfterFunc(50*time.Millisecond, flushEvents)
				}

			case err, ok := <-w.fsw.Errors:
				if !ok {
					return
				}
				_ = err
			}
		}
	}()
}

func (w *Watcher) shouldIgnore(path string) bool {
	parts := strings.Split(filepath.ToSlash(path), "/")
	for _, part := range parts {
		if DefaultIgnoreDirs[part] {
			return true
		}
		if strings.HasPrefix(part, ".") && part != "." {
			return true
		}
	}
	return false
}

func (w *Watcher) startPolling(interval time.Duration) {
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

func (w *Watcher) Scan() map[string]time.Time {
	state := make(map[string]time.Time)
	if w.rootPath == "" {
		return state
	}
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

func (w *Watcher) checkChanges() {
	current := w.Scan()

	w.mu.Lock()
	prev := w.lastState
	w.lastState = current
	w.mu.Unlock()

	for path, mtime := range current {
		if prevMtime, ok := prev[path]; !ok {
			w.emit(path, "created")
		} else if mtime.After(prevMtime) {
			w.emit(path, "modified")
		}
	}

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
	select {
	case <-w.stopCh:
	default:
		close(w.stopCh)
	}
	if w.fsw != nil {
		w.fsw.Close()
		w.fsw = nil
	}
}
