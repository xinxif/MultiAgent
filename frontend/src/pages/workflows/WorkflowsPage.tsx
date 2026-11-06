/**
 * 工作流管理页 —— 列表 / 新建与编辑（DSL 为 YAML 文本）/ 同步执行。
 *
 * 职责：
 * 1. 拉取工作流列表（单页 100 条），展示名称、描述、版本、启用状态与更新时间；
 * 2. 新建与编辑复用同一 Modal：名称 + 描述 + DSL 文本，支持先「校验 DSL」再保存；
 * 3. 执行 Modal：入参为 JSON 对象文本（JSON.parse 校验），调用同步执行接口后
 *    就地展示结果（状态 / 输出 / 错误），waiting_approval 时引导到执行记录页审批。
 */
import { useCallback, useEffect, useRef, useState } from 'react';
import {
  createWorkflow,
  deleteWorkflow,
  listWorkflows,
  runWorkflow,
  updateWorkflow,
  validateWorkflow,
} from '@/api/workflow';
import type { Workflow, WorkflowRun } from '@/types/workflow';
import { Badge, Button, EmptyState, Input, PageHeader, Spinner, StatusBadge, Textarea } from '@/components/ui/primitives';
import { Modal } from '@/components/ui/Modal';
import { ConfirmDialog } from '@/components/ui/ConfirmDialog';
import { relativeTime, truncate } from '@/lib/format';
import { staggerEnter } from '@/lib/gsap';
import { toast } from '@/stores/ui';
import { errMsg } from '@/lib/request';

/** 新建时预填的 DSL 示例模板：注释说明各节点类型与连线规则 */
const DSL_TEMPLATE = `# 节点类型：llm(prompt/model) / tool(tool_code/args) / kb(kb_id/query)
#             condition(expr/then/else) / parallel(branches) / human(审批) / subflow(workflow)
# edges 省略时按 nodes 声明顺序线性连接
name: 示例工作流
desc: 简单的检索问答流水线
input: [question]
nodes:
  - key: retrieve
    type: kb
    kb_id: 1
    query: "{{question}}"
  - key: answer
    type: llm
    prompt: |
      根据资料回答问题。资料：{{retrieve.result}}
      问题：{{question}}
edges: [retrieve, answer]
`;

export function WorkflowsPage() {
  /* ---- 列表状态 ---- */
  const [list, setList] = useState<Workflow[]>([]);
  const [loading, setLoading] = useState(true);
  const tbodyRef = useRef<HTMLTableSectionElement>(null);

  /* ---- 编辑器状态（新建/编辑复用） ---- */
  const [editorOpen, setEditorOpen] = useState(false);
  const [editing, setEditing] = useState<Workflow | null>(null);
  const [name, setName] = useState('');
  const [description, setDescription] = useState('');
  const [dsl, setDsl] = useState(DSL_TEMPLATE);
  const [saving, setSaving] = useState(false);
  const [validating, setValidating] = useState(false);

  /* ---- 执行状态 ---- */
  const [runTarget, setRunTarget] = useState<Workflow | null>(null);
  const [inputText, setInputText] = useState('{}');
  const [running, setRunning] = useState(false);
  const [runResult, setRunResult] = useState<WorkflowRun | null>(null);

  /* ---- 删除确认 ---- */
  const [delTarget, setDelTarget] = useState<Workflow | null>(null);
  const [deleting, setDeleting] = useState(false);

  /** 拉取工作流列表（固定单页 100 条，控制台场景足够） */
  const load = useCallback(async () => {
    setLoading(true);
    try {
      const p = await listWorkflows({ page: 1, page_size: 100 });
      setList(p.list);
    } catch (e) {
      toast.err(errMsg(e));
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    void load();
  }, [load]);

  // 列表加载完成后：行级 stagger 入场
  useEffect(() => {
    if (!loading && tbodyRef.current && tbodyRef.current.children.length > 0) {
      staggerEnter(Array.from(tbodyRef.current.children));
    }
  }, [loading, list]);

  /** 打开编辑器：传 w 为编辑（回填表单），否则新建（DSL 预填示例模板） */
  const openEditor = (w?: Workflow) => {
    setEditing(w ?? null);
    setName(w?.name ?? '');
    setDescription(w?.description ?? '');
    setDsl(w?.dsl || DSL_TEMPLATE);
    setEditorOpen(true);
  };

  /** 校验 DSL：后端返回校验错误信息，直接透传给 toast */
  const handleValidate = async () => {
    setValidating(true);
    try {
      await validateWorkflow(dsl);
      toast.ok('DSL 校验通过');
    } catch (e) {
      toast.err(errMsg(e));
    } finally {
      setValidating(false);
    }
  };

  /** 保存：新建走 create，编辑走 update */
  const handleSave = async () => {
    if (!name.trim()) {
      toast.err('请输入工作流名称');
      return;
    }
    setSaving(true);
    try {
      const payload = { name: name.trim(), description: description.trim(), dsl };
      if (editing) {
        await updateWorkflow(editing.id, payload);
        toast.ok('工作流已保存');
      } else {
        await createWorkflow(payload);
        toast.ok('工作流已创建');
      }
      setEditorOpen(false);
      void load();
    } catch (e) {
      toast.err(errMsg(e));
    } finally {
      setSaving(false);
    }
  };

  /** 打开执行 Modal：重置入参与上次结果 */
  const openRun = (w: Workflow) => {
    setRunTarget(w);
    setInputText('{}');
    setRunResult(null);
  };

  /** 解析入参文本：必须是 JSON 对象（拒绝数组/字面量），失败时 toast 并返回 null */
  const parseInput = (): Record<string, unknown> | null => {
    const raw = inputText.trim() || '{}';
    try {
      const v: unknown = JSON.parse(raw);
      if (v === null || typeof v !== 'object' || Array.isArray(v)) {
        toast.err('入参必须是 JSON 对象，如 {"question":"..."}');
        return null;
      }
      return v as Record<string, unknown>;
    } catch {
      toast.err('入参 JSON 解析失败，请检查格式');
      return null;
    }
  };

  /** 同步执行：结果就地展示（succeeded/failed 立即出结果，waiting_approval 需去审批） */
  const handleRun = async () => {
    if (!runTarget) return;
    const parsed = parseInput();
    if (!parsed) return;
    setRunning(true);
    try {
      const run = await runWorkflow(runTarget.id, parsed);
      setRunResult(run);
    } catch (e) {
      toast.err(errMsg(e));
    } finally {
      setRunning(false);
    }
  };

  /** 删除（ConfirmDialog 二次确认后调用） */
  const handleDelete = async () => {
    if (!delTarget) return;
    setDeleting(true);
    try {
      await deleteWorkflow(delTarget.id);
      toast.ok('工作流已删除');
      setDelTarget(null);
      void load();
    } catch (e) {
      toast.err(errMsg(e));
    } finally {
      setDeleting(false);
    }
  };

  return (
    <div className="page-container">
      <PageHeader
        title="工作流"
        desc="用 YAML DSL 编排 LLM、工具、知识库等节点，支持人工审批（HITL）"
        action={
          <Button onClick={() => openEditor()}>＋ 新建工作流</Button>
        }
      />

      {/* 列表 */}
      <div className="panel overflow-hidden">
        <table className="w-full text-sm">
          <thead>
            <tr className="border-b border-smoke/60 bg-carbon/60 text-left text-xs text-ash">
              <th className="px-4 py-3 font-medium">名称</th>
              <th className="px-4 py-3 font-medium">描述</th>
              <th className="px-4 py-3 font-medium">版本</th>
              <th className="px-4 py-3 font-medium">状态</th>
              <th className="px-4 py-3 font-medium">更新时间</th>
              <th className="px-4 py-3 text-right font-medium">操作</th>
            </tr>
          </thead>
          <tbody ref={tbodyRef}>
            {loading ? (
              // 加载中：占位一行
              <tr>
                <td colSpan={6} className="px-4 py-12 text-center">
                  <Spinner className="mx-auto h-5 w-5 text-ash" />
                </td>
              </tr>
            ) : list.length === 0 ? (
              // 空态：引导新建
              <tr>
                <td colSpan={6}>
                  <EmptyState
                    icon="⟳"
                    title="还没有工作流"
                    hint="点击右上角「新建工作流」，用示例 DSL 快速开始；节点支持 llm / tool / kb / condition / parallel / human / subflow"
                  />
                </td>
              </tr>
            ) : (
              list.map((w) => (
                <tr key={w.id} className="border-b border-smoke/40 transition-colors last:border-b-0 hover:bg-graphite/40">
                  <td className="px-4 py-3 font-medium text-bone">{w.name}</td>
                  <td className="max-w-[18rem] px-4 py-3 text-xs text-fog" title={w.description}>
                    {w.description ? truncate(w.description, 40) : '—'}
                  </td>
                  <td className="px-4 py-3 font-mono text-xs text-fog">v{w.version}</td>
                  <td className="px-4 py-3">
                    <Badge tone={w.is_active ? 'pulse' : 'gray'}>{w.is_active ? '启用' : '停用'}</Badge>
                  </td>
                  <td className="px-4 py-3 text-xs text-ash">{relativeTime(w.updated_at)}</td>
                  <td className="px-4 py-3">
                    <div className="flex justify-end gap-3 text-xs">
                      <button className="text-ash transition-colors hover:text-lime" onClick={() => openEditor(w)}>
                        编辑
                      </button>
                      <button className="text-ash transition-colors hover:text-teal" onClick={() => openRun(w)}>
                        执行
                      </button>
                      <button className="text-ash transition-colors hover:text-coral" onClick={() => setDelTarget(w)}>
                        删除
                      </button>
                    </div>
                  </td>
                </tr>
              ))
            )}
          </tbody>
        </table>
      </div>

      {/* 新建 / 编辑 Modal */}
      <Modal
        open={editorOpen}
        title={editing ? `编辑工作流 · ${editing.name}` : '新建工作流'}
        onClose={() => setEditorOpen(false)}
        width="max-w-2xl"
        footer={
          <>
            {/* 校验独立于保存：先确认 DSL 合法再落库 */}
            <Button variant="ghost" loading={validating} onClick={() => void handleValidate()}>
              校验 DSL
            </Button>
            <Button variant="ghost" onClick={() => setEditorOpen(false)}>
              取消
            </Button>
            <Button loading={saving} onClick={() => void handleSave()}>
              保存
            </Button>
          </>
        }
      >
        <div className="space-y-4">
          <div>
            <label className="field-label">名称 *</label>
            <Input value={name} placeholder="工作流名称" onChange={(e) => setName(e.target.value)} />
          </div>
          <div>
            <label className="field-label">描述</label>
            <Input value={description} placeholder="一句话说明用途（可选）" onChange={(e) => setDescription(e.target.value)} />
          </div>
          <div>
            <label className="field-label">DSL（YAML）</label>
            {/* 等宽小字号，贴合代码编辑直觉 */}
            <Textarea
              rows={16}
              className="font-mono text-xs leading-relaxed"
              spellCheck={false}
              value={dsl}
              onChange={(e) => setDsl(e.target.value)}
            />
          </div>
        </div>
      </Modal>

      {/* 执行 Modal */}
      <Modal
        open={runTarget !== null}
        title={runTarget ? `执行工作流 · ${runTarget.name}` : '执行工作流'}
        onClose={() => setRunTarget(null)}
        width="max-w-xl"
        footer={
          <>
            <Button variant="ghost" onClick={() => setRunTarget(null)}>
              关闭
            </Button>
            <Button loading={running} onClick={() => void handleRun()}>
              执行
            </Button>
          </>
        }
      >
        <div className="space-y-3">
          <div>
            <label className="field-label">入参 input（JSON 对象）</label>
            <Textarea
              rows={4}
              className="font-mono text-xs"
              spellCheck={false}
              placeholder='{"question": "你好"}'
              value={inputText}
              onChange={(e) => setInputText(e.target.value)}
            />
          </div>

          {/* 结果区：状态 / 提示 / 错误 / 输出 */}
          {runResult && (
            <div className="space-y-2 border-t border-smoke/60 pt-3">
              <div className="flex items-center gap-2">
                <span className="section-title">执行结果</span>
                <StatusBadge status={runResult.status} />
              </div>
              {runResult.status === 'waiting_approval' && (
                <p className="rounded-lg border border-lime/40 bg-lime/10 px-3 py-2 text-xs text-lime">
                  已挂起等待审批，请在执行记录页处理
                </p>
              )}
              {runResult.error && <p className="text-xs text-coral">✕ {runResult.error}</p>}
              {runResult.output && (
                <pre className="code-block max-h-48 whitespace-pre-wrap">
                  {truncate(JSON.stringify(runResult.output, null, 2), 500)}
                </pre>
              )}
            </div>
          )}
        </div>
      </Modal>

      {/* 删除二次确认 */}
      <ConfirmDialog
        open={delTarget !== null}
        title="删除工作流"
        content={`确定删除工作流「${delTarget?.name ?? ''}」？已产生的执行记录会保留，但无法再发起执行。`}
        confirmText="删除"
        danger
        loading={deleting}
        onCancel={() => setDelTarget(null)}
        onConfirm={() => void handleDelete()}
      />
    </div>
  );
}
