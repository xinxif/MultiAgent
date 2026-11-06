/**
 * 集成中心 —— 「MCP Server」与「A2A 远程 Agent」两个区块上下排列。
 *
 * 职责：
 * 1. MCP Server：CRUD（args/env/headers 等 JSON 字段提交前解析校验）、同步工具、
 *    健康检查，以及基于 tools_cache 的工具手动调试；
 * 2. A2A 远程 Agent：注册（后端先拉名片验活）、健康检查、任务委派与删除；
 * 3. 对 tools_cache / card 等后端结构做宽容处理，字段缺失或结构变化时降级展示。
 */
import { useEffect, useRef, useState } from 'react';
import {
  callMCPTool,
  checkA2AAgent,
  checkMCPServer,
  createMCPServer,
  deleteA2AAgent,
  deleteMCPServer,
  listA2AAgents,
  listMCPServers,
  registerA2AAgent,
  sendA2ATask,
  syncMCPServer,
  updateMCPServer,
} from '@/api/admin';
import { ConfirmDialog } from '@/components/ui/ConfirmDialog';
import { Modal } from '@/components/ui/Modal';
import {
  Badge,
  Button,
  EmptyState,
  Input,
  PageHeader,
  Select,
  Spinner,
  StatusBadge,
  Textarea,
} from '@/components/ui/primitives';
import { relativeTime, truncate } from '@/lib/format';
import { staggerEnter } from '@/lib/gsap';
import { errMsg } from '@/lib/request';
import { toast } from '@/stores/ui';
import type { A2AAgent, MCPServer } from '@/types/admin';

/* ---- 常量与工具 ---- */

/** 传输方式 → 徽标色调（未知传输回退灰色） */
const TRANSPORT_TONE: Record<string, 'lime' | 'teal' | 'violet'> = {
  streamable_http: 'lime',
  stdio: 'teal',
  sse: 'violet',
};

/** 传输方式选项（与后端 MCPServer.transport 枚举一致） */
const TRANSPORTS = ['streamable_http', 'stdio', 'sse'] as const;

/** 解析 JSON 文本：空串回退默认值，非法返回 null（由调用方 toast 阻断提交） */
function parseJson<T>(text: string, fallback: T): T | null {
  const t = text.trim();
  if (!t) return fallback;
  try {
    return JSON.parse(t) as T;
  } catch {
    return null;
  }
}

/** 结果输出统一截断，避免超长响应撑爆弹窗 */
const clip = (s: string) => truncate(s, 2000);

/* ---- 表单草稿类型 ---- */

/** MCP 表单草稿：三个 JSON 字段以文本承载，提交时统一解析校验 */
interface McpDraft {
  name: string;
  transport: string;
  url: string;
  command: string;
  args: string;
  env: string;
  headers: string;
}

const EMPTY_MCP: McpDraft = {
  name: '',
  transport: 'streamable_http',
  url: '',
  command: '',
  args: '[]',
  env: '{}',
  headers: '{}',
};

export function IntegrationsPage() {
  /* ---- 列表数据与全局状态 ---- */
  const [servers, setServers] = useState<MCPServer[]>([]);
  const [agents, setAgents] = useState<A2AAgent[]>([]);
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const [deleting, setDeleting] = useState(false);
  /** 行内动作（同步 / 健康检查）进行中的目标 id，用于禁用对应按钮 */
  const [busyId, setBusyId] = useState<number | null>(null);

  /* ---- MCP 表单 ---- */
  const [mcpOpen, setMcpOpen] = useState(false);
  const [editingMcp, setEditingMcp] = useState<MCPServer | null>(null);
  const [mcpDraft, setMcpDraft] = useState<McpDraft>(EMPTY_MCP);

  /* ---- MCP 工具调试 ---- */
  const [debugServer, setDebugServer] = useState<MCPServer | null>(null);
  const [debugTool, setDebugTool] = useState('');
  const [debugArgs, setDebugArgs] = useState('{}');
  const [debugOutput, setDebugOutput] = useState('');
  const [debugRunning, setDebugRunning] = useState(false);

  /* ---- A2A 注册 ---- */
  const [registerOpen, setRegisterOpen] = useState(false);
  const [regDraft, setRegDraft] = useState({ base_url: '', name: '' });

  /* ---- A2A 委派任务 ---- */
  const [taskAgent, setTaskAgent] = useState<A2AAgent | null>(null);
  const [taskText, setTaskText] = useState('');
  const [taskResult, setTaskResult] = useState('');
  const [taskRunning, setTaskRunning] = useState(false);

  /** 删除目标：kind 区分区块，label 用于确认文案 */
  const [delTarget, setDelTarget] = useState<{
    kind: 'mcp' | 'a2a';
    id: number;
    label: string;
  } | null>(null);

  const rootRef = useRef<HTMLDivElement>(null);

  /* ---- 数据加载 ---- */

  /** 并行拉取 MCP Server + A2A Agent；写操作成功后复用整体刷新 */
  const load = async () => {
    setLoading(true);
    try {
      const [serverList, agentList] = await Promise.all([listMCPServers(), listA2AAgents()]);
      setServers(serverList);
      setAgents(agentList);
    } catch (e) {
      toast.err(errMsg(e));
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    void load();
  }, []);

  /** 首屏数据就绪后两个区块交错入场（仅首次，后续刷新不重播） */
  const playedRef = useRef(false);
  useEffect(() => {
    if (loading || playedRef.current) return;
    playedRef.current = true;
    const sections = rootRef.current?.querySelectorAll('section');
    if (sections && sections.length > 0) staggerEnter(Array.from(sections));
  }, [loading]);

  /* ---- MCP：新增 / 编辑 ---- */

  const patchMcp = (patch: Partial<McpDraft>) => setMcpDraft((d) => ({ ...d, ...patch }));

  const openMcpCreate = () => {
    setEditingMcp(null);
    setMcpDraft(EMPTY_MCP);
    setMcpOpen(true);
  };

  const openMcpEdit = (s: MCPServer) => {
    setEditingMcp(s);
    // JSON 字段序列化为缩进文本便于编辑；结构异常时回退默认值
    setMcpDraft({
      name: s.name,
      transport: s.transport,
      url: s.url,
      command: s.command,
      args: JSON.stringify(Array.isArray(s.args) ? s.args : [], null, 2),
      env: JSON.stringify(s.env ?? {}, null, 2),
      headers: JSON.stringify(s.headers ?? {}, null, 2),
    });
    setMcpOpen(true);
  };

  const submitMcp = async () => {
    if (!mcpDraft.name.trim()) {
      toast.err('名称为必填项');
      return;
    }
    // JSON 字段提交前统一解析，任一非法即阻断（args=字符串数组，env/headers=对象）
    const args = parseJson<string[]>(mcpDraft.args, []);
    if (!args || !Array.isArray(args)) {
      toast.err('args 不是合法的 JSON 数组');
      return;
    }
    const env = parseJson<Record<string, string>>(mcpDraft.env, {});
    if (!env) {
      toast.err('env 不是合法的 JSON 对象');
      return;
    }
    const headers = parseJson<Record<string, string>>(mcpDraft.headers, {});
    if (!headers) {
      toast.err('headers 不是合法的 JSON 对象');
      return;
    }
    setSaving(true);
    try {
      const payload: Partial<MCPServer> = {
        name: mcpDraft.name.trim(),
        transport: mcpDraft.transport,
        url: mcpDraft.url.trim(),
        command: mcpDraft.command.trim(),
        args,
        env,
        headers,
      };
      if (editingMcp) {
        await updateMCPServer(editingMcp.id, payload);
        toast.ok('MCP Server 已更新');
      } else {
        await createMCPServer(payload);
        toast.ok('MCP Server 已创建');
      }
      setMcpOpen(false);
      await load();
    } catch (e) {
      toast.err(errMsg(e));
    } finally {
      setSaving(false);
    }
  };

  /* ---- MCP：行内动作 ---- */

  /** 同步工具清单：toast 展示同步数量后刷新列表 */
  const handleSync = async (s: MCPServer) => {
    setBusyId(s.id);
    try {
      const res = await syncMCPServer(s.id);
      // synced 字段可能缺省，兜底 0
      toast.ok(`已同步 ${res.synced ?? 0} 个工具`);
      await load();
    } catch (e) {
      toast.err(errMsg(e));
    } finally {
      setBusyId(null);
    }
  };

  const handleMcpHealth = async (s: MCPServer) => {
    setBusyId(s.id);
    try {
      const res = await checkMCPServer(s.id);
      toast.ok(`健康检查完成：${res.status}`);
      await load();
    } catch (e) {
      toast.err(errMsg(e));
    } finally {
      setBusyId(null);
    }
  };

  /* ---- MCP：工具调试 ---- */

  const openDebug = (s: MCPServer) => {
    setDebugServer(s);
    setDebugTool('');
    setDebugArgs('{}');
    setDebugOutput('');
  };

  /** 执行工具调试：args 非法 JSON 时阻断并提示 */
  const runDebug = async () => {
    if (!debugServer) return;
    if (!debugTool.trim()) {
      toast.err('请选择或输入工具名');
      return;
    }
    let args: Record<string, unknown> = {};
    const text = debugArgs.trim();
    if (text) {
      try {
        args = JSON.parse(text) as Record<string, unknown>;
      } catch {
        toast.err('参数不是合法的 JSON');
        return;
      }
    }
    setDebugRunning(true);
    try {
      const res = await callMCPTool(debugServer.id, debugTool.trim(), args);
      setDebugOutput(clip(JSON.stringify(res.output ?? null, null, 2)));
    } catch (e) {
      toast.err(errMsg(e));
    } finally {
      setDebugRunning(false);
    }
  };

  // 宽容解析 tools_cache：元素结构不确定，逐项尝试取 name 字段；取不到则退化为手动输入
  const toolNames =
    debugServer && Array.isArray(debugServer.tools_cache)
      ? debugServer.tools_cache
          .map((t) => (typeof t === 'object' && t !== null ? (t as { name?: string }).name : undefined))
          .filter((n): n is string => typeof n === 'string' && n.length > 0)
      : [];

  /* ---- A2A：注册 / 健康检查 / 委派任务 ---- */

  const submitRegister = async () => {
    if (!regDraft.base_url.trim()) {
      toast.err('Base URL 为必填项');
      return;
    }
    setSaving(true);
    try {
      // name 留空时不传，由后端注册时从 Agent Card 名片取
      const payload: { base_url: string; name?: string } = { base_url: regDraft.base_url.trim() };
      const name = regDraft.name.trim();
      if (name) payload.name = name;
      await registerA2AAgent(payload);
      toast.ok('远程 Agent 已注册');
      setRegisterOpen(false);
      setRegDraft({ base_url: '', name: '' });
      await load();
    } catch (e) {
      toast.err(errMsg(e));
    } finally {
      setSaving(false);
    }
  };

  const handleAgentHealth = async (a: A2AAgent) => {
    setBusyId(a.id);
    try {
      const res = await checkA2AAgent(a.id);
      toast.ok(`健康检查完成：${res.status}`);
      await load();
    } catch (e) {
      toast.err(errMsg(e));
    } finally {
      setBusyId(null);
    }
  };

  const openTask = (a: A2AAgent) => {
    setTaskAgent(a);
    setTaskText('');
    setTaskResult('');
  };

  const submitTask = async () => {
    if (!taskAgent) return;
    if (!taskText.trim()) {
      toast.err('请输入任务内容');
      return;
    }
    setTaskRunning(true);
    try {
      const res = await sendA2ATask(taskAgent.id, taskText.trim());
      setTaskResult(clip(JSON.stringify(res ?? null, null, 2)));
    } catch (e) {
      toast.err(errMsg(e));
    } finally {
      setTaskRunning(false);
    }
  };

  /* ---- 删除 ---- */

  const confirmDelete = async () => {
    if (!delTarget) return;
    setDeleting(true);
    try {
      if (delTarget.kind === 'mcp') await deleteMCPServer(delTarget.id);
      else await deleteA2AAgent(delTarget.id);
      toast.ok('删除成功');
      setDelTarget(null);
      await load();
    } catch (e) {
      toast.err(errMsg(e));
    } finally {
      setDeleting(false);
    }
  };

  return (
    <div ref={rootRef} className="page-container">
      <PageHeader
        title="集成中心"
        desc="接入外部能力：MCP 工具服务与 A2A 远程智能体，支持健康检查与在线调试"
      />

      {loading ? (
        <div className="flex justify-center py-24">
          <Spinner className="h-6 w-6 text-lime" />
        </div>
      ) : (
        <div className="space-y-6">
          {/* 区块一：MCP Server */}
          <section className="panel overflow-hidden">
            <div className="flex items-center justify-between border-b border-smoke/60 px-4 py-3">
              <h2 className="section-title">MCP Server</h2>
              <Button onClick={openMcpCreate}>＋ 新增 MCP Server</Button>
            </div>
            {servers.length === 0 ? (
              <EmptyState
                title="暂无 MCP Server"
                hint="接入 MCP 工具服务后，Agent 即可调用其暴露的工具"
                action={<Button onClick={openMcpCreate}>新增 MCP Server</Button>}
              />
            ) : (
              <div className="overflow-x-auto">
                <table className="w-full text-left text-sm">
                  <thead>
                    <tr className="border-b border-smoke/60 text-xs text-ash">
                      <th className="px-4 py-2.5 font-medium">名称</th>
                      <th className="px-4 py-2.5 font-medium">传输</th>
                      <th className="px-4 py-2.5 font-medium">端点</th>
                      <th className="px-4 py-2.5 font-medium">状态</th>
                      <th className="px-4 py-2.5 font-medium">工具数</th>
                      <th className="px-4 py-2.5 font-medium">最近健康检查</th>
                      <th className="px-4 py-2.5 text-right font-medium">操作</th>
                    </tr>
                  </thead>
                  <tbody className="divide-y divide-smoke/40">
                    {servers.map((s) => (
                      <tr key={s.id} className="transition-colors hover:bg-graphite/40">
                        <td className="px-4 py-2.5 text-bone">{s.name}</td>
                        <td className="px-4 py-2.5">
                          <Badge tone={TRANSPORT_TONE[s.transport] ?? 'gray'}>{s.transport}</Badge>
                        </td>
                        {/* stdio 展示 command，远程传输展示 url */}
                        <td className="px-4 py-2.5">
                          <span
                            className="block max-w-[220px] truncate font-mono text-xs text-fog"
                            title={s.command || s.url}
                          >
                            {s.command || s.url || '—'}
                          </span>
                        </td>
                        <td className="px-4 py-2.5">
                          <StatusBadge status={s.status} />
                        </td>
                        <td className="px-4 py-2.5 font-mono text-xs text-fog">
                          {/* tools_cache 结构不确定，仅计数展示 */}
                          {Array.isArray(s.tools_cache) ? s.tools_cache.length : 0}
                        </td>
                        <td className="px-4 py-2.5 text-xs text-fog">
                          {relativeTime(s.last_health_at) || '—'}
                        </td>
                        <td className="px-4 py-2.5">
                          <div className="flex justify-end gap-3 whitespace-nowrap text-xs">
                            <button
                              disabled={busyId === s.id}
                              className="text-ash transition-colors hover:text-lime disabled:opacity-40"
                              onClick={() => void handleSync(s)}
                            >
                              同步工具
                            </button>
                            <button
                              disabled={busyId === s.id}
                              className="text-ash transition-colors hover:text-lime disabled:opacity-40"
                              onClick={() => void handleMcpHealth(s)}
                            >
                              健康检查
                            </button>
                            <button
                              className="text-ash transition-colors hover:text-lime"
                              onClick={() => openDebug(s)}
                            >
                              调试
                            </button>
                            <button
                              className="text-ash transition-colors hover:text-lime"
                              onClick={() => openMcpEdit(s)}
                            >
                              编辑
                            </button>
                            <button
                              className="text-ash transition-colors hover:text-coral"
                              onClick={() => setDelTarget({ kind: 'mcp', id: s.id, label: s.name })}
                            >
                              删除
                            </button>
                          </div>
                        </td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
            )}
          </section>

          {/* 区块二：A2A 远程 Agent */}
          <section className="panel overflow-hidden">
            <div className="flex items-center justify-between border-b border-smoke/60 px-4 py-3">
              <h2 className="section-title">A2A 远程 Agent</h2>
              <Button onClick={() => setRegisterOpen(true)}>＋ 注册远程 Agent</Button>
            </div>
            {agents.length === 0 ? (
              <EmptyState
                title="暂无远程 Agent"
                hint="注册 A2A 远程 Agent 后，可将任务委派给其协作完成"
                action={<Button onClick={() => setRegisterOpen(true)}>注册远程 Agent</Button>}
              />
            ) : (
              <div className="overflow-x-auto">
                <table className="w-full text-left text-sm">
                  <thead>
                    <tr className="border-b border-smoke/60 text-xs text-ash">
                      <th className="px-4 py-2.5 font-medium">名称</th>
                      <th className="px-4 py-2.5 font-medium">Base URL</th>
                      <th className="px-4 py-2.5 font-medium">状态</th>
                      <th className="px-4 py-2.5 font-medium">名片摘要</th>
                      <th className="px-4 py-2.5 font-medium">最近健康检查</th>
                      <th className="px-4 py-2.5 text-right font-medium">操作</th>
                    </tr>
                  </thead>
                  <tbody className="divide-y divide-smoke/40">
                    {agents.map((a) => (
                      <tr key={a.id} className="transition-colors hover:bg-graphite/40">
                        <td className="px-4 py-2.5 text-bone">{a.name}</td>
                        <td className="px-4 py-2.5">
                          <span
                            className="block max-w-[220px] truncate font-mono text-xs text-fog"
                            title={a.base_url}
                          >
                            {a.base_url}
                          </span>
                        </td>
                        <td className="px-4 py-2.5">
                          <StatusBadge status={a.status} />
                        </td>
                        <td className="px-4 py-2.5">
                          {/* card 名片结构宽容处理：缺失或字段缺失时降级展示 */}
                          {a.card ? (
                            <div className="min-w-0">
                              <p className="max-w-[240px] truncate text-xs text-mist">
                                {(a.card as { name?: string }).name || '未命名'}
                              </p>
                              <p className="max-w-[240px] truncate text-xs text-ash">
                                {(a.card as { description?: string }).description ?? ''}
                              </p>
                            </div>
                          ) : (
                            <span className="text-xs text-ash">—</span>
                          )}
                        </td>
                        <td className="px-4 py-2.5 text-xs text-fog">
                          {relativeTime(a.last_health_at) || '—'}
                        </td>
                        <td className="px-4 py-2.5">
                          <div className="flex justify-end gap-3 whitespace-nowrap text-xs">
                            <button
                              disabled={busyId === a.id}
                              className="text-ash transition-colors hover:text-lime disabled:opacity-40"
                              onClick={() => void handleAgentHealth(a)}
                            >
                              健康检查
                            </button>
                            <button
                              className="text-ash transition-colors hover:text-lime"
                              onClick={() => openTask(a)}
                            >
                              委派任务
                            </button>
                            <button
                              className="text-ash transition-colors hover:text-coral"
                              onClick={() => setDelTarget({ kind: 'a2a', id: a.id, label: a.name })}
                            >
                              删除
                            </button>
                          </div>
                        </td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
            )}
          </section>
        </div>
      )}

      {/* 新增 / 编辑 MCP Server */}
      <Modal
        open={mcpOpen}
        title={editingMcp ? '编辑 MCP Server' : '新增 MCP Server'}
        onClose={() => setMcpOpen(false)}
        width="max-w-2xl"
        footer={
          <>
            <Button variant="ghost" onClick={() => setMcpOpen(false)}>
              取消
            </Button>
            <Button loading={saving} onClick={() => void submitMcp()}>
              保存
            </Button>
          </>
        }
      >
        <div className="space-y-3">
          <div className="grid grid-cols-2 gap-3">
            <div>
              <label className="field-label">名称 *</label>
              <Input
                value={mcpDraft.name}
                placeholder="如 filesystem"
                onChange={(e) => patchMcp({ name: e.target.value })}
              />
            </div>
            <div>
              <label className="field-label">传输方式</label>
              <Select
                value={mcpDraft.transport}
                onChange={(e) => patchMcp({ transport: e.target.value })}
              >
                {TRANSPORTS.map((t) => (
                  <option key={t} value={t}>
                    {t}
                  </option>
                ))}
              </Select>
            </div>
          </div>
          <div>
            <label className="field-label">URL（streamable_http / sse）</label>
            <Input
              value={mcpDraft.url}
              placeholder="如 https://mcp.example.com/mcp"
              onChange={(e) => patchMcp({ url: e.target.value })}
            />
          </div>
          <div>
            <label className="field-label">Command（stdio）</label>
            <Input
              value={mcpDraft.command}
              placeholder="如 npx -y @modelcontextprotocol/server-filesystem /tmp"
              onChange={(e) => patchMcp({ command: e.target.value })}
            />
          </div>
          <div>
            <label className="field-label">args（JSON 数组）</label>
            <Textarea
              rows={2}
              className="font-mono"
              value={mcpDraft.args}
              onChange={(e) => patchMcp({ args: e.target.value })}
            />
          </div>
          <div className="grid grid-cols-2 gap-3">
            <div>
              <label className="field-label">env（JSON 对象）</label>
              <Textarea
                rows={3}
                className="font-mono"
                value={mcpDraft.env}
                onChange={(e) => patchMcp({ env: e.target.value })}
              />
            </div>
            <div>
              <label className="field-label">headers（JSON 对象）</label>
              <Textarea
                rows={3}
                className="font-mono"
                value={mcpDraft.headers}
                onChange={(e) => patchMcp({ headers: e.target.value })}
              />
            </div>
          </div>
        </div>
      </Modal>

      {/* MCP 工具调试 */}
      <Modal
        open={debugServer !== null}
        title={`工具调试 · ${debugServer?.name ?? ''}`}
        onClose={() => setDebugServer(null)}
        width="max-w-xl"
        footer={
          <>
            <Button variant="ghost" onClick={() => setDebugServer(null)}>
              关闭
            </Button>
            <Button loading={debugRunning} onClick={() => void runDebug()}>
              执行
            </Button>
          </>
        }
      >
        <div className="space-y-3">
          <div>
            <label className="field-label">工具名</label>
            {toolNames.length > 0 ? (
              // tools_cache 中解析出工具名 → 下拉选择
              <Select value={debugTool} onChange={(e) => setDebugTool(e.target.value)}>
                <option value="">请选择工具</option>
                {toolNames.map((n) => (
                  <option key={n} value={n}>
                    {n}
                  </option>
                ))}
              </Select>
            ) : (
              // 解析不到工具名 → 退化为手动输入
              <Input
                value={debugTool}
                placeholder="tools_cache 中未解析出工具名，请手动输入"
                onChange={(e) => setDebugTool(e.target.value)}
              />
            )}
          </div>
          <div>
            <label className="field-label">参数（JSON）</label>
            <Textarea
              rows={4}
              className="font-mono"
              value={debugArgs}
              onChange={(e) => setDebugArgs(e.target.value)}
            />
          </div>
          {debugOutput && (
            <div>
              <label className="field-label">输出</label>
              <pre className="code-block max-h-64 whitespace-pre-wrap">{debugOutput}</pre>
            </div>
          )}
        </div>
      </Modal>

      {/* 注册远程 Agent */}
      <Modal
        open={registerOpen}
        title="注册远程 Agent"
        onClose={() => setRegisterOpen(false)}
        footer={
          <>
            <Button variant="ghost" onClick={() => setRegisterOpen(false)}>
              取消
            </Button>
            <Button loading={saving} onClick={() => void submitRegister()}>
              注册
            </Button>
          </>
        }
      >
        <div className="space-y-3">
          <div>
            <label className="field-label">Base URL *</label>
            <Input
              value={regDraft.base_url}
              placeholder="如 https://agent.example.com"
              onChange={(e) => setRegDraft((d) => ({ ...d, base_url: e.target.value }))}
            />
          </div>
          <div>
            <label className="field-label">名称（可选）</label>
            <Input
              value={regDraft.name}
              placeholder="留空则自动取名片名称；注册时会拉取 /.well-known/agent.json 验活"
              onChange={(e) => setRegDraft((d) => ({ ...d, name: e.target.value }))}
            />
          </div>
        </div>
      </Modal>

      {/* A2A 委派任务 */}
      <Modal
        open={taskAgent !== null}
        title={`委派任务 · ${taskAgent?.name ?? ''}`}
        onClose={() => setTaskAgent(null)}
        width="max-w-xl"
        footer={
          <>
            <Button variant="ghost" onClick={() => setTaskAgent(null)}>
              关闭
            </Button>
            <Button loading={taskRunning} onClick={() => void submitTask()}>
              发送任务
            </Button>
          </>
        }
      >
        <div className="space-y-3">
          <div>
            <label className="field-label">任务内容</label>
            <Textarea
              rows={4}
              value={taskText}
              placeholder="用自然语言描述要委派给该 Agent 的任务"
              onChange={(e) => setTaskText(e.target.value)}
            />
          </div>
          {taskResult && (
            <div>
              <label className="field-label">执行结果</label>
              <pre className="code-block max-h-64 whitespace-pre-wrap">{taskResult}</pre>
            </div>
          )}
        </div>
      </Modal>

      {/* 删除二次确认 */}
      <ConfirmDialog
        open={delTarget !== null}
        title={delTarget?.kind === 'mcp' ? '删除 MCP Server' : '删除远程 Agent'}
        content={
          delTarget
            ? `确认删除「${delTarget.label}」？该操作不可恢复。`
            : ''
        }
        danger
        confirmText="删除"
        loading={deleting}
        onCancel={() => setDelTarget(null)}
        onConfirm={() => void confirmDelete()}
      />
    </div>
  );
}
