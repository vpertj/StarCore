package mcp

import (
	"context"
	"encoding/json"
	"fmt"

	"StarCore/internal/mcp/transport"
)

const mcpProtoVersion = "2024-11-05"

type MCPClient struct {
	config      MCPServerConfig
	transport   Transport
	initialized bool
	serverInfo  ServerInfo
	tools       []ToolInfo
	resources   []ResourceInfo
}

type Transport interface {
	Send(method string, params any) (map[string]any, error)
	Close() error
}

func NewClient(config MCPServerConfig) (*MCPClient, error) {
	var t Transport
	var err error

	switch config.Transport {
	case "stdio":
		t, err = transport.NewStdioTransport(config.Command, config.Args, config.Env)
	case "sse":
		t = transport.NewSSETransport(config.Endpoint)
	default:
		return nil, fmt.Errorf("unsupported transport: %s", config.Transport)
	}

	if err != nil {
		return nil, err
	}

	return &MCPClient{
		config:    config,
		transport: t,
	}, nil
}

func (c *MCPClient) Initialize(ctx context.Context) error {
	params := InitializeParams{
		ProtocolVersion: mcpProtoVersion,
		Capabilities: ClientCapabilities{
			Tools:     &struct{}{},
			Resources: &struct{}{},
		},
		ClientInfo: ClientInfo{
			Name:    "StarCore",
			Version: "1.0.0",
		},
	}

	resp, err := c.transport.Send("initialize", params)
	if err != nil {
		return fmt.Errorf("initialize failed: %w", err)
	}

	resultData, _ := json.Marshal(resp["result"])
	var initResult InitializeResult
	if err := json.Unmarshal(resultData, &initResult); err != nil {
		return fmt.Errorf("failed to parse initialize result: %w", err)
	}

	c.serverInfo = initResult.ServerInfo
	c.initialized = true

	c.transport.Send("notifications/initialized", nil)

	c.loadTools(ctx)
	c.loadResources(ctx)

	return nil
}

func (c *MCPClient) loadTools(_ context.Context) {
	resp, err := c.transport.Send("tools/list", nil)
	if err != nil {
		return
	}

	resultData, _ := json.Marshal(resp["result"])
	var result ToolsListResult
	if err := json.Unmarshal(resultData, &result); err != nil {
		return
	}
	c.tools = result.Tools
}

func (c *MCPClient) loadResources(_ context.Context) {
	resp, err := c.transport.Send("resources/list", nil)
	if err != nil {
		return
	}

	resultData, _ := json.Marshal(resp["result"])
	var result ResourcesListResult
	if err := json.Unmarshal(resultData, &result); err != nil {
		return
	}
	c.resources = result.Resources
}

func (c *MCPClient) CallTool(ctx context.Context, name string, args map[string]any) (string, error) {
	_ = ctx
	params := ToolCallParams{Name: name, Arguments: args}
	resp, err := c.transport.Send("tools/call", params)
	if err != nil {
		return "", err
	}

	resultData, _ := json.Marshal(resp["result"])
	var result ToolCallResult
	if err := json.Unmarshal(resultData, &result); err != nil {
		if text, ok := resp["result"].(string); ok {
			return text, nil
		}
		return string(resultData), nil
	}

	var texts []string
	for _, block := range result.Content {
		if block.Type == "text" {
			texts = append(texts, block.Text)
		}
	}
	return fmt.Sprintf("%s", texts), nil
}

func (c *MCPClient) ReadResource(ctx context.Context, uri string) (string, error) {
	_ = ctx
	params := ResourceReadParams{URI: uri}
	resp, err := c.transport.Send("resources/read", params)
	if err != nil {
		return "", err
	}

	resultData, _ := json.Marshal(resp["result"])
	var result ResourceReadResult
	if err := json.Unmarshal(resultData, &result); err != nil {
		return string(resultData), nil
	}

	var texts []string
	for _, content := range result.Contents {
		texts = append(texts, content.Text)
	}
	return fmt.Sprintf("%s", texts), nil
}

func (c *MCPClient) GetTools() []ToolInfo         { return c.tools }
func (c *MCPClient) GetResources() []ResourceInfo { return c.resources }
func (c *MCPClient) GetServerInfo() ServerInfo    { return c.serverInfo }
func (c *MCPClient) GetConfig() MCPServerConfig   { return c.config }

func (c *MCPClient) Close() error {
	return c.transport.Close()
}
