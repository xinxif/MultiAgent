// 对话接口：SSE 流式对话 / 会话与消息管理。
//
// SSE 事件序列（event: 名 + data: JSON）：
//
//	meta → (reasoning/plan/tool_call/tool_result/agent_event/refs/usage)*
//	     → delta* → done | error
//
// handler 只做"事件流转写"—— Channel ↔ SSE 的桥接，
// 组装/记忆/工具循环全部在 service 与 agent 层。
package api

import (
	"context"
	"encoding/json"
	"io"

	"github.com/cloudwego/hertz/pkg/app"

	"agent/internal/service"
)

// ChatReq 对话请求。
type ChatReq struct {
	SessionID string `json:"session_id"`
	AgentID   int64  `json:"agent_id"`
	Content   string `json:"content"`
	// 本次发送使用的模型（对话页选择器）；0/未传 = 沿用 Agent 绑定模型
	ModelConfigID int64 `json:"model_config_id"`
}

// Chat POST /api/v1/chat/stream —— SSE 流式对话。
func (h *Handler) Chat(ctx context.Context, c *app.RequestContext) {
	var req ChatReq
	if err := c.BindAndValidate(&req); err != nil {
		Fail(c, err)
		return
	}

	events, err := h.svcs.Chat.Stream(ctx, userID(c), &service.ChatInput{
		SessionID:     req.SessionID,
		AgentID:       req.AgentID,
		Content:       req.Content,
		ModelConfigID: req.ModelConfigID,
	})
	if err != nil {
		Fail(c, err)
		return
	}

	// 流式响应：把事件 Channel 转写为 SSE。
	// SetBodyStream(reader, -1) = 未知长度的 chunked 流式响应。
	pr, pw := io.Pipe()
	c.Response.Header.Set("Content-Type", "text/event-stream")
	c.Response.Header.Set("Cache-Control", "no-cache")
	c.Response.Header.Set("Connection", "keep-alive")
	c.Response.Header.Set("X-Accel-Buffering", "no") // Nginx 不缓冲
	c.Response.SetBodyStream(pr, -1)

	// SSE 是长连接：handler 在这里就返回了，但事件流还要写很久。
	// 所以从 Tracing 中间件手里接管根 span，等流真正写完再结束它，
	// 否则 Jaeger 里会看到"根 span 只有几毫秒、子 span 却好几秒"。
	streamSpan := TakeRootSpan(c)

	go func() {
		defer pw.Close()
		if streamSpan != nil {
			defer streamSpan.End()
		}
		for ev := range events {
			data, err := json.Marshal(ev.Data)
			if err != nil {
				continue
			}
			if _, werr := writeSSE(pw, ev.Type, data); werr != nil {
				return // 客户端断开
			}
		}
	}()
}

// writeSSE 写一条 SSE 事件。
func writeSSE(w io.Writer, event string, data []byte) (int, error) {
	return io.WriteString(w, "event: "+event+"\ndata: "+string(data)+"\n\n")
}

// ListSessions GET /api/v1/sessions —— 会话列表。
func (h *Handler) ListSessions(ctx context.Context, c *app.RequestContext) {
	q := parsePagination(c)
	list, total, err := h.svcs.Repos.Session.ListSessions(ctx, userID(c), q.Offset(), q.Limit())
	if err != nil {
		Fail(c, err)
		return
	}
	OK(c, map[string]any{"list": list, "total": total})
}

// ListMessages GET /api/v1/sessions/:id/messages —— 会话消息（含决策链 metadata）。
func (h *Handler) ListMessages(ctx context.Context, c *app.RequestContext) {
	sid := c.Param("id")
	session, err := h.svcs.Repos.Session.GetSession(ctx, sid)
	if err != nil {
		Fail(c, err)
		return
	}
	if session.UserID != userID(c) {
		forbidden(c)
		return
	}
	msgs, err := h.svcs.Repos.Session.ListMessages(ctx, sid, 200)
	if err != nil {
		Fail(c, err)
		return
	}
	OK(c, msgs)
}

// RenameSession PUT /api/v1/sessions/:id —— 会话改名。
func (h *Handler) RenameSession(ctx context.Context, c *app.RequestContext) {
	sid := c.Param("id")
	session, err := h.svcs.Repos.Session.GetSession(ctx, sid)
	if err != nil {
		Fail(c, err)
		return
	}
	if session.UserID != userID(c) {
		forbidden(c)
		return
	}
	var req struct {
		Title string `json:"title"`
	}
	if err := c.BindAndValidate(&req); err != nil {
		Fail(c, err)
		return
	}
	if err := h.svcs.Repos.Session.RetitleSession(ctx, sid, req.Title); err != nil {
		Fail(c, err)
		return
	}
	OK(c, nil)
}

// DeleteSession DELETE /api/v1/sessions/:id
func (h *Handler) DeleteSession(ctx context.Context, c *app.RequestContext) {
	sid := c.Param("id")
	session, err := h.svcs.Repos.Session.GetSession(ctx, sid)
	if err != nil {
		Fail(c, err)
		return
	}
	if session.UserID != userID(c) {
		forbidden(c)
		return
	}
	if err := h.svcs.Repos.Session.DeleteSession(ctx, sid); err != nil {
		Fail(c, err)
		return
	}
	OK(c, nil)
}
