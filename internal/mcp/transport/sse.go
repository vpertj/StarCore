package transport

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"
)

// SSETransport communicates with an MCP server over HTTP POST.
// For servers that support SSE, a full-duplex streaming implementation
// would be needed (persistent GET connection for inbound events).
// This transport uses individual POST requests for each call, which works
// for the standard JSON-RPC request/response pattern used by most MCP servers.
type SSETransport struct {
	endpoint string
	client   *http.Client
	mu       sync.Mutex
	nextID   int
}

func NewSSETransport(endpoint string) *SSETransport {
	return &SSETransport{
		endpoint: endpoint,
		client:   &http.Client{Timeout: 60 * time.Second},
	}
}

func (t *SSETransport) Send(method string, params any) (map[string]any, error) {
	t.mu.Lock()
	id := t.nextID
	t.nextID++
	t.mu.Unlock()

	req := map[string]any{
		"jsonrpc": "2.0",
		"id":      id,
		"method":  method,
	}
	if params != nil {
		req["params"] = params
	}

	data, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}

	resp, err := t.client.Post(t.endpoint, "application/json", bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("sse transport: request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("sse transport: HTTP %d", resp.StatusCode)
	}

	var result map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("sse transport: decode failed: %w", err)
	}
	return result, nil
}

func (t *SSETransport) Close() error {
	t.client.CloseIdleConnections()
	return nil
}
