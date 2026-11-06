// A2A 客户端：获取远程 Agent 名片 + 发起 JSON-RPC 任务。
package a2a

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"agent/internal/pkg/errcode"
)

// Client A2A 远程 Agent 客户端。
type Client struct {
	httpClient *http.Client
}

// NewClient 构造。
func NewClient() *Client {
	return &Client{httpClient: &http.Client{Timeout: 120 * time.Second}}
}

// FetchCard 拉取远程 Agent 名片（注册/健康检查时调用）。
func (c *Client) FetchCard(ctx context.Context, baseURL string) (*AgentCard, error) {
	url := strings.TrimRight(baseURL, "/") + "/.well-known/agent.json"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, errcode.ErrA2ACardFetch.WithCause(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, errcode.ErrA2ACardFetch.WithMsg("card http %d", resp.StatusCode)
	}
	var card AgentCard
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&card); err != nil {
		return nil, errcode.ErrA2ACardFetch.WithCause(err)
	}
	return &card, nil
}

// SendMessage 向远程 Agent 发送任务（message/send）。
func (c *Client) SendMessage(ctx context.Context, baseURL string, text string) (*Task, error) {
	endpoint := strings.TrimRight(baseURL, "/") + "/"
	if card, err := c.FetchCard(ctx, baseURL); err == nil && card.URL != "" {
		endpoint = card.URL // 名片中的端点优先
	}

	params := SendMessageParams{
		Message: Message{
			Role: RoleUser,
			Parts: []Part{{Type: "text", Text: text}},
		},
	}
	raw, err := json.Marshal(map[string]any{
		"jsonrpc": "2.0", "id": time.Now().UnixNano(),
		"method": MethodMessageSend, "params": params,
	})
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(raw))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, errcode.ErrA2ATaskFail.WithCause(err)
	}
	defer resp.Body.Close()

	var rpcResp struct {
		Result *Task          `json:"result"`
		Error  *struct {
			Code    int    `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 8<<20)).Decode(&rpcResp); err != nil {
		return nil, errcode.ErrA2ATaskFail.WithCause(err)
	}
	if rpcResp.Error != nil {
		return nil, errcode.ErrA2ATaskFail.WithMsg("remote: %s", rpcResp.Error.Message)
	}
	if rpcResp.Result == nil {
		return nil, errcode.ErrA2ATaskFail.WithMsg("empty task result")
	}
	return rpcResp.Result, nil
}

// ExtractText 从任务产出中提取全部文本。
func ExtractText(t *Task) string {
	var b strings.Builder
	for _, art := range t.Artifacts {
		for _, p := range art.Parts {
			if p.Type == "text" {
				b.WriteString(p.Text)
			}
		}
	}
	if b.Len() == 0 {
		return fmt.Sprintf("(task %s in state %s, no artifacts)", t.ID, t.State)
	}
	return b.String()
}
