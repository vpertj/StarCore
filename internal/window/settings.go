package window

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	"os/exec"
	"sync"
	"syscall"
	"time"
)

//go:embed settings.html
var settingsHTML []byte

type Settings struct {
	Theme      string         `json:"theme"`
	FontFamily string         `json:"fontFamily"`
	FontSize   int            `json:"fontSize"`
	LineHeight float64        `json:"lineHeight"`
	WordWrap   bool           `json:"wordWrap"`
	Minimap    bool           `json:"minimap"`
	Lang       string         `json:"lang"`
	Providers  []ProviderInfo `json:"providers"`
	Models     []ModelInfo    `json:"models"`
}

type ProviderInfo struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Endpoint string `json:"endpoint"`
}

type ModelInfo struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	ProviderName string `json:"providerName"`
	Enabled      bool   `json:"enabled"`
}

var (
	mu       sync.Mutex
	getCB    func() Settings
	saveCB   func(Settings)
	listener net.Listener
	server   *http.Server
	serverMu sync.Mutex
)

func InitCallbacks(get func() Settings, save func(Settings)) {
	getCB = get
	saveCB = save
}

func ShowStandaloneSettings(mainX, mainY, mainW, mainH int) {
	serverMu.Lock()
	if server != nil {
		serverMu.Unlock()
		return
	}
	serverMu.Unlock()

	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		log.Printf("settings popup: listen err: %v", err)
		return
	}
	listener = l

	mux := http.NewServeMux()
	mux.HandleFunc("/settings", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Write(settingsHTML)
	})
	mux.HandleFunc("/api/get", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Access-Control-Allow-Origin", "*")
		if getCB == nil {
			w.Write([]byte("{}"))
			return
		}
		s := getCB()
		b, _ := json.Marshal(s)
		w.Write(b)
	})
	mux.HandleFunc("/api/save", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusOK)
			return
		}
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		var s Settings
		if err := json.NewDecoder(r.Body).Decode(&s); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if saveCB != nil {
			saveCB(s)
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"ok":true}`))
	})
	mux.HandleFunc("/api/close", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Write([]byte(`{"ok":true}`))
		go func() {
			serverMu.Lock()
			defer serverMu.Unlock()
			if server != nil {
				server.Close()
				server = nil
			}
			if listener != nil {
				listener.Close()
				listener = nil
			}
		}()
	})

	srv := &http.Server{Handler: mux}
	serverMu.Lock()
	server = srv
	serverMu.Unlock()

	go func() {
		if err := srv.Serve(l); err != nil && err != http.ErrServerClosed {
			log.Printf("settings popup: serve err: %v", err)
		}
	}()

	port := l.Addr().(*net.TCPAddr).Port
	url := fmt.Sprintf("http://127.0.0.1:%d/settings", port)

	ww, wh := 620, 480
	cx := mainX + (mainW-ww)/2
	cy := mainY + (mainH-wh)/2
	if mainW == 0 || mainH == 0 {
		// Fallback: center on screen
		dll := syscall.NewLazyDLL("user32.dll")
		proc := dll.NewProc("GetSystemMetrics")
		sx, _, _ := proc.Call(0)
		sy, _, _ := proc.Call(1)
		cx = (int(sx) - ww) / 2
		cy = (int(sy) - wh) / 2
	}
	if cx < 0 { cx = 0 }
	if cy < 0 { cy = 0 }
	launchEdge(url, ww, wh, cx, cy)

	go func() {
		time.Sleep(5 * time.Minute)
		serverMu.Lock()
		if server != nil {
			server.Close()
			server = nil
		}
		if listener != nil {
			listener.Close()
			listener = nil
		}
		serverMu.Unlock()
	}()
}

func launchEdge(url string, ww, wh, cx, cy int) {
	edges := []string{
		`C:\Program Files (x86)\Microsoft\Edge\Application\msedge.exe`,
		`C:\Program Files\Microsoft\Edge\Application\msedge.exe`,
		`msedge`,
		`msedge.exe`,
	}
	for _, e := range edges {
		cmd := exec.Command(e,
			fmt.Sprintf("--app=%s", url),
			"--new-window",
			fmt.Sprintf("--window-size=%d,%d", ww, wh),
			fmt.Sprintf("--window-position=%d,%d", cx, cy),
			"--no-first-run",
			"--no-default-browser-check",
		)
		if err := cmd.Start(); err == nil {
			return
		}
	}
	log.Printf("settings popup: could not launch Edge")
}
