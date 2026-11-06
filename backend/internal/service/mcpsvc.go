// MCP 服务：Server 注册管理 / 工具同步 / 健康检查。
//
// MCP 的接入是一次"协议适配"而非"工具开发"：
//
//	注册 Server（URL + 鉴权头） → tools/list 发现工具
//	  → 工具落库（type=mcp，code = mcp_{serverID}_{name}）
//	  → handler 闭包注册进 Registry（执行时 tools/call 透传）
//
// 同步后的 MCP 工具与内置工具在 Agent 视角完全一致 ——
// ReAct 循环、工具执行器、决策链都不感知差异，这就是协议标准的意义。
package service

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"agent/internal/mcp"
	"agent/internal/model"
	"agent/internal/pkg/errcode"
	"agent/internal/tool"
)

// MCPService MCP 服务端管理业务。
type MCPService struct {
	svcs     *Services
	registry tool.Registry // 工具同步时注册执行 handler（main 注入）
}

// NewMCPService 构造。
func NewMCPService(svcs *Services) *MCPService {
	return &MCPService{svcs: svcs}
}

// SetRegistry 注入工具注册表。
func (s *MCPService) SetRegistry(reg tool.Registry) { s.registry = reg }

// Create 注册 MCP 服务端。
func (s *MCPService) Create(ctx context.Context, srv *model.MCPServer) error {
	if srv.Name == "" || srv.URL == "" {
		return errcode.ErrInvalidParam.WithMsg("name 和 url 不能为空")
	}
	if srv.Transport == "" {
		srv.Transport = "streamable_http" // 当前仅实现 Streamable HTTP 传输
	}
	if err := s.svcs.Deps.Repos.MCP.Create(ctx, srv); err != nil {
		return errcode.ErrInternal.WithCause(err)
	}
	return nil
}

// Update 更新配置（URL/鉴权头变更后需重新同步工具）。
func (s *MCPService) Update(ctx context.Context, srv *model.MCPServer) error {
	if srv.ID <= 0 {
		return errcode.ErrInvalidParam.WithMsg("id 不能为空")
	}
	if _, err := s.get(ctx, srv.ID); err != nil {
		return err
	}
	return s.svcs.Deps.Repos.MCP.Update(ctx, srv)
}

// Delete 删除服务端及其同步的全部工具。
func (s *MCPService) Delete(ctx context.Context, id int64) error {
	if _, err := s.get(ctx, id); err != nil {
		return err
	}
	if err := s.svcs.Deps.Repos.MCP.DeleteToolsByServer(ctx, id); err != nil {
		return errcode.ErrInternal.WithCause(err)
	}
	return s.svcs.Deps.Repos.MCP.Delete(ctx, id)
}

// List 服务端列表。
func (s *MCPService) List(ctx context.Context) ([]model.MCPServer, error) {
	list, err := s.svcs.Deps.Repos.MCP.List(ctx)
	if err != nil {
		return nil, errcode.ErrInternal.WithCause(err)
	}
	return list, nil
}

// get 详情（统一 NotFound 转换）。
func (s *MCPService) get(ctx context.Context, id int64) (*model.MCPServer, error) {
	srv, err := s.svcs.Deps.Repos.MCP.Get(ctx, id)
	if err != nil {
		return nil, errcode.ErrMCPConnFail.WithMsg("MCP 服务端不存在: %d", id)
	}
	return srv, nil
}

// Get 详情。
func (s *MCPService) Get(ctx context.Context, id int64) (*model.MCPServer, error) {
	return s.get(ctx, id)
}

// SyncTools 工具同步（本服务核心）：
//
//	① 建连握手 → ② tools/list 发现 → ③ 全量替换工具表
//	④ 逐个注册执行 handler → ⑤ 状态/缓存回写。
func (s *MCPService) SyncTools(ctx context.Context, serverID int64) ([]model.Tool, error) {
	srv, err := s.get(ctx, serverID)
	if err != nil {
		return nil, err
	}

	// ① 建连（headers 从 JSONB 解析，鉴权信息不进日志）
	client := mcp.NewClient(srv.URL, parseHeaders(srv.Headers))

	// ② 工具发现
	defs, err := client.ListTools(ctx)
	if err != nil {
		s.markStatus(ctx, srv, "error")
		return nil, errcode.ErrMCPConnFail.WithCause(err)
	}

	// ③ 全量替换：先删旧（避免下线工具残留），再建新
	repo := s.svcs.Deps.Repos
	if err := repo.MCP.DeleteToolsByServer(ctx, srv.ID); err != nil {
		return nil, errcode.ErrInternal.WithCause(err)
	}
	tools := make([]model.Tool, 0, len(defs))
	for _, d := range defs {
		params, _ := json.Marshal(schemaOrEmpty(d.InputSchema))
		t := model.Tool{
			Code:        mcpToolCode(srv.ID, d.Name),
			Name:        d.Name,
			Type:        model.ToolTypeMCP,
			Description: d.Description,
			Parameters:  params,
			MCPServerID: &srv.ID,
			TimeoutMs:   60000,
			Status:      1,
		}
		if err := repo.Tool.Create(ctx, &t); err != nil {
			return nil, errcode.ErrInternal.WithCause(err)
		}
		tools = append(tools, t)

		// ④ 注册执行 handler：闭包捕获 serverID + 远端工具名
		s.registerHandler(srv.ID, d.Name)
	}

	// ⑤ 状态与缓存回写
	now := time.Now()
	srv.Status = "connected"
	srv.LastHealthAt = &now
	if cache, err := json.Marshal(defs); err == nil {
		srv.ToolsCache = cache
	}
	if err := repo.MCP.Update(ctx, srv); err != nil {
		return nil, errcode.ErrInternal.WithCause(err)
	}
	return tools, nil
}

// CallTool 直接调用某服务端工具（管理台调试用；Agent 链路走 Executor）。
func (s *MCPService) CallTool(ctx context.Context, serverID int64, name string, args map[string]any) (string, error) {
	srv, err := s.get(ctx, serverID)
	if err != nil {
		return "", err
	}
	client := mcp.NewClient(srv.URL, parseHeaders(srv.Headers))
	out, err := client.CallTool(ctx, name, args)
	if err != nil {
		return "", errcode.ErrMCPToolFail.WithCause(err)
	}
	return out, nil
}

// HealthCheck 健康检查：Ping 成功即视为连接正常。
func (s *MCPService) HealthCheck(ctx context.Context, serverID int64) error {
	srv, err := s.get(ctx, serverID)
	if err != nil {
		return err
	}
	client := mcp.NewClient(srv.URL, parseHeaders(srv.Headers))
	if err := client.Ping(ctx); err != nil {
		s.markStatus(ctx, srv, "error")
		return errcode.ErrMCPConnFail.WithCause(err)
	}
	s.markStatus(ctx, srv, "connected")
	return nil
}

// registerHandler 把 MCP 工具注册为普通 BuiltinHandler。
// handler 闭包只存 serverID/远端工具名，执行时再查库取最新配置 ——
// 服务端 URL 或鉴权变更后无需重新注册。
func (s *MCPService) registerHandler(serverID int64, remoteName string) {
	if s.registry == nil {
		return
	}
	code := mcpToolCode(serverID, remoteName)
	repo := s.svcs.Deps.Repos.MCP
	s.registry.Register(code, func(ctx context.Context, args map[string]any, _ string) (string, error) {
		srv, err := repo.Get(ctx, serverID)
		if err != nil {
			return "", fmt.Errorf("mcp server %d unavailable: %w", serverID, err)
		}
		client := mcp.NewClient(srv.URL, parseHeaders(srv.Headers))
		return client.CallTool(ctx, remoteName, args)
	})
}

// markStatus 回写服务端健康状态（失败不阻塞主流程）。
func (s *MCPService) markStatus(ctx context.Context, srv *model.MCPServer, status string) {
	now := time.Now()
	srv.Status = status
	srv.LastHealthAt = &now
	_ = s.svcs.Deps.Repos.MCP.Update(ctx, srv)
}

// mcpToolCode 平台侧工具唯一 code：mcp_{serverID}_{远端工具名}。
// serverID 保证跨服务端同名工具不冲突。
func mcpToolCode(serverID int64, remoteName string) string {
	return fmt.Sprintf("mcp_%d_%s", serverID, remoteName)
}

// parseHeaders 服务端鉴权头 JSONB → map。
func parseHeaders(raw []byte) map[string]string {
	out := map[string]string{}
	if len(raw) > 0 {
		_ = json.Unmarshal(raw, &out)
	}
	return out
}

// schemaOrEmpty 兜底空 Schema（部分 MCP 服务端不返回 inputSchema）。
func schemaOrEmpty(m map[string]any) map[string]any {
	if m == nil {
		return map[string]any{"type": "object", "properties": map[string]any{}}
	}
	return m
}
