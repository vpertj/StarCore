package tools

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
	"unicode"

	"StarCore/internal/agent"
	"StarCore/internal/sandbox"

	"golang.org/x/net/html"
)

const webFetchMaxChars = 8000

type WebFetchTool struct{}

func NewWebFetchTool() *WebFetchTool { return &WebFetchTool{} }

func (t *WebFetchTool) ID() string             { return "web_fetch" }
func (t *WebFetchTool) Name() string           { return "Web Fetch" }
func (t *WebFetchTool) RequiresApproval() bool { return false }

func (t *WebFetchTool) Description() string {
	return "抓取网页内容并提取文本。用于读取文档、检查网页。"
}

func (t *WebFetchTool) Parameters() agent.ToolParameters {
	return agent.ToolParameters{
		Type: "object",
		Properties: map[string]agent.ToolParamProp{
			"url": {Type: "string", Description: "Full URL to fetch (e.g. https://example.com/docs)"},
		},
		Required: []string{"url"},
	}
}

func (t *WebFetchTool) Execute(ctx context.Context, args map[string]any) (string, error) {
	url, _ := args["url"].(string)
	url = strings.TrimSpace(url)
	if url == "" {
		return "", fmt.Errorf("url is required")
	}

	if err := sandbox.ValidateURL(url); err != nil {
		return "", fmt.Errorf("URL validation failed: %w", err)
	}

	client := &http.Client{Timeout: 15 * time.Second}
	httpReq, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}
	httpReq.Header.Set("User-Agent", "StarCore/1.0")

	resp, err := client.Do(httpReq)
	if err != nil {
		return "", fmt.Errorf("fetch failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 500))
		return "", fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(body))
	}

	contentType := resp.Header.Get("Content-Type")
	if !strings.Contains(contentType, "text/html") && !strings.Contains(contentType, "text/plain") {
		// Non-HTML content: return raw text
		body, _ := io.ReadAll(io.LimitReader(resp.Body, int64(webFetchMaxChars)))
		return string(body), nil
	}

	// Parse HTML and extract text
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20)) // 1MB max
	text := extractText(body)
	if len(text) > webFetchMaxChars {
		text = text[:webFetchMaxChars] + "\n... [truncated]"
	}
	if text == "" {
		return "(page loaded but no text content extracted)", nil
	}
	return text, nil
}

func extractText(htmlBytes []byte) string {
	doc, err := html.Parse(bytes.NewReader(htmlBytes))
	if err != nil {
		return string(htmlBytes)
	}

	var sb strings.Builder
	var extract func(*html.Node)
	skipTags := map[string]bool{"script": true, "style": true, "nav": true, "footer": true, "header": true, "noscript": true}

	extract = func(n *html.Node) {
		if n.Type == html.ElementNode && skipTags[n.Data] {
			return
		}
		if n.Type == html.TextNode {
			t := strings.TrimSpace(n.Data)
			if t != "" {
				sb.WriteString(t)
				sb.WriteString(" ")
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			extract(c)
		}
		if n.Type == html.ElementNode && (n.Data == "p" || n.Data == "br" || n.Data == "h1" || n.Data == "h2" || n.Data == "h3" || n.Data == "h4" || n.Data == "li" || n.Data == "div" || n.Data == "section") {
			sb.WriteString("\n")
		}
	}
	extract(doc)

	// Collapse whitespace
	result := strings.Join(strings.Fields(sb.String()), " ")
	// Restore paragraph breaks
	result = strings.ReplaceAll(result, " \n ", "\n")
	result = strings.ReplaceAll(result, " \n", "\n")

	// Remove non-printable chars except newlines
	result = strings.Map(func(r rune) rune {
		if r == '\n' || unicode.IsPrint(r) {
			return r
		}
		return -1
	}, result)

	return result
}
