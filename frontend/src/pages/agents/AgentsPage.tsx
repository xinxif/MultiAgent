/**
 * Agent 配置管理页 —— 平台 Agent（host 路由 / expert 专项）的增删改查。
 *
 * 职责：
 * 1. 拉取 Agent 清单，卡片网格展示名称、类型、参数摘要与工具/知识库绑定数；
 * 2. 新建与编辑复用同一 Modal 表单（编辑预填），工具绑定用 checkbox 多选；
 * 3. 删除走 ConfirmDialog 二次确认，预置 Agent 禁止删除。
 */
import { useCallback, useEffect, useRef, useState } from 'react';
import {
  Badge,
  Button,
  Card,
  EmptyState,
  Input,
  PageHeader,
  Select,
  Spinner,
  Textarea,
} from '@/components/ui/primitives';
import { Modal } from '@/components/ui/Modal';
import { ConfirmDialog } from '@/components/ui/ConfirmDialog';
import { createAgent, deleteAgent, listAgents, listTools, updateAgent } from '@/api/agent';
import type { Agent, AgentReq, Tool } from '@/types/agent';
import { errMsg } from '@/lib/request';
import { toast } from '@/stores/ui';
import { staggerEnter } from '@/lib/gsap';

/* ---- 常量映射 ---- */

/** Agent 类型 → Badge 文案 / 色调 */
const TYPE_BADGE: Record<Agent['type'], { text: string; tone: 'violet' | 'teal' }> = {
  host: { text: 'Host·路由', tone: 'violet' },
  expert: { text: 'Expert·专项', tone: 'teal' },
};

/** 表单草稿：数字输入统一存 string，提交时再转换（避免输入中间态被强转 NaN） */
interface AgentForm {
  name: string;
  description: string;
  type: Agent['type'];
  system_prompt: string;
  model_config_id: string;
  temperature: string;
  max_tokens: string;
  max_iterations: string;
  memory_enabled: boolean;
  memory_long_enabled: boolean;
  /** 逗号分隔的知识库 ID 串 */
  kb_ids: string;
  /** 已勾选绑定的工具 ID 集合 */
  tool_ids: number[];
}

const EMPTY_FORM: AgentForm = {
  name: '',
  description: '',
  type: 'expert',
  system_prompt: '',
  model_config_id: '',
  temperature: '0.7',
  max_tokens: '',
  max_iterations: '',
  memory_enabled: true,
  memory_long_enabled: false,
  kb_ids: '',
  tool_ids: [],
};

/** 编辑预填：Agent 实体 → 表单草稿 */
function toForm(a: Agent): AgentForm {
  return {
    name: a.name,
    description: a.description,
    type: a.type,
    system_prompt: a.system_prompt,
    model_config_id: a.model_config_id ? String(a.model_config_id) : '',
    temperature: a.temperature != null ? String(a.temperature) : '',
    max_tokens: a.max_tokens != null ? String(a.max_tokens) : '',
    max_iterations: a.max_iterations != null ? String(a.max_iterations) : '',
    memory_enabled: a.memory_enabled,
    memory_long_enabled: a.memory_long_enabled,
    kb_ids: (a.kb_ids ?? []).join(','),
    tool_ids: (a.tools ?? []).map((t) => t.tool_id),
  };
}

/** 逗号 / 空白分隔的数字串 → number[]（过滤非法片段） */
function parseIds(s: string): number[] {
  return s
    .split(/[,，\s]+/)
    .filter((t) => t !== '')
    .map(Number)
    .filter((n) => Number.isFinite(n));
}

export function AgentsPage() {
  const [agents, setAgents] = useState<Agent[]>([]);
  const [loading, setLoading] = useState(true);
  const [tools, setTools] = useState<Tool[]>([]);
  const [modalOpen, setModalOpen] = useState(false);
  const [editing, setEditing] = useState<Agent | null>(null);
  const [form, setForm] = useState<AgentForm>(EMPTY_FORM);
  const [saving, setSaving] = useState(false);
  const [delTarget, setDelTarget] = useState<Agent | null>(null);
  const gridRef = useRef<HTMLDivElement>(null);

  /** 拉取 Agent 清单（单页 100 条足够管理场景） */
  const refresh = useCallback(async () => {
    setLoading(true);
    try {
      const data = await listAgents({ page: 1, page_size: 100 });
      setAgents(data.list);
    } catch (e) {
      toast.err(errMsg(e));
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    void refresh();
  }, [refresh]);

  // 列表挂载 / 刷新后子项交错入场（GSAP）
  useEffect(() => {
    const els = gridRef.current ? Array.from(gridRef.current.children) : [];
    if (els.length === 0) return;
    const tween = staggerEnter(els);
    return () => {
      tween.kill();
    };
  }, [agents]);

  // Modal 打开时拉取工具清单，供绑定勾选
  useEffect(() => {
    if (!modalOpen) return;
    listTools({ page: 1, page_size: 100 })
      .then((d) => setTools(d.list))
      .catch((e) => toast.err(errMsg(e)));
  }, [modalOpen]);

  const openCreate = () => {
    setEditing(null);
    setForm(EMPTY_FORM);
    setModalOpen(true);
  };

  const openEdit = (a: Agent) => {
    setEditing(a);
    setForm(toForm(a));
    setModalOpen(true);
  };

  /** 通用表单字段写入 */
  const setField = <K extends keyof AgentForm>(key: K, value: AgentForm[K]) => {
    setForm((f) => ({ ...f, [key]: value }));
  };

  /** 工具勾选切换 */
  const toggleTool = (id: number) => {
    setForm((f) => ({
      ...f,
      tool_ids: f.tool_ids.includes(id)
        ? f.tool_ids.filter((t) => t !== id)
        : [...f.tool_ids, id],
    }));
  };

  /** 提交：新建走 createAgent，编辑走 updateAgent */
  const submit = async () => {
    if (!form.name.trim()) {
      toast.err('请填写 Agent 名称');
      return;
    }
    // 空串数字字段不提交（AgentReq 各字段均可选）
    const num = (s: string): number | undefined => (s.trim() === '' ? undefined : Number(s));
    const req: AgentReq = {
      name: form.name.trim(),
      description: form.description.trim(),
      type: form.type,
      system_prompt: form.system_prompt,
      model_config_id: num(form.model_config_id),
      temperature: num(form.temperature),
      max_tokens: num(form.max_tokens),
      max_iterations: num(form.max_iterations),
      memory_enabled: form.memory_enabled,
      memory_long_enabled: form.memory_long_enabled,
      kb_ids: parseIds(form.kb_ids),
      // 勾选结果映射为 {tool_id}[]
      tools: form.tool_ids.map((tool_id) => ({ tool_id })),
    };
    setSaving(true);
    try {
      if (editing) {
        await updateAgent(editing.id, req);
        toast.ok('Agent 已更新');
      } else {
        await createAgent(req);
        toast.ok('Agent 已创建');
      }
      setModalOpen(false);
      await refresh();
    } catch (e) {
      toast.err(errMsg(e));
    } finally {
      setSaving(false);
    }
  };

  const confirmDelete = async () => {
    if (!delTarget) return;
    try {
      await deleteAgent(delTarget.id);
      toast.ok('Agent 已删除');
      setDelTarget(null);
      await refresh();
    } catch (e) {
      toast.err(errMsg(e));
    }
  };

  return (
    <div className="page-container">
      <PageHeader
        title="Agent 管理"
        desc="host 负责路由聚合，expert 负责专项执行"
        action={<Button onClick={openCreate}>＋ 新建 Agent</Button>}
      />

      {loading ? (
        <div className="flex justify-center py-20">
          <Spinner className="h-6 w-6 text-lime" />
        </div>
      ) : agents.length === 0 ? (
        <EmptyState
          title="暂无 Agent"
          hint="点击右上角「新建 Agent」，创建第一个 Agent 配置"
        />
      ) : (
        <div ref={gridRef} className="grid grid-cols-1 gap-4 md:grid-cols-2 xl:grid-cols-3">
          {agents.map((a) => (
            <Card key={a.id} className="flex flex-col gap-3">
              {/* 名称 + 预置标记 + 类型徽标 */}
              <div className="space-y-1.5">
                <div className="flex items-center gap-2">
                  <h3 className="truncate text-sm font-semibold text-bone">{a.name}</h3>
                  {a.is_preset && <Badge tone="gray">预置</Badge>}
                </div>
                <Badge tone={TYPE_BADGE[a.type].tone}>{TYPE_BADGE[a.type].text}</Badge>
              </div>

              {/* 描述（最多两行） */}
              <p className="line-clamp-2 text-xs leading-relaxed text-fog">
                {a.description || '暂无描述'}
              </p>

              {/* 参数摘要 */}
              <div className="grid grid-cols-3 gap-2 rounded-lg border border-smoke/40 bg-carbon/60 px-3 py-2 text-center">
                <div>
                  <p className="text-sm text-bone">{a.temperature}</p>
                  <p className="text-[10px] text-ash">温度</p>
                </div>
                <div>
                  <p className="text-sm text-bone">{a.max_tokens}</p>
                  <p className="text-[10px] text-ash">Max Tokens</p>
                </div>
                <div>
                  <p className="text-sm text-bone">{a.max_iterations}</p>
                  <p className="text-[10px] text-ash">迭代上限</p>
                </div>
              </div>

              {/* 绑定计数 */}
              <div className="flex gap-4 text-xs text-ash">
                <span>
                  工具 <span className="text-fog">{a.tools.length}</span>
                </span>
                <span>
                  知识库 <span className="text-fog">{a.kb_ids.length}</span>
                </span>
              </div>

              {/* 操作区：编辑始终可用；预置 Agent 禁止删除 */}
              <div className="mt-auto flex justify-end gap-2 border-t border-smoke/40 pt-3">
                <Button variant="ghost" className="px-3 py-1 text-xs" onClick={() => openEdit(a)}>
                  编辑
                </Button>
                <Button
                  variant="danger"
                  className="px-3 py-1 text-xs"
                  disabled={a.is_preset}
                  title={a.is_preset ? '预置 Agent 不可删除' : undefined}
                  onClick={() => setDelTarget(a)}
                >
                  删除
                </Button>
              </div>
            </Card>
          ))}
        </div>
      )}

      {/* 新建 / 编辑 Modal（复用同一表单，编辑时预填） */}
      <Modal
        open={modalOpen}
        title={editing ? `编辑 Agent · ${editing.name}` : '新建 Agent'}
        onClose={() => setModalOpen(false)}
        width="max-w-2xl"
        footer={
          <>
            <Button variant="ghost" onClick={() => setModalOpen(false)}>
              取消
            </Button>
            <Button loading={saving} onClick={() => void submit()}>
              {editing ? '保存修改' : '创建'}
            </Button>
          </>
        }
      >
        <form
          className="space-y-4"
          onSubmit={(e) => {
            e.preventDefault();
            void submit();
          }}
        >
          <div className="grid grid-cols-2 gap-3">
            <div>
              <label className="field-label">名称 *</label>
              <Input
                value={form.name}
                onChange={(e) => setField('name', e.target.value)}
                placeholder="如：客服专员"
              />
            </div>
            <div>
              <label className="field-label">类型</label>
              <Select
                value={form.type}
                onChange={(e) => setField('type', e.target.value as Agent['type'])}
              >
                <option value="expert">expert · 专项执行</option>
                <option value="host">host · 路由聚合</option>
              </Select>
            </div>
          </div>

          <div>
            <label className="field-label">描述</label>
            <Input
              value={form.description}
              onChange={(e) => setField('description', e.target.value)}
              placeholder="Agent 职责说明"
            />
          </div>

          <div>
            <label className="field-label">System Prompt</label>
            <Textarea
              rows={6}
              value={form.system_prompt}
              onChange={(e) => setField('system_prompt', e.target.value)}
              placeholder="设定 Agent 的角色、能力边界与输出规范"
            />
          </div>

          <div className="grid grid-cols-2 gap-3">
            <div>
              <label className="field-label">模型配置 ID</label>
              <Input
                type="number"
                value={form.model_config_id}
                onChange={(e) => setField('model_config_id', e.target.value)}
                placeholder="如：1"
              />
              <p className="mt-1 text-xs text-ash">模型配置 ID，管理员在模型管理页查看</p>
            </div>
            <div>
              <label className="field-label">Temperature</label>
              <Input
                type="number"
                step="0.1"
                value={form.temperature}
                onChange={(e) => setField('temperature', e.target.value)}
                placeholder="0.7"
              />
            </div>
            <div>
              <label className="field-label">Max Tokens</label>
              <Input
                type="number"
                value={form.max_tokens}
                onChange={(e) => setField('max_tokens', e.target.value)}
                placeholder="2048"
              />
            </div>
            <div>
              <label className="field-label">最大迭代次数</label>
              <Input
                type="number"
                value={form.max_iterations}
                onChange={(e) => setField('max_iterations', e.target.value)}
                placeholder="5"
              />
            </div>
          </div>

          {/* 记忆开关 */}
          <div className="grid grid-cols-2 gap-3">
            <label className="flex cursor-pointer items-center gap-2 text-sm text-mist">
              <input
                type="checkbox"
                className="h-3.5 w-3.5 accent-lime"
                checked={form.memory_enabled}
                onChange={(e) => setField('memory_enabled', e.target.checked)}
              />
              启用短期记忆
            </label>
            <label className="flex cursor-pointer items-center gap-2 text-sm text-mist">
              <input
                type="checkbox"
                className="h-3.5 w-3.5 accent-lime"
                checked={form.memory_long_enabled}
                onChange={(e) => setField('memory_long_enabled', e.target.checked)}
              />
              启用长期记忆
            </label>
          </div>

          <div>
            <label className="field-label">绑定知识库 ID</label>
            <Input
              value={form.kb_ids}
              onChange={(e) => setField('kb_ids', e.target.value)}
              placeholder="如：1,2,3（逗号分隔数字）"
            />
            <p className="mt-1 text-xs text-ash">多个 ID 用逗号分隔，留空表示不绑定</p>
          </div>

          {/* 工具绑定：checkbox 列表，危险工具带 coral 警示 */}
          <div>
            <label className="field-label">工具绑定</label>
            <div className="max-h-40 space-y-0.5 overflow-y-auto rounded-lg border border-smoke/60 bg-graphite/40 p-2">
              {tools.map((t) => (
                <label
                  key={t.id}
                  className="flex cursor-pointer items-center gap-2 rounded px-1.5 py-1 text-xs hover:bg-obsidian"
                >
                  <input
                    type="checkbox"
                    className="h-3.5 w-3.5 accent-lime"
                    checked={form.tool_ids.includes(t.id)}
                    onChange={() => toggleTool(t.id)}
                  />
                  <span className="text-bone">{t.name}</span>
                  <span className="font-mono text-ash">{t.code}</span>
                  {t.is_dangerous && <Badge tone="coral" className="ml-auto">危险</Badge>}
                </label>
              ))}
              {tools.length === 0 && (
                <p className="py-2 text-center text-xs text-ash">暂无可用工具</p>
              )}
            </div>
          </div>
        </form>
      </Modal>

      {/* 删除二次确认 */}
      <ConfirmDialog
        open={delTarget !== null}
        title="删除 Agent"
        content={`确认删除 Agent「${delTarget?.name ?? ''}」？删除后不可恢复。`}
        danger
        confirmText="删除"
        onCancel={() => setDelTarget(null)}
        onConfirm={() => void confirmDelete()}
      />
    </div>
  );
}
