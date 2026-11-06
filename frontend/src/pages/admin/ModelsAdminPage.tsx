/**
 * 模型网关管理页 —— 「模型供应商」与「模型配置」两个区块上下排列。
 *
 * 职责：
 * 1. 供应商 CRUD：API Key 由服务端 AES-GCM 加密存储、不回显，编辑时留空表示不修改；
 * 2. 模型配置 CRUD：类型（chat/embedding/rerank）、上下文窗口、输入/输出计价、
 *    故障转移优先级、能力位（vision/function_call/reasoning）与默认模型标记；
 * 3. 挂载时并行拉取供应商与模型列表，任一写操作成功后整体刷新，保证两区数据一致。
 */
import { useEffect, useRef, useState } from 'react';
import {
  createModel,
  createProvider,
  deleteModel,
  deleteProvider,
  listModels,
  listProviders,
  updateModel,
  updateProvider,
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
} from '@/components/ui/primitives';
import { relativeTime } from '@/lib/format';
import { staggerEnter } from '@/lib/gsap';
import { errMsg } from '@/lib/request';
import { toast } from '@/stores/ui';
import type { ModelConfig, ModelProvider, ModelType } from '@/types/admin';

/* ---- 常量映射 ---- */

/** 模型类型 → 徽标色调：chat 主色 / embedding 青色 / rerank 紫色 */
const TYPE_TONE: Record<ModelType, 'lime' | 'teal' | 'violet'> = {
  chat: 'lime',
  embedding: 'teal',
  rerank: 'violet',
};

/** 模型类型 → 中文展示 */
const TYPE_LABEL: Record<ModelType, string> = {
  chat: '对话',
  embedding: '向量',
  rerank: '重排',
};

/** 能力位选项（对应后端 capabilities 数组元素） */
const CAP_DEFS: { key: string; label: string }[] = [
  { key: 'vision', label: '视觉理解' },
  { key: 'function_call', label: '函数调用' },
  { key: 'reasoning', label: '深度推理' },
];

/** 数字输入统一转换：非法输入归 0，避免 NaN 传给后端 */
const toNum = (v: string): number => {
  const n = Number(v);
  return Number.isFinite(n) ? n : 0;
};

/** 数字状态 → 启停徽标（约定 1 = 启用） */
function EnabledBadge({ status }: { status: number }) {
  return status === 1 ? <Badge tone="lime">启用</Badge> : <Badge tone="gray">停用</Badge>;
}

/* ---- 表单草稿类型 ---- */

/** 供应商表单草稿；api_key 仅在填写时随请求提交 */
interface ProviderDraft {
  code: string;
  name: string;
  base_url: string;
  api_key: string;
  status: boolean;
}

const EMPTY_PROVIDER: ProviderDraft = {
  code: '',
  name: '',
  base_url: '',
  api_key: '',
  status: true,
};

/** 模型表单草稿；provider_id 用字符串承载 Select 值，提交时转数字 */
interface ModelDraft {
  provider_id: string;
  model_name: string;
  alias: string;
  type: ModelType;
  context_window: number;
  max_output: number;
  input_price: number;
  output_price: number;
  priority: number;
  capabilities: string[];
  is_default: boolean;
  status: boolean;
}

const EMPTY_MODEL: ModelDraft = {
  provider_id: '',
  model_name: '',
  alias: '',
  type: 'chat',
  context_window: 8192,
  max_output: 4096,
  input_price: 0,
  output_price: 0,
  priority: 100,
  capabilities: [],
  is_default: false,
  status: true,
};

export function ModelsAdminPage() {
  /* ---- 列表数据与全局状态 ---- */
  const [providers, setProviders] = useState<ModelProvider[]>([]);
  const [models, setModels] = useState<ModelConfig[]>([]);
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const [deleting, setDeleting] = useState(false);

  /* ---- 供应商表单 ---- */
  const [providerOpen, setProviderOpen] = useState(false);
  const [editingProvider, setEditingProvider] = useState<ModelProvider | null>(null);
  const [providerDraft, setProviderDraft] = useState<ProviderDraft>(EMPTY_PROVIDER);

  /* ---- 模型表单 ---- */
  const [modelOpen, setModelOpen] = useState(false);
  const [editingModel, setEditingModel] = useState<ModelConfig | null>(null);
  const [modelDraft, setModelDraft] = useState<ModelDraft>(EMPTY_MODEL);

  /** 删除目标：kind 区分区块，label 用于确认文案 */
  const [delTarget, setDelTarget] = useState<{
    kind: 'provider' | 'model';
    id: number;
    label: string;
  } | null>(null);

  const rootRef = useRef<HTMLDivElement>(null);

  /* ---- 数据加载 ---- */

  /** 并行拉取供应商 + 模型；写操作成功后复用整体刷新 */
  const load = async () => {
    setLoading(true);
    try {
      const [providerList, modelList] = await Promise.all([listProviders(), listModels()]);
      setProviders(providerList);
      setModels(modelList);
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

  /* ---- 表单辅助 ---- */

  const patchProvider = (patch: Partial<ProviderDraft>) =>
    setProviderDraft((d) => ({ ...d, ...patch }));

  const patchModel = (patch: Partial<ModelDraft>) =>
    setModelDraft((d) => ({ ...d, ...patch }));

  /** 切换能力位勾选状态 */
  const toggleCap = (key: string) => {
    setModelDraft((d) => ({
      ...d,
      capabilities: d.capabilities.includes(key)
        ? d.capabilities.filter((c) => c !== key)
        : [...d.capabilities, key],
    }));
  };

  /* ---- 供应商：新增 / 编辑 ---- */

  const openProviderCreate = () => {
    setEditingProvider(null);
    setProviderDraft(EMPTY_PROVIDER);
    setProviderOpen(true);
  };

  const openProviderEdit = (p: ModelProvider) => {
    setEditingProvider(p);
    // api_key 加密存储不回显，留空提交表示不修改
    setProviderDraft({
      code: p.code,
      name: p.name,
      base_url: p.base_url,
      api_key: '',
      status: p.status === 1,
    });
    setProviderOpen(true);
  };

  const submitProvider = async () => {
    if (!providerDraft.code.trim() || !providerDraft.name.trim() || !providerDraft.base_url.trim()) {
      toast.err('code、名称、Base URL 均为必填项');
      return;
    }
    setSaving(true);
    try {
      // status 显式提交（1=启用 / 0=停用），配合后端局部更新语义
      const base = {
        code: providerDraft.code.trim(),
        name: providerDraft.name.trim(),
        base_url: providerDraft.base_url.trim(),
        status: providerDraft.status ? 1 : 0,
      };
      // api_key 仅在填写时提交（服务端 AES-GCM 加密存储）
      const payload = providerDraft.api_key.trim()
        ? { ...base, api_key: providerDraft.api_key.trim() }
        : base;
      if (editingProvider) {
        await updateProvider(editingProvider.id, payload);
        toast.ok('供应商已更新');
      } else {
        await createProvider(payload);
        toast.ok('供应商已创建');
      }
      setProviderOpen(false);
      await load();
    } catch (e) {
      toast.err(errMsg(e));
    } finally {
      setSaving(false);
    }
  };

  /* ---- 模型：新增 / 编辑 ---- */

  const openModelCreate = () => {
    setEditingModel(null);
    setModelDraft(EMPTY_MODEL);
    setModelOpen(true);
  };

  const openModelEdit = (m: ModelConfig) => {
    setEditingModel(m);
    setModelDraft({
      provider_id: String(m.provider_id),
      model_name: m.model_name,
      alias: m.alias,
      type: m.type,
      context_window: m.context_window,
      max_output: m.max_output,
      input_price: m.input_price,
      output_price: m.output_price,
      priority: m.priority,
      capabilities: m.capabilities ?? [],
      is_default: m.is_default,
      status: m.status === 1,
    });
    setModelOpen(true);
  };

  const submitModel = async () => {
    if (!modelDraft.model_name.trim()) {
      toast.err('模型名称为必填项');
      return;
    }
    if (!modelDraft.provider_id) {
      toast.err('请选择所属供应商');
      return;
    }
    setSaving(true);
    try {
      const payload: Partial<ModelConfig> = {
        provider_id: Number(modelDraft.provider_id),
        model_name: modelDraft.model_name.trim(),
        alias: modelDraft.alias.trim(),
        type: modelDraft.type,
        context_window: modelDraft.context_window,
        max_output: modelDraft.max_output,
        input_price: modelDraft.input_price,
        output_price: modelDraft.output_price,
        priority: modelDraft.priority,
        capabilities: modelDraft.capabilities,
        is_default: modelDraft.is_default,
        status: modelDraft.status ? 1 : 0,
      };
      if (editingModel) {
        await updateModel(editingModel.id, payload);
        toast.ok('模型已更新');
      } else {
        await createModel(payload);
        toast.ok('模型已创建');
      }
      setModelOpen(false);
      await load();
    } catch (e) {
      toast.err(errMsg(e));
    } finally {
      setSaving(false);
    }
  };

  /* ---- 删除 ---- */

  const confirmDelete = async () => {
    if (!delTarget) return;
    setDeleting(true);
    try {
      if (delTarget.kind === 'provider') await deleteProvider(delTarget.id);
      else await deleteModel(delTarget.id);
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
        title="模型网关"
        desc="管理上游模型供应商与模型配置；API Key 由服务端 AES-GCM 加密存储，仅回显打码摘要（前2位+****+后4位）"
      />

      {loading ? (
        <div className="flex justify-center py-24">
          <Spinner className="h-6 w-6 text-lime" />
        </div>
      ) : (
        <div className="space-y-6">
          {/* 区块一：模型供应商 */}
          <section className="panel overflow-hidden">
            <div className="flex items-center justify-between border-b border-smoke/60 px-4 py-3">
              <h2 className="section-title">模型供应商</h2>
              <Button onClick={openProviderCreate}>＋ 新增供应商</Button>
            </div>
            {providers.length === 0 ? (
              <EmptyState
                title="暂无供应商"
                hint="先接入一家模型供应商（如 deepseek / qwen），再配置具体模型"
                action={<Button onClick={openProviderCreate}>新增供应商</Button>}
              />
            ) : (
              <div className="overflow-x-auto">
                <table className="w-full text-left text-sm">
                  <thead>
                    <tr className="border-b border-smoke/60 text-xs text-ash">
                      <th className="px-4 py-2.5 font-medium">Code</th>
                      <th className="px-4 py-2.5 font-medium">名称</th>
                      <th className="px-4 py-2.5 font-medium">Base URL</th>
                      <th className="px-4 py-2.5 font-medium">API Key</th>
                      <th className="px-4 py-2.5 font-medium">状态</th>
                      <th className="px-4 py-2.5 font-medium">更新时间</th>
                      <th className="px-4 py-2.5 text-right font-medium">操作</th>
                    </tr>
                  </thead>
                  <tbody className="divide-y divide-smoke/40">
                    {providers.map((p) => (
                      <tr key={p.id} className="transition-colors hover:bg-graphite/40">
                        <td className="px-4 py-2.5 font-mono text-lime">{p.code}</td>
                        <td className="px-4 py-2.5 text-bone">{p.name}</td>
                        <td className="px-4 py-2.5">
                          <span
                            className="block max-w-[240px] truncate font-mono text-xs text-fog"
                            title={p.base_url}
                          >
                            {p.base_url}
                          </span>
                        </td>
                        <td className="px-4 py-2.5">
                          {p.api_key_masked ? (
                            <span
                              className="font-mono text-xs text-fog"
                              title="已配置（加密存储，仅展示打码摘要）"
                            >
                              {p.api_key_masked}
                            </span>
                          ) : (
                            <Badge tone="gray">未配置</Badge>
                          )}
                        </td>
                        <td className="px-4 py-2.5">
                          <EnabledBadge status={p.status} />
                        </td>
                        <td className="px-4 py-2.5 text-xs text-fog">
                          {relativeTime(p.updated_at) || '—'}
                        </td>
                        <td className="px-4 py-2.5">
                          <div className="flex justify-end gap-3 text-xs">
                            <button
                              className="text-ash transition-colors hover:text-lime"
                              onClick={() => openProviderEdit(p)}
                            >
                              编辑
                            </button>
                            <button
                              className="text-ash transition-colors hover:text-coral"
                              onClick={() =>
                                setDelTarget({ kind: 'provider', id: p.id, label: p.name })
                              }
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

          {/* 区块二：模型配置 */}
          <section className="panel overflow-hidden">
            <div className="flex items-center justify-between border-b border-smoke/60 px-4 py-3">
              <h2 className="section-title">模型配置</h2>
              <Button onClick={openModelCreate}>＋ 新增模型</Button>
            </div>
            {models.length === 0 ? (
              <EmptyState
                title="暂无模型配置"
                hint={providers.length === 0 ? '请先添加供应商，再为平台注册可用模型' : '为平台注册可用的对话 / 向量 / 重排模型'}
                action={<Button onClick={openModelCreate}>新增模型</Button>}
              />
            ) : (
              <div className="overflow-x-auto">
                <table className="w-full text-left text-sm">
                  <thead>
                    <tr className="border-b border-smoke/60 text-xs text-ash">
                      <th className="px-4 py-2.5 font-medium">别名</th>
                      <th className="px-4 py-2.5 font-medium">模型名称</th>
                      <th className="px-4 py-2.5 font-medium">类型</th>
                      <th className="px-4 py-2.5 font-medium">供应商</th>
                      <th className="px-4 py-2.5 font-medium">上下文窗口</th>
                      <th className="px-4 py-2.5 font-medium">输入价(元/1M)</th>
                      <th className="px-4 py-2.5 font-medium">输出价(元/1M)</th>
                      <th className="px-4 py-2.5 font-medium">优先级</th>
                      <th className="px-4 py-2.5 font-medium">默认</th>
                      <th className="px-4 py-2.5 font-medium">状态</th>
                      <th className="px-4 py-2.5 text-right font-medium">操作</th>
                    </tr>
                  </thead>
                  <tbody className="divide-y divide-smoke/40">
                    {models.map((m) => (
                      <tr key={m.id} className="transition-colors hover:bg-graphite/40">
                        <td className="px-4 py-2.5 text-bone">{m.alias || '—'}</td>
                        <td className="px-4 py-2.5 font-mono text-xs text-mist">{m.model_name}</td>
                        <td className="px-4 py-2.5">
                          <Badge tone={TYPE_TONE[m.type] ?? 'gray'}>
                            {TYPE_LABEL[m.type] ?? m.type}
                          </Badge>
                        </td>
                        <td className="px-4 py-2.5 text-fog">{m.provider?.name ?? '—'}</td>
                        <td className="px-4 py-2.5 font-mono text-xs text-fog">{m.context_window}</td>
                        <td className="px-4 py-2.5 font-mono text-xs text-fog">{m.input_price}</td>
                        <td className="px-4 py-2.5 font-mono text-xs text-fog">{m.output_price}</td>
                        <td className="px-4 py-2.5 font-mono text-xs text-fog">{m.priority}</td>
                        <td className="px-4 py-2.5">
                          {m.is_default ? <Badge tone="lime">默认</Badge> : <span className="text-xs text-ash">—</span>}
                        </td>
                        <td className="px-4 py-2.5">
                          <EnabledBadge status={m.status} />
                        </td>
                        <td className="px-4 py-2.5">
                          <div className="flex justify-end gap-3 text-xs">
                            <button
                              className="text-ash transition-colors hover:text-lime"
                              onClick={() => openModelEdit(m)}
                            >
                              编辑
                            </button>
                            <button
                              className="text-ash transition-colors hover:text-coral"
                              onClick={() =>
                                setDelTarget({ kind: 'model', id: m.id, label: m.alias || m.model_name })
                              }
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

      {/* 新增 / 编辑供应商 */}
      <Modal
        open={providerOpen}
        title={editingProvider ? '编辑供应商' : '新增供应商'}
        onClose={() => setProviderOpen(false)}
        footer={
          <>
            <Button variant="ghost" onClick={() => setProviderOpen(false)}>
              取消
            </Button>
            <Button loading={saving} onClick={() => void submitProvider()}>
              保存
            </Button>
          </>
        }
      >
        <div className="space-y-3">
          <div className="grid grid-cols-2 gap-3">
            <div>
              <label className="field-label">Code（唯一标识）</label>
              <Input
                value={providerDraft.code}
                placeholder="如 deepseek"
                onChange={(e) => patchProvider({ code: e.target.value })}
              />
            </div>
            <div>
              <label className="field-label">名称</label>
              <Input
                value={providerDraft.name}
                placeholder="如 DeepSeek"
                onChange={(e) => patchProvider({ name: e.target.value })}
              />
            </div>
          </div>
          <div>
            <label className="field-label">Base URL</label>
            <Input
              value={providerDraft.base_url}
              placeholder="如 https://api.deepseek.com/v1"
              onChange={(e) => patchProvider({ base_url: e.target.value })}
            />
          </div>
          <div>
            <label className="field-label">API Key</label>
            {/* AES-GCM 加密存储；编辑时留空表示不修改 */}
            <Input
              type="password"
              autoComplete="new-password"
              value={providerDraft.api_key}
              placeholder={editingProvider ? '留空表示不修改' : 'sk-...'}
              onChange={(e) => patchProvider({ api_key: e.target.value })}
            />
          </div>
          <div>
            <label className="flex cursor-pointer items-center gap-2 text-sm text-mist">
              <input
                type="checkbox"
                className="accent-lime"
                checked={providerDraft.status}
                onChange={(e) => patchProvider({ status: e.target.checked })}
              />
              启用该供应商（停用后其下模型不可用）
            </label>
          </div>
        </div>
      </Modal>

      {/* 新增 / 编辑模型 */}
      <Modal
        open={modelOpen}
        title={editingModel ? '编辑模型' : '新增模型'}
        onClose={() => setModelOpen(false)}
        width="max-w-2xl"
        footer={
          <>
            <Button variant="ghost" onClick={() => setModelOpen(false)}>
              取消
            </Button>
            <Button loading={saving} onClick={() => void submitModel()}>
              保存
            </Button>
          </>
        }
      >
        <div className="grid grid-cols-2 gap-3">
          <div className="col-span-2">
            <label className="field-label">所属供应商</label>
            <Select
              value={modelDraft.provider_id}
              onChange={(e) => patchModel({ provider_id: e.target.value })}
            >
              <option value="">请选择供应商</option>
              {providers.map((p) => (
                <option key={p.id} value={String(p.id)}>
                  {p.name}（{p.code}）
                </option>
              ))}
            </Select>
          </div>
          <div>
            <label className="field-label">模型名称 *</label>
            <Input
              value={modelDraft.model_name}
              placeholder="上游模型名，如 deepseek-chat"
              onChange={(e) => patchModel({ model_name: e.target.value })}
            />
          </div>
          <div>
            <label className="field-label">平台别名</label>
            <Input
              value={modelDraft.alias}
              placeholder="如 DeepSeek V3"
              onChange={(e) => patchModel({ alias: e.target.value })}
            />
          </div>
          <div>
            <label className="field-label">类型</label>
            <Select
              value={modelDraft.type}
              onChange={(e) => patchModel({ type: e.target.value as ModelType })}
            >
              <option value="chat">对话（chat）</option>
              <option value="embedding">向量（embedding）</option>
              <option value="rerank">重排（rerank）</option>
            </Select>
          </div>
          <div>
            <label className="field-label">优先级（小者优先）</label>
            <Input
              type="number"
              value={modelDraft.priority}
              onChange={(e) => patchModel({ priority: toNum(e.target.value) })}
            />
          </div>
          <div>
            <label className="field-label">上下文窗口</label>
            <Input
              type="number"
              value={modelDraft.context_window}
              onChange={(e) => patchModel({ context_window: toNum(e.target.value) })}
            />
          </div>
          <div>
            <label className="field-label">最大输出</label>
            <Input
              type="number"
              value={modelDraft.max_output}
              onChange={(e) => patchModel({ max_output: toNum(e.target.value) })}
            />
          </div>
          <div>
            <label className="field-label">输入价（元/1M tokens）</label>
            <Input
              type="number"
              step="0.000001"
              value={modelDraft.input_price}
              onChange={(e) => patchModel({ input_price: toNum(e.target.value) })}
            />
          </div>
          <div>
            <label className="field-label">输出价（元/1M tokens）</label>
            <Input
              type="number"
              step="0.000001"
              value={modelDraft.output_price}
              onChange={(e) => patchModel({ output_price: toNum(e.target.value) })}
            />
          </div>
          <div className="col-span-2">
            <label className="field-label">能力</label>
            <div className="flex gap-5">
              {CAP_DEFS.map((cap) => (
                <label key={cap.key} className="flex cursor-pointer items-center gap-1.5 text-sm text-mist">
                  <input
                    type="checkbox"
                    className="accent-lime"
                    checked={modelDraft.capabilities.includes(cap.key)}
                    onChange={() => toggleCap(cap.key)}
                  />
                  {cap.label}
                </label>
              ))}
            </div>
          </div>
          <div className="col-span-2">
            <label className="flex cursor-pointer items-center gap-2 text-sm text-mist">
              <input
                type="checkbox"
                className="accent-lime"
                checked={modelDraft.is_default}
                onChange={(e) => patchModel({ is_default: e.target.checked })}
              />
              设为默认模型
            </label>
          </div>
          <div className="col-span-2">
            <label className="flex cursor-pointer items-center gap-2 text-sm text-mist">
              <input
                type="checkbox"
                className="accent-lime"
                checked={modelDraft.status}
                onChange={(e) => patchModel({ status: e.target.checked })}
              />
              启用该模型（停用后不会被解析为默认/备用模型）
            </label>
          </div>
        </div>
      </Modal>

      {/* 删除二次确认 */}
      <ConfirmDialog
        open={delTarget !== null}
        title={delTarget?.kind === 'provider' ? '删除供应商' : '删除模型'}
        content={
          delTarget
            ? delTarget.kind === 'provider'
              ? `确认删除供应商「${delTarget.label}」？其下模型配置可能一并失效，该操作不可恢复。`
              : `确认删除模型「${delTarget.label}」？该操作不可恢复。`
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
