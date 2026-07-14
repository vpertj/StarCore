package sandbox

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"
)

type AuditEntry struct {
	Timestamp   string `json:"timestamp"`
	ToolID      string `json:"toolId"`
	Action      string `json:"action"`
	User        string `json:"user,omitempty"`
	Args        string `json:"args,omitempty"`
	Result      string `json:"result,omitempty"`
	Error       string `json:"error,omitempty"`
	Approved    bool   `json:"approved"`
	ProjectPath string `json:"projectPath,omitempty"`
}

type AuditLogger struct {
	filePath string
	mu       sync.Mutex
	file     *os.File
}

var (
	globalAuditLogger *AuditLogger
	auditOnce         sync.Once
)

func InitAuditLogger(configDir string) error {
	var initErr error
	auditOnce.Do(func() {
		logDir := filepath.Join(configDir, "audit")
		if err := os.MkdirAll(logDir, 0755); err != nil {
			initErr = fmt.Errorf("create audit dir: %w", err)
			return
		}

		logPath := filepath.Join(logDir, fmt.Sprintf("audit_%s.log", time.Now().Format("2006-01-02")))
		f, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
		if err != nil {
			initErr = fmt.Errorf("open audit log: %w", err)
			return
		}

		globalAuditLogger = &AuditLogger{
			filePath: logPath,
			file:     f,
		}
	})
	return initErr
}

func LogAudit(toolID, action string, args map[string]any, result string, err error, approved bool) {
	if globalAuditLogger == nil {
		return
	}

	argsJSON, _ := json.Marshal(args)
	if len(argsJSON) > 2000 {
		argsJSON = argsJSON[:2000]
	}

	errorStr := ""
	if err != nil {
		errorStr = err.Error()
	}

	resultTruncated := result
	if len(resultTruncated) > 1000 {
		resultTruncated = resultTruncated[:1000] + "... [truncated]"
	}

	entry := AuditEntry{
		Timestamp: time.Now().Format(time.RFC3339Nano),
		ToolID:    toolID,
		Action:    action,
		Args:      string(argsJSON),
		Result:    resultTruncated,
		Error:     errorStr,
		Approved:  approved,
	}

	globalAuditLogger.writeEntry(entry)
}

func (l *AuditLogger) writeEntry(entry AuditEntry) {
	l.mu.Lock()
	defer l.mu.Unlock()

	data, err := json.Marshal(entry)
	if err != nil {
		return
	}

	if _, err := l.file.Write(data); err != nil {
		log.Printf("WARNING: failed to write audit entry: %v", err)
		return
	}
	if _, err := l.file.Write([]byte("\n")); err != nil {
		log.Printf("WARNING: failed to write audit newline: %v", err)
	}
}

func CloseAuditLogger() {
	if globalAuditLogger != nil && globalAuditLogger.file != nil {
		globalAuditLogger.file.Close()
	}
}
