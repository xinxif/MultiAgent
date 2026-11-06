// Package mcp MCP（Model Context Protocol）客户端。
//
// MCP 是 Anthropic 提出的工具/资源接入开放协议，2025 年成为行业标准。
// 本文件实现 Streamable HTTP 传输的客户端 —— 一切皆是 JSON-RPC 2.0：
//
//	initialize        → 握手，声明能力
//	tools/list        → 发现服务端提供的工具（含 JSON Schema）
//	tools/call        → 调用工具
//
// MCP 的价值在于"一次实现，处处可用"——
// 我们不写任何具体工具逻辑，只做协议适配，工具生态直接接入。
package mcp

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"agent/internal/pkg/errcode"
)

// jsonRPCRequest JSON-RPC 2.0 请求。
type jsonRPCRequest struct {
	JSONRPC string `json:"jsonrpc"`
	ID      int64  `json:"id"`
	Method  string `json:"method"`
	Params  any    `json:"params,omitempty"`
}

// jsonRPCError JSON-RPC 错误体。
type jsonRPCError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// jsonRPCResponse JSON-RPC 2.0 响应。
type jsonRPCResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      int64           `json:"id"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *jsonRPCError   `json:"error,omitempty"`
}

// Client 单个 MCP Server 的客户端连接。
type Client struct {
	baseURL     string
	headers     map[string]string
	httpClient  *http.Client
	nextID      int64
	sessionID   string // Streamable HTTP 会话（Mcp-Session-Id 头）
	initialized bool
}

// NewClient 构造。
func NewClient(baseURL string, headers map[string]string) *Client {
	return &Client{
		baseURL:    baseURL,
		headers:    headers,
		httpClient: &http.Client{Timeout: 60 * time.Second},
	}
}

// ToolDef MCP 工具定义（协议字段与 OpenAI Function Schema 高度同构）。
type ToolDef struct {
	Name        string         `json:"name"`
	Description string         `json:"description,omitempty"`
	InputSchema map[string]any `json:"inputSchema"`
}

// Initialize 协议握手（惰性：首次调用前自动执行）。
func (c *Client) Initialize(ctx context.Context) error {
	if c.initialized {
		return nil
	}
	var result struct {
		ProtocolVersion string `json:"protocolVersion"`
		ServerInfo      struct {
			Name    string `json:"name"`
			Version string `json:"version"`
		} `json:"serverInfo"`
	}
	if _, err := c.call(ctx, "initialize", map[string]any{
		"protocolVersion": "2025-03-26",
		"capabilities":    map[string]any{},
		"clientInfo":      map[string]any{"name": "agent", "version": "1.0.0"},
	}, &result); err != nil {
		return errcode.ErrMCPConnFail.WithCause(err)
	}
	c.initialized = true
	return nil
}

// ListTools 工具发现。
func (c *Client) ListTools(ctx context.Context) ([]ToolDef, error) {
	if err := c.Initialize(ctx); err != nil {
		return nil, err
	}
	var result struct {
		Tools []ToolDef `json:"tools"`
	}
	if _, err := c.call(ctx, "tools/list", map[string]any{}, &result); err != nil {
		return nil, errcode.ErrMCPConnFail.WithCause(err)
	}
	return result.Tools, nil
}

// CallTool 调用工具。返回结构化内容文本。
func (c *Client) CallTool(ctx context.Context, name string, args map[string]any) (string, error) {
	if err := c.Initialize(ctx); err != nil {
		return "", err
	}
	var result struct {
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
		IsError bool `json:"isError"`
	}
	if _, err := c.call(ctx, "tools/call", map[string]any{"name": name, "arguments": args}, &result); err != nil {
		return "", errcode.ErrMCPToolFail.WithCause(err)
	}
	if result.IsError {
		return "", errcode.ErrMCPToolFail.WithMsg("mcp tool %s failed", name)
	}
	out := ""
	for _, part := range result.Content {
		if part.Type == "text" {
			out += part.Text
		}
	}
	return out, nil
}

// call 发送一次 JSON-RPC 请求（Streamable HTTP：单次 POST）。
func (c *Client) call(ctx context.Context, method string, params any, out any) (json.RawMessage, error) {
	c.nextID++
	reqBody, err := json.Marshal(jsonRPCRequest{
		JSONRPC: "2.0", ID: c.nextID, Method: method, Params: params,
	})
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL, bytes.NewReader(reqBody))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/event-stream")
	for k, v := range c.headers {
		req.Header.Set(k, v)
	}
	if c.sessionID != "" {
		req.Header.Set("Mcp-Session-Id", c.sessionID)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("mcp transport: %w", err)
	}
	defer resp.Body.Close()

	// 会话 ID 由服务端在 initialize 响应头下发
	if sid := resp.Header.Get("Mcp-Session-Id"); sid != "" {
		c.sessionID = sid
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("http %d: %s", resp.StatusCode, truncate(string(body), 256))
	}

	// 响应可能是 JSON 或 SSE 帧（服务端可选流式回包），兼容处理
	payload := extractJSONPayload(body)
	var rpcResp jsonRPCResponse
	if err := json.Unmarshal(payload, &rpcResp); err != nil {
		return nil, fmt.Errorf("decode jsonrpc: %w", err)
	}
	if rpcResp.Error != nil {
		return nil, fmt.Errorf("jsonrpc error %d: %s", rpcResp.Error.Code, rpcResp.Error.Message)
	}
	if out != nil && len(rpcResp.Result) > 0 {
		if err := json.Unmarshal(rpcResp.Result, out); err != nil {
			return nil, fmt.Errorf("decode result: %w", err)
		}
	}
	return rpcResp.Result, nil
}

// extractJSONPayload 从响应体提取 JSON（兼容纯 JSON 与 SSE `data:` 帧）。
func extractJSONPayload(body []byte) []byte {
	s := string(body)
	// SSE 形态：event: message\ndata: {...}
	for _, line := range splitLines(s) {
		if after, ok := cutPrefix(line, "data:"); ok {
			return []byte(after)
		}
	}
	return body
}

func splitLines(s string) []string {
	var out []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '\n' {
			out = append(out, trimCR(s[start:i]))
			start = i + 1
		}
	}
	if start < len(s) {
		out = append(out, trimCR(s[start:]))
	}
	return out
}

func trimCR(s string) string {
	if len(s) > 0 && s[len(s)-1] == '\r' {
		return s[:len(s)-1]
	}
	return s
}

func cutPrefix(s, prefix string) (string, bool) {
	if len(s) > len(prefix) && s[:len(prefix)] == prefix {
		return trimSpace(s[len(prefix):]), true
	}
	return "", false
}

func trimSpace(s string) string {
	for len(s) > 0 && (s[0] == ' ' || s[0] == '\t') {
		s = s[1:]
	}
	return s
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}

// Ping 健康检查（tools/list 可达即视为健康）。
func (c *Client) Ping(ctx context.Context) error {
	_, err := c.ListTools(ctx)
	return err
}
