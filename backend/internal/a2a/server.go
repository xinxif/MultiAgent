// A2A 服务端：把本平台的 Agent 暴露为 A2A 可调用节点。
//
// 端点：
//   GET  /.well-known/agent.json   名片自描述
//   POST /a2a                      JSON-RPC（message/send）
//
// 设计：server 包不含 Agent 执行逻辑，通过 Handler 回调注入（service 层装配），
// 保持协议层与业务层解耦。
package a2a

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync/atomic"
	"time"
)

// AgentFunc Agent 执行回调：输入用户文本，返回回答文本。
type AgentFunc func(ctx context.Context, text string) (string, error)

// Server A2A 服务端。
type Server struct {
	card     *AgentCard
	handler  AgentFunc
	nextTask int64
}

// NewServer 构造。
func NewServer(card *AgentCard, handler AgentFunc) *Server {
	return &Server{card: card, handler: handler}
}

// CardHandler GET /.well-known/agent.json
func (s *Server) CardHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(s.card)
}

// RPCHandler POST /a2a —— JSON-RPC 2.0 分发。
func (s *Server) RPCHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		JSONRPC string          `json:"jsonrpc"`
		ID      json.RawMessage `json:"id"`
		Method  string          `json:"method"`
		Params  json.RawMessage `json:"params"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeRPCError(w, nil, -32700, "parse error")
		return
	}

	switch req.Method {
	case MethodMessageSend:
		var p SendMessageParams
		if err := json.Unmarshal(req.Params, &p); err != nil {
			writeRPCError(w, req.ID, -32602, "invalid params")
			return
		}
		text := ""
		for _, part := range p.Message.Parts {
			if part.Type == "text" {
				text += part.Text
			}
		}

		answer, err := s.handler(r.Context(), text)
		if err != nil {
			task := Task{
				ID:    s.newTaskID(),
				State: StateFailed,
			}
			writeRPCResult(w, req.ID, task, err.Error())
			return
		}
		task := Task{
			ID:    s.newTaskID(),
			State: StateCompleted,
			Messages: []Message{{
				Role: RoleAgent,
				Parts: []Part{{Type: "text", Text: answer}},
			}},
			Artifacts: []Artifact{{
				Name:  "answer",
				Parts: []Part{{Type: "text", Text: answer}},
			}},
		}
		writeRPCResult(w, req.ID, task, "")

	case MethodMessageProbe:
		writeRPCResult(w, req.ID, map[string]any{"status": "alive", "time": time.Now()}, "")

	default:
		writeRPCError(w, req.ID, -32601, "method not found: "+req.Method)
	}
}

func (s *Server) newTaskID() string {
	n := atomic.AddInt64(&s.nextTask, 1)
	return fmt.Sprintf("task-%d-%d", time.Now().UnixMilli(), n)
}

// writeRPCResult 成功响应。
func writeRPCResult(w http.ResponseWriter, id json.RawMessage, result any, errMsg string) {
	resp := map[string]any{
		"jsonrpc": "2.0",
		"id":      id,
		"result":  result,
	}
	if errMsg != "" {
		resp["error"] = map[string]any{"code": -32000, "message": errMsg}
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}

// writeRPCError 错误响应。
func writeRPCError(w http.ResponseWriter, id json.RawMessage, code int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"jsonrpc": "2.0",
		"id":      id,
		"error":   map[string]any{"code": code, "message": msg},
	})
}

// RegisterRoutes 把 A2A 端点挂到标准 http mux（主服务复用同一 mux 或独立端口均可）。
func (s *Server) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/.well-known/agent.json", s.CardHandler)
	mux.HandleFunc("/a2a", s.RPCHandler)
}
