// Package sse SSE 事件编码器：将内部 AgentEvent 统一转写为 text/event-stream 格式。
// 事件协议：meta/delta/reasoning/plan/tool_call/tool_result/
// agent_event/refs/usage/approval/done/error
package sse

import (
	"encoding/json"
	"fmt"
	"io"
)

// Event 一条 SSE 事件。
type Event struct {
	Event string `json:"event"` // 事件类型
	Data  any    `json:"data"`  // 任意负载
}

// Write 将事件按 SSE 格式写入 writer。
//
//	格式：
//	event: delta
//	data: {"content":"..."}
//
//	(空行分隔)
func Write(w io.Writer, event string, data any) error {
	payload, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("sse marshal: %w", err)
	}
	if _, err := fmt.Fprintf(w, "event: %s\ndata: %s\n\n", event, payload); err != nil {
		return fmt.Errorf("sse write: %w", err)
	}
	return nil
}

// WriteString 写入原始字符串负载（不转义）。
func WriteString(w io.Writer, event, raw string) error {
	_, err := fmt.Fprintf(w, "event: %s\ndata: %s\n\n", event, raw)
	return err
}

// Ping 写入注释行心跳，防止代理超时断连。
func Ping(w io.Writer) error {
	_, err := fmt.Fprint(w, ": ping\n\n")
	return err
}
