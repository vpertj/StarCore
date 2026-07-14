package ai

import (
	"compress/gzip"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// --- SQLite Trace Storage ---
//
// Phase 6: Full-chain trace persistence.
//
// Stores every trace event in a SQLite database shared with the memory store.
// This enables:
//   - Historical trace viewing (see what the agent did in past sessions)
//   - Token usage analytics (correlate trace events with cost)
//   - Debugging (replay agent decisions step by step)
//   - Performance analysis (identify bottlenecks per tool/stage)
//
// Design:
//   - Batched writes via buffered channel + background goroutine
//   - Events compressed with gzip to save disk space (text-heavy)
//   - Automatic retention: configurable max age + max rows per conversation
//   - Schema: traces (header) + trace_events (one row per event)
//
// Thread-safe: all operations go through a single goroutine via channels.

const (
	// Default retention policy.
	defaultMaxTraceAge   = 30 * 24 * time.Hour // 30 days
	defaultMaxTraces     = 500                 // max traces total
	defaultMaxEventsSize = 5 * 1024 * 1024     // 5 MB compressed events per trace
)

// SQLiteTraceSink persists traces to SQLite.
type SQLiteTraceSink struct {
	db *sql.DB

	// Configuration
	maxAge  time.Duration
	maxRows int

	// Background writer
	writeCh chan *Trace
	done    chan struct{}
}

// NewSQLiteTraceSink creates a SQLite-backed trace sink.
// The db handle is shared with the memory store.
func NewSQLiteTraceSink(db *sql.DB) (*SQLiteTraceSink, error) {
	sink := &SQLiteTraceSink{
		db:      db,
		maxAge:  defaultMaxTraceAge,
		maxRows: defaultMaxTraces,
		writeCh: make(chan *Trace, 16),
		done:    make(chan struct{}),
	}

	if err := sink.initSchema(); err != nil {
		return nil, fmt.Errorf("trace schema init failed: %w", err)
	}

	// Start background writer goroutine.
	go sink.writerLoop()

	return sink, nil
}

// NewSQLiteTraceSinkWithRetention creates a sink with custom retention.
func NewSQLiteTraceSinkWithRetention(db *sql.DB, maxAge time.Duration, maxRows int) (*SQLiteTraceSink, error) {
	sink, err := NewSQLiteTraceSink(db)
	if err != nil {
		return nil, err
	}
	sink.maxAge = maxAge
	sink.maxRows = maxRows
	return sink, nil
}

// initSchema creates the trace tables if they don't exist.
func (s *SQLiteTraceSink) initSchema() error {
	ddl := `
	CREATE TABLE IF NOT EXISTS traces (
		id TEXT PRIMARY KEY,
		conversation_id TEXT NOT NULL,
		total_loops INTEGER DEFAULT 0,
		total_tools INTEGER DEFAULT 0,
		total_errors INTEGER DEFAULT 0,
		token_in INTEGER DEFAULT 0,
		token_out INTEGER DEFAULT 0,
		duration_ms INTEGER DEFAULT 0,
		event_count INTEGER DEFAULT 0,
		events_blob BLOB,            -- gzip-compressed JSON array of events
		start_time TEXT NOT NULL,
		end_time TEXT,
		created_at TEXT DEFAULT (datetime('now'))
	);
	CREATE INDEX IF NOT EXISTS idx_traces_conv ON traces(conversation_id);
	CREATE INDEX IF NOT EXISTS idx_traces_time ON traces(created_at);

	CREATE TABLE IF NOT EXISTS trace_events (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		trace_id TEXT NOT NULL,
		evt_id TEXT NOT NULL,
		evt_type TEXT NOT NULL,
		stage TEXT DEFAULT '',
		agent_id TEXT DEFAULT '',
		tool_name TEXT DEFAULT '',
		message TEXT DEFAULT '',
		loop_num INTEGER DEFAULT 0,
		token_in INTEGER DEFAULT 0,
		token_out INTEGER DEFAULT 0,
		timestamp TEXT NOT NULL,
		FOREIGN KEY (trace_id) REFERENCES traces(id) ON DELETE CASCADE
	);
	CREATE INDEX IF NOT EXISTS idx_trace_events_trace ON trace_events(trace_id);
	CREATE INDEX IF NOT EXISTS idx_trace_events_type ON trace_events(evt_type);
	`
	_, err := s.db.Exec(ddl)
	return err
}

// SaveTrace implements TraceSink. It sends the trace to the background writer.
// This method is non-blocking (the channel has a buffer).
func (s *SQLiteTraceSink) SaveTrace(t *Trace) error {
	if t == nil {
		return nil
	}
	select {
	case s.writeCh <- t:
		return nil
	default:
		// Channel full — do a synchronous write to avoid losing data.
		return s.persistTrace(t)
	}
}

// writerLoop is the background goroutine that drains the write channel.
func (s *SQLiteTraceSink) writerLoop() {
	for {
		select {
		case t := <-s.writeCh:
			if err := s.persistTrace(t); err != nil {
				// Log but don't panic — trace storage is best-effort.
				logPrintf("trace persist error: %v", err)
			}
		case <-s.done:
			// Drain remaining.
			for {
				select {
				case t := <-s.writeCh:
					_ = s.persistTrace(t)
				default:
					return
				}
			}
		}
	}
}

// persistTrace writes a trace and its events to the database.
func (s *SQLiteTraceSink) persistTrace(t *Trace) error {
	// Compress events.
	eventsJSON, err := json.Marshal(t.Events)
	if err != nil {
		return fmt.Errorf("marshal events: %w", err)
	}

	var compressed []byte
	if len(eventsJSON) > 1024 {
		// Only compress if there's meaningful data.
		var buf strings.Builder
		wc := gzip.NewWriter(&buf)
		if _, werr := wc.Write(eventsJSON); werr != nil {
			wc.Close()
			return fmt.Errorf("gzip write: %w", werr)
		}
		if cerr := wc.Close(); cerr != nil {
			return fmt.Errorf("gzip close: %w", cerr)
		}
		compressed = []byte(buf.String())
	} else {
		compressed = eventsJSON
	}

	if int64(len(compressed)) > defaultMaxEventsSize {
		// Truncate events if too large — keep first and last portions.
		targetSize := defaultMaxEventsSize / 2
		truncated := make([]TraceEvent, 0, targetSize)
		// Keep first half of events.
		for i := 0; i < len(t.Events); i++ {
			truncated = append(truncated, t.Events[i])
			if len(truncated) >= targetSize/2 {
				break
			}
		}
		// Add a separator event.
		remaining := len(t.Events) - len(truncated)*2
		if remaining < 0 {
			remaining = 0
		}
		truncated = append(truncated, TraceEvent{
			Type:    EventStreamInterrupt,
			Message: fmt.Sprintf("[截断了 %d 个事件]", remaining),
		})
		// Keep last portion.
		startIdx := len(t.Events) - targetSize/2
		if startIdx < len(truncated) {
			startIdx = len(truncated)
		}
		for i := startIdx; i < len(t.Events); i++ {
			truncated = append(truncated, t.Events[i])
		}
		eventsJSON, _ = json.Marshal(truncated)
		var buf strings.Builder
		wc := gzip.NewWriter(&buf)
		_, _ = wc.Write(eventsJSON)
		wc.Close()
		compressed = []byte(buf.String())
	}

	endTime := ""
	if !t.EndTime.IsZero() {
		endTime = t.EndTime.Format(time.RFC3339)
	}

	// Insert trace header.
	_, err = s.db.Exec(
		`INSERT INTO traces (id, conversation_id, total_loops, total_tools, total_errors,
			token_in, token_out, duration_ms, event_count, events_blob, start_time, end_time)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		t.ID, t.ConversationID, t.TotalLoops, t.TotalTools, t.TotalErrors,
		t.TokenIn, t.TokenOut, t.Duration, len(t.Events), compressed,
		t.StartTime.Format(time.RFC3339), endTime,
	)
	if err != nil {
		return fmt.Errorf("insert trace: %w", err)
	}

	// Insert individual events (for querying by type/tool).
	for _, evt := range t.Events {
		_, err = s.db.Exec(
			`INSERT INTO trace_events (trace_id, evt_id, evt_type, stage, agent_id, tool_name, message, loop_num, token_in, token_out, timestamp)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			t.ID, evt.ID, string(evt.Type), string(evt.Stage), evt.AgentID, evt.ToolName,
			evt.Message, evt.Loop, evt.TokenIn, evt.TokenOut,
			evt.Timestamp.Format(time.RFC3339),
		)
		if err != nil {
			return fmt.Errorf("insert event %s: %w", evt.ID, err)
		}
	}

	// Apply retention policy.
	s.enforceRetention()

	return nil
}

// enforceRetention deletes old traces beyond the configured limits.
func (s *SQLiteTraceSink) enforceRetention() {
	if s.maxAge > 0 {
		cutoff := time.Now().Add(-s.maxAge).Format(time.RFC3339)
		s.db.Exec(`DELETE FROM traces WHERE created_at < ?`, cutoff)
	}
	if s.maxRows > 0 {
		// Keep only the newest maxRows traces.
		s.db.Exec(
			`DELETE FROM traces WHERE id NOT IN (
				SELECT id FROM traces ORDER BY created_at DESC LIMIT ?
			)`,
			s.maxRows,
		)
	}
}

// Close shuts down the background writer.
func (s *SQLiteTraceSink) Close() {
	close(s.done)
}

// --- Query Methods ---

// GetTraces returns traces for a conversation (newest first).
func (s *SQLiteTraceSink) GetTraces(convID string, limit int) ([]TraceHeader, error) {
	if limit <= 0 {
		limit = 50
	}
	rows, err := s.db.Query(
		`SELECT id, conversation_id, total_loops, total_tools, total_errors,
			token_in, token_out, duration_ms, event_count, start_time, end_time, created_at
		FROM traces WHERE conversation_id = ?
		ORDER BY created_at DESC LIMIT ?`,
		convID, limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var traces []TraceHeader
	for rows.Next() {
		var h TraceHeader
		var endTime sql.NullString
		err := rows.Scan(&h.ID, &h.ConversationID, &h.TotalLoops, &h.TotalTools,
			&h.TotalErrors, &h.TokenIn, &h.TokenOut, &h.DurationMs, &h.EventCount,
			&h.StartTime, &endTime, &h.CreatedAt)
		if err != nil {
			return nil, err
		}
		h.EndTime = endTime.String
		traces = append(traces, h)
	}
	return traces, rows.Err()
}

// GetTraceEvents returns all events for a specific trace.
func (s *SQLiteTraceSink) GetTraceEvents(traceID string) ([]TraceEvent, error) {
	rows, err := s.db.Query(
		`SELECT evt_id, evt_type, stage, agent_id, tool_name, message, loop_num, token_in, token_out, timestamp
		FROM trace_events WHERE trace_id = ?
		ORDER BY id ASC`,
		traceID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var events []TraceEvent
	for rows.Next() {
		var e TraceEvent
		var stage, agentID, sqlToolName sql.NullString
		if err := rows.Scan(&e.ID, &e.Type, &stage, &agentID, &sqlToolName,
			&e.Message, &e.Loop, &e.TokenIn, &e.TokenOut, &e.Timestamp); err != nil {
			return nil, err
		}
		e.Stage = Stage(stage.String)
		e.AgentID = agentID.String
		e.ToolName = sqlToolName.String
		events = append(events, e)
	}
	return events, rows.Err()
}

// GetTraceByConversation returns the most recent trace for a conversation.
func (s *SQLiteTraceSink) GetTraceByConversation(convID string) (*TraceHeader, error) {
	var h TraceHeader
	var endTime sql.NullString
	err := s.db.QueryRow(
		`SELECT id, conversation_id, total_loops, total_tools, total_errors,
			token_in, token_out, duration_ms, event_count, start_time, end_time, created_at
		FROM traces WHERE conversation_id = ?
		ORDER BY created_at DESC LIMIT 1`,
		convID,
	).Scan(&h.ID, &h.ConversationID, &h.TotalLoops, &h.TotalTools,
		&h.TotalErrors, &h.TokenIn, &h.TokenOut, &h.DurationMs, &h.EventCount,
		&h.StartTime, &endTime, &h.CreatedAt)
	if err != nil {
		return nil, err
	}
	h.EndTime = endTime.String
	return &h, nil
}

// GetEventStats returns aggregated statistics for a conversation.
func (s *SQLiteTraceSink) GetEventStats(convID string) (map[string]int, error) {
	rows, err := s.db.Query(
		`SELECT te.evt_type, COUNT(*) as cnt
		FROM trace_events te
		JOIN traces t ON te.trace_id = t.id
		WHERE t.conversation_id = ?
		GROUP BY te.evt_type`,
		convID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	stats := make(map[string]int)
	for rows.Next() {
		var evtType string
		var count int
		if err := rows.Scan(&evtType, &count); err != nil {
			return nil, err
		}
		stats[evtType] = count
	}
	return stats, rows.Err()
}

// TraceHeader is the summary view of a trace (without full events).
type TraceHeader struct {
	ID             string `json:"id"`
	ConversationID string `json:"conversation_id"`
	TotalLoops     int    `json:"total_loops"`
	TotalTools     int    `json:"total_tools"`
	TotalErrors    int    `json:"total_errors"`
	TokenIn        int    `json:"token_in"`
	TokenOut       int    `json:"token_out"`
	DurationMs     int64  `json:"duration_ms"`
	EventCount     int    `json:"event_count"`
	StartTime      string `json:"start_time"`
	EndTime        string `json:"end_time"`
	CreatedAt      string `json:"created_at"`
}

// logPrintf is a simple logging helper (avoids importing log just for this).
func logPrintf(format string, args ...interface{}) {
	// Use fmt to avoid pulling in log package for a single usage.
	fmt.Printf("[TraceStorage] "+format+"\n", args...)
}
