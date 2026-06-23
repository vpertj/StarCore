package tools

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"StarCore/internal/agent"
	"StarCore/internal/sandbox"
)

const httpMaxBody = 5000

type HTTPRequestTool struct{}

func NewHTTPRequestTool() *HTTPRequestTool { return &HTTPRequestTool{} }

func (t *HTTPRequestTool) ID() string             { return "http_request" }
func (t *HTTPRequestTool) Name() string           { return "HTTP Request" }
func (t *HTTPRequestTool) RequiresApproval() bool { return true }

func (t *HTTPRequestTool) Description() string {
	return "Make an HTTP request to test APIs or fetch web resources. " +
		"Supports GET, POST, PUT, DELETE. Returns status code, headers, and body. " +
		"Use this for API testing, endpoint verification, or checking web services."
}

func (t *HTTPRequestTool) Parameters() agent.ToolParameters {
	return agent.ToolParameters{
		Type: "object",
		Properties: map[string]agent.ToolParamProp{
			"url":     {Type: "string", Description: "Full URL including protocol (e.g. https://api.example.com/users)"},
			"method":  {Type: "string", Description: "HTTP method: GET, POST, PUT, DELETE (default GET)"},
			"headers": {Type: "string", Description: "Optional JSON object of headers (e.g. {\"Authorization\":\"Bearer xxx\"})"},
			"body":    {Type: "string", Description: "Optional request body for POST/PUT"},
		},
		Required: []string{"url"},
	}
}

func (t *HTTPRequestTool) Execute(ctx context.Context, args map[string]any) (string, error) {
	url, _ := args["url"].(string)
	url = strings.TrimSpace(url)
	if url == "" {
		return "", fmt.Errorf("url is required")
	}

	if err := sandbox.ValidateURL(url); err != nil {
		return "", fmt.Errorf("SSRF protection: %w", err)
	}

	method, _ := args["method"].(string)
	if method == "" {
		method = "GET"
	}
	method = strings.ToUpper(method)

	var reqBody io.Reader
	if body, ok := args["body"].(string); ok && body != "" {
		reqBody = bytes.NewReader([]byte(body))
	}

	client := &http.Client{Timeout: 15 * time.Second}
	httpReq, err := http.NewRequestWithContext(ctx, method, url, reqBody)
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("User-Agent", "StarCore/1.0")
	httpReq.Header.Set("Accept", "*/*")

	if headers, ok := args["headers"].(string); ok && headers != "" {
		headers = strings.TrimSpace(headers)
		if strings.HasPrefix(headers, "{") {
			var headerMap map[string]string
			if json.Unmarshal([]byte(headers), &headerMap) == nil {
				for k, v := range headerMap {
					httpReq.Header.Set(k, v)
				}
			}
		} else {
			for _, line := range strings.Split(headers, "\n") {
				line = strings.TrimSpace(line)
				if line == "" {
					continue
				}
				parts := strings.SplitN(line, ":", 2)
				if len(parts) == 2 {
					httpReq.Header.Set(strings.TrimSpace(parts[0]), strings.TrimSpace(parts[1]))
				}
			}
		}
	}

	resp, err := client.Do(httpReq)
	if err != nil {
		return "", fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(io.LimitReader(resp.Body, int64(httpMaxBody)))
	if err != nil {
		return "", fmt.Errorf("failed to read response: %w", err)
	}
	bodyStr := string(bodyBytes)
	if len(bodyStr) > httpMaxBody {
		bodyStr = bodyStr[:httpMaxBody] + "\n... [truncated]"
	}

	var result strings.Builder
	result.WriteString(fmt.Sprintf("Status: %d %s\n", resp.StatusCode, resp.Status))

	// Show key response headers
	keyHeaders := []string{"Content-Type", "Content-Length", "Location", "Set-Cookie", "X-Request-Id"}
	for _, h := range keyHeaders {
		if v := resp.Header.Get(h); v != "" {
			result.WriteString(fmt.Sprintf("%s: %s\n", h, v))
		}
	}

	result.WriteString(fmt.Sprintf("\nBody:\n%s", bodyStr))
	return result.String(), nil
}
