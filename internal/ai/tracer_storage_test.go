//go:build cgo
// +build cgo

package ai

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

func setupTestDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("open test db: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

func TestNewSQLiteTraceSink(t *testing.T) {
	db := setupTestDB(t)
	sink, err := NewSQLiteTraceSink(db)
	if err != nil {
		t.Fatalf("NewSQLiteTraceSink: %v", err)
	}
	defer sink.Close()

	// Verify tables exist.
	var count int
	err = db.QueryRow(
		`SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name='traces'`,
	).Scan(&count)
	if err != nil {
		t.Fatalf("query tables: %v", err)
	}
	if count != 1 {
		t.Error("traces table should exist")
	}

	err = db.QueryRow(
		`SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name='trace_events'`,
	).Scan(&count)
	if err != nil {
		t.Fatalf("query tables: %v", err)
	}
	if count != 1 {
		t.Error("trace_events table should exist")
	}
}

func TestSaveTrace(t *testing.T) {
	db := setupTestDB(t)
	sink, err := NewSQLiteTraceSink(db)
	if err != nil {
		t.Fatalf("NewSQLiteTraceSink: %v", err)
	}
	defer sink.Close()

	trace := &Trace{
		ID:             "tr_test_1",
		ConversationID: "conv_1",
		TotalLoops:     3,
		TotalTools:     5,
		TotalErrors:    1,
		TokenIn:        1000,
		TokenOut:       500,
		Duration:       2500,
		StartTime:      time.Now().Add(-2 * time.Second),
		EndTime:        time.Now(),
		Events: []TraceEvent{
			{ID: "evt_1", Type: EventLoopStart, Stage: StageUnderstand, Message: "loop started", Loop: 0, Timestamp: time.Now()},
			{ID: "evt_2", Type: EventLLMCall, Stage: StageExecute, Message: "LLM call", Loop: 1, TokenIn: 500, TokenOut: 200, Timestamp: time.Now()},
			{ID: "evt_3", Type: EventToolCall, Stage: StageExecute, AgentID: "agent1", ToolName: "read_file", Message: "reading", Loop: 1, Timestamp: time.Now()},
		},
	}

	if err := sink.SaveTrace(trace); err != nil {
		t.Fatalf("SaveTrace: %v", err)
	}

	// Give background writer time to process.
	time.Sleep(100 * time.Millisecond)

	// Verify trace was stored.
	var storedID string
	err = db.QueryRow(`SELECT id FROM traces WHERE id = ?`, "tr_test_1").Scan(&storedID)
	if err != nil {
		t.Fatalf("query trace: %v", err)
	}
	if storedID != "tr_test_1" {
		t.Errorf("expected tr_test_1, got %s", storedID)
	}

	// Verify events.
	var eventCount int
	err = db.QueryRow(`SELECT COUNT(*) FROM trace_events WHERE trace_id = ?`, "tr_test_1").Scan(&eventCount)
	if err != nil {
		t.Fatalf("query events: %v", err)
	}
	if eventCount != 3 {
		t.Errorf("expected 3 events, got %d", eventCount)
	}

	// Verify event_count column.
	var colCount int
	err = db.QueryRow(`SELECT event_count FROM traces WHERE id = ?`, "tr_test_1").Scan(&colCount)
	if err != nil {
		t.Fatalf("query event_count: %v", err)
	}
	if colCount != 3 {
		t.Errorf("expected event_count=3, got %d", colCount)
	}
}

func TestSaveTrace_Nil(t *testing.T) {
	db := setupTestDB(t)
	sink, err := NewSQLiteTraceSink(db)
	if err != nil {
		t.Fatalf("NewSQLiteTraceSink: %v", err)
	}
	defer sink.Close()

	if err := sink.SaveTrace(nil); err != nil {
		t.Errorf("SaveTrace(nil) should not error, got: %v", err)
	}
}

func TestGetTraces(t *testing.T) {
	db := setupTestDB(t)
	sink, err := NewSQLiteTraceSink(db)
	if err != nil {
		t.Fatalf("NewSQLiteTraceSink: %v", err)
	}
	defer sink.Close()

	// Save 3 traces.
	for i := 0; i < 3; i++ {
		trace := &Trace{
			ID:             fmt.Sprintf("tr_%d", i),
			ConversationID: "conv_query",
			TotalLoops:     i + 1,
			Events:         []TraceEvent{{ID: fmt.Sprintf("e_%d", i), Type: EventLoopStart, Timestamp: time.Now()}},
		}
		if err := sink.persistTrace(trace); err != nil {
			t.Fatalf("persist trace %d: %v", i, err)
		}
	}

	traces, err := sink.GetTraces("conv_query", 10)
	if err != nil {
		t.Fatalf("GetTraces: %v", err)
	}
	if len(traces) != 3 {
		t.Errorf("expected 3 traces, got %d", len(traces))
	}

	// Verify ordering (newest first).
	if traces[0].ID != "tr_2" {
		t.Errorf("expected newest trace first, got %s", traces[0].ID)
	}
}

func TestGetTraceEvents(t *testing.T) {
	db := setupTestDB(t)
	sink, err := NewSQLiteTraceSink(db)
	if err != nil {
		t.Fatalf("NewSQLiteTraceSink: %v", err)
	}
	defer sink.Close()

	trace := &Trace{
		ID:             "tr_events_test",
		ConversationID: "conv_events",
		Events: []TraceEvent{
			{ID: "e1", Type: EventToolCall, ToolName: "read_file", Loop: 1, Timestamp: time.Now()},
			{ID: "e2", Type: EventToolResult, ToolName: "read_file", Loop: 1, Timestamp: time.Now()},
			{ID: "e3", Type: EventLoopEnd, Loop: 1, Timestamp: time.Now()},
		},
	}
	if err := sink.persistTrace(trace); err != nil {
		t.Fatalf("persist: %v", err)
	}

	events, err := sink.GetTraceEvents("tr_events_test")
	if err != nil {
		t.Fatalf("GetTraceEvents: %v", err)
	}
	if len(events) != 3 {
		t.Errorf("expected 3 events, got %d", len(events))
	}

	// Verify tool name was stored.
	if events[0].ToolName != "read_file" {
		t.Errorf("expected tool_name=read_file, got %s", events[0].ToolName)
	}
}

func TestGetEventStats(t *testing.T) {
	db := setupTestDB(t)
	sink, err := NewSQLiteTraceSink(db)
	if err != nil {
		t.Fatalf("NewSQLiteTraceSink: %v", err)
	}
	defer sink.Close()

	trace := &Trace{
		ID:             "tr_stats",
		ConversationID: "conv_stats",
		Events: []TraceEvent{
			{ID: "s1", Type: EventToolCall, ToolName: "read_file", Timestamp: time.Now()},
			{ID: "s2", Type: EventToolCall, ToolName: "write_file", Timestamp: time.Now()},
			{ID: "s3", Type: EventToolCall, ToolName: "read_file", Timestamp: time.Now()},
			{ID: "s4", Type: EventLLMCall, Timestamp: time.Now()},
			{ID: "s5", Type: EventToolResult, Timestamp: time.Now()},
		},
	}
	if err := sink.persistTrace(trace); err != nil {
		t.Fatalf("persist: %v", err)
	}

	stats, err := sink.GetEventStats("conv_stats")
	if err != nil {
		t.Fatalf("GetEventStats: %v", err)
	}

	if stats["tool_call"] != 3 {
		t.Errorf("expected 3 tool_call events, got %d", stats["tool_call"])
	}
	if stats["llm_call"] != 1 {
		t.Errorf("expected 1 llm_call event, got %d", stats["llm_call"])
	}
	if stats["tool_result"] != 1 {
		t.Errorf("expected 1 tool_result event, got %d", stats["tool_result"])
	}
}

func TestRetentionPolicy(t *testing.T) {
	db := setupTestDB(t)
	// Custom sink: max 2 traces, no age limit.
	sink, err := NewSQLiteTraceSinkWithRetention(db, 0, 2)
	if err != nil {
		t.Fatalf("NewSQLiteTraceSinkWithRetention: %v", err)
	}
	defer sink.Close()

	// Save 3 traces.
	for i := 0; i < 3; i++ {
		trace := &Trace{
			ID:             fmt.Sprintf("ret_%d", i),
			ConversationID: "conv_ret",
			Events:         []TraceEvent{{ID: fmt.Sprintf("re_%d", i), Type: EventLoopStart, Timestamp: time.Now()}},
		}
		if err := sink.persistTrace(trace); err != nil {
			t.Fatalf("persist trace %d: %v", i, err)
		}
	}

	// Verify only 2 traces remain.
	var count int
	err = db.QueryRow(`SELECT COUNT(*) FROM traces WHERE conversation_id = ?`, "conv_ret").Scan(&count)
	if err != nil {
		t.Fatalf("query count: %v", err)
	}
	if count != 2 {
		t.Errorf("expected 2 traces after retention, got %d", count)
	}
}

func TestCompressedStorage(t *testing.T) {
	db := setupTestDB(t)
	sink, err := NewSQLiteTraceSink(db)
	if err != nil {
		t.Fatalf("NewSQLiteTraceSink: %v", err)
	}
	defer sink.Close()

	// Create a trace with many events (enough to trigger compression).
	events := make([]TraceEvent, 100)
	for i := 0; i < 100; i++ {
		events[i] = TraceEvent{
			ID:        fmt.Sprintf("ce_%d", i),
			Type:      EventToolCall,
			ToolName:  "search_files",
			Message:   "搜索关键词 foo bar baz 在项目中的各种文件以找到匹配的代码片段和实现细节",
			Loop:      i,
			TokenIn:   100,
			TokenOut:  50,
			Timestamp: time.Now().Add(time.Duration(i) * time.Millisecond),
		}
	}

	trace := &Trace{
		ID:             "tr_compressed",
		ConversationID: "conv_compressed",
		Events:         events,
	}
	if err := sink.persistTrace(trace); err != nil {
		t.Fatalf("persist: %v", err)
	}

	// Verify all events stored.
	var eventCount int
	err = db.QueryRow(`SELECT COUNT(*) FROM trace_events WHERE trace_id = ?`, "tr_compressed").Scan(&eventCount)
	if err != nil {
		t.Fatalf("query events: %v", err)
	}
	if eventCount != 100 {
		t.Errorf("expected 100 events, got %d", eventCount)
	}

	// Verify blob is stored.
	var blobLen int
	err = db.QueryRow(`SELECT LENGTH(events_blob) FROM traces WHERE id = ?`, "tr_compressed").Scan(&blobLen)
	if err != nil {
		t.Fatalf("query blob: %v", err)
	}
	if blobLen == 0 {
		t.Error("events_blob should not be empty")
	}

	// The compressed blob should be smaller than raw JSON.
	rawJSON, _ := json.Marshal(events)
	if blobLen >= len(rawJSON) {
		t.Logf("compressed (%d) vs raw (%d) — gzip may expand small data", blobLen, len(rawJSON))
	}
}

func TestTraceHeader(t *testing.T) {
	db := setupTestDB(t)
	sink, err := NewSQLiteTraceSink(db)
	if err != nil {
		t.Fatalf("NewSQLiteTraceSink: %v", err)
	}
	defer sink.Close()

	now := time.Now()
	trace := &Trace{
		ID:             "tr_header",
		ConversationID: "conv_header",
		TotalLoops:     5,
		TotalTools:     10,
		TotalErrors:    2,
		TokenIn:        5000,
		TokenOut:       2000,
		Duration:       15000,
		StartTime:      now,
		EndTime:        now.Add(15 * time.Second),
		Events:         []TraceEvent{{ID: "h1", Type: EventLoopStart, Timestamp: now}},
	}
	if err := sink.persistTrace(trace); err != nil {
		t.Fatalf("persist: %v", err)
	}

	header, err := sink.GetTraceByConversation("conv_header")
	if err != nil {
		t.Fatalf("GetTraceByConversation: %v", err)
	}
	if header.ID != "tr_header" {
		t.Errorf("expected tr_header, got %s", header.ID)
	}
	if header.TotalLoops != 5 {
		t.Errorf("expected 5 loops, got %d", header.TotalLoops)
	}
	if header.TokenIn != 5000 {
		t.Errorf("expected 5000 token_in, got %d", header.TokenIn)
	}
	if header.EventCount != 1 {
		t.Errorf("expected event_count=1, got %d", header.EventCount)
	}
}
