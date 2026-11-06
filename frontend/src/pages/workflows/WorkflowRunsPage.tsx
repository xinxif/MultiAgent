/**
 * 工作流执行记录页 —— 分页列表 / 行展开查看步骤轨迹 / HITL 审批。
 *
 * 职责：
 * 1. 分页拉取执行记录（20 条/页），可按 workflow_id 过滤；
 * 2. 行点击展开：懒加载 getRun 详情，展示节点级步骤时间线
 *    （node_key / node_type / status / tokens / 起止时间 / 错误）；
 * 3. HITL：waiting_approval 的行高亮并提供「审批」入口，
 *    通过 / 驳回（可附审批意见）成功后刷新列表。
 */
import { useCallback, useEffect, useState } from 'react';
import { clsx } from 'clsx';
import { approveRun, getRun, listRuns } from '@/api/workflow';
import type { WorkflowRun, WorkflowStepRun } from '@/types/workflow';
import { Badge, Button, EmptyState, Input, PageHeader, Spinner, StatusBadge, Textarea } from '@/components/ui/primitives';
import { Modal } from '@/components/ui/Modal';
import { fullTime, tokenCount, truncate } from '@/lib/format';
import { toast } from '@/stores/ui';
import { errMsg } from '@/lib/request';

/** 每页条数，与后端默认分页保持一致 */
const PAGE_SIZE = 20;

/** 步骤时间线：节点级执行快照（只读展示） */
function StepTimeline({ steps }: { steps: WorkflowStepRun[] }) {
  if (steps.length === 0) {
    return <p className="text-xs text-ash">暂无步骤轨迹</p>;
  }
  return (
    <div className="space-y-3 border-l border-smoke/60 pl-4">
      {steps.map((s) => (
        <div key={s.id} className="relative">
          {/* 时间线节点圆点 */}
          <span className="absolute -left-[21px] top-1.5 h-2 w-2 rounded-full bg-smoke" />
          <div className="flex flex-wrap items-center gap-2 text-xs">
            <span className="font-mono font-medium text-lime">{s.node_key}</span>
            <Badge tone="violet">{s.node_type}</Badge>
            <StatusBadge status={s.status} />
            <span className="text-ash">tokens {tokenCount(s.tokens)}</span>
          </div>
          <div className="mt-1 flex flex-wrap items-center gap-3 text-[11px] text-ash">
            <span>开始 {fullTime(s.started_at)}</span>
            <span>结束 {fullTime(s.finished_at)}</span>
          </div>
          {s.error && <p className="mt-1 text-xs text-coral">✕ {s.error}</p>}
        </div>
      ))}
    </div>
  );
}

export function WorkflowRunsPage() {
  /* ---- 列表 / 分页 / 过滤 ---- */
  const [list, setList] = useState<WorkflowRun[]>([]);
  const [total, setTotal] = useState(0);
  const [page, setPage] = useState(1);
  const [loading, setLoading] = useState(true);
  const [filterInput, setFilterInput] = useState(''); // 输入框原始值
  const [wfFilter, setWfFilter] = useState<number | undefined>(undefined); // 已生效的过滤条件

  /* ---- 行展开详情 ---- */
  const [expandedId, setExpandedId] = useState<number | null>(null);
  const [detail, setDetail] = useState<WorkflowRun | null>(null);
  const [detailLoading, setDetailLoading] = useState(false);

  /* ---- HITL 审批 ---- */
  const [approveTarget, setApproveTarget] = useState<WorkflowRun | null>(null);
  const [comment, setComment] = useState('');
  const [approving, setApproving] = useState<'pass' | 'reject' | null>(null);

  /** 拉取执行记录列表：分页 + 可选 workflow_id 过滤 */
  const load = useCallback(async () => {
    setLoading(true);
    try {
      const p = await listRuns({ page, page_size: PAGE_SIZE, workflow_id: wfFilter });
      setList(p.list);
      setTotal(p.total);
    } catch (e) {
      toast.err(errMsg(e));
    } finally {
      setLoading(false);
    }
  }, [page, wfFilter]);

  useEffect(() => {
    void load();
  }, [load]);

  /** 应用过滤：空值视为清除；非正整数 ID 提示并中止 */
  const applyFilter = () => {
    const raw = filterInput.trim();
    if (raw === '') {
      setWfFilter(undefined);
    } else {
      const n = Number(raw);
      if (!Number.isInteger(n) || n <= 0) {
        toast.err('请输入有效的工作流 ID（正整数）');
        return;
      }
      setWfFilter(n);
    }
    // 过滤条件变化时回到第一页
    setPage(1);
  };

  /** 展开/收起一行：展开时懒加载含 steps 的详情 */
  const toggleExpand = async (run: WorkflowRun) => {
    if (expandedId === run.id) {
      setExpandedId(null);
      setDetail(null);
      return;
    }
    setExpandedId(run.id);
    setDetail(null);
    setDetailLoading(true);
    try {
      setDetail(await getRun(run.id));
    } catch (e) {
      toast.err(errMsg(e));
      setExpandedId(null); // 加载失败则回弹收起态
    } finally {
      setDetailLoading(false);
    }
  };

  /** 打开审批 Modal：重置意见 */
  const openApprove = (run: WorkflowRun) => {
    setApproveTarget(run);
    setComment('');
  };

  /** 提交审批：approved=true 通过 / false 驳回；成功后关闭并刷新列表 */
  const doApprove = async (approved: boolean) => {
    if (!approveTarget) return;
    setApproving(approved ? 'pass' : 'reject');
    try {
      await approveRun(approveTarget.id, approved, comment.trim() || undefined);
      toast.ok(approved ? '已通过审批，工作流继续执行' : '已驳回');
      setApproveTarget(null);
      void load();
    } catch (e) {
      toast.err(errMsg(e));
    } finally {
      setApproving(null);
    }
  };

  const hasFilter = wfFilter !== undefined;

  return (
    <div className="page-container">
      <PageHeader
        title="执行记录"
        desc="工作流每次执行的轨迹与状态，waiting_approval 的记录需要人工审批（HITL）"
        action={
          <Button variant="ghost" onClick={() => void load()}>刷新</Button>
        }
      />

      {/* 过滤栏：按 workflow_id 精确查询 */}
      <div className="mb-4 flex items-center gap-2">
        <Input
          className="w-56"
          placeholder="按 workflow_id 过滤"
          value={filterInput}
          onChange={(e) => setFilterInput(e.target.value)}
          onKeyDown={(e) => e.key === 'Enter' && applyFilter()}
        />
        <Button variant="ghost" className="px-3 py-1.5 text-xs" onClick={applyFilter}>
          查询
        </Button>
        {hasFilter && (
          // 清除过滤：还原输入框并重查全部
          <button
            className="text-xs text-ash transition-colors hover:text-lime"
            onClick={() => {
              setFilterInput('');
              setWfFilter(undefined);
              setPage(1);
            }}
          >
            ✕ 清除过滤
          </button>
        )}
      </div>

      {/* 列表 */}
      <div className="panel overflow-hidden">
        <table className="w-full text-sm">
          <thead>
            <tr className="border-b border-smoke/60 bg-carbon/60 text-left text-xs text-ash">
              <th className="w-8 px-4 py-3 font-medium" />
              <th className="px-4 py-3 font-medium">ID</th>
              <th className="px-4 py-3 font-medium">工作流</th>
              <th className="px-4 py-3 font-medium">触发方式</th>
              <th className="px-4 py-3 font-medium">状态</th>
              <th className="px-4 py-3 font-medium">创建时间</th>
              <th className="px-4 py-3 font-medium">错误</th>
              <th className="px-4 py-3 text-right font-medium">操作</th>
            </tr>
          </thead>
          <tbody>
            {loading ? (
              <tr>
                <td colSpan={8} className="px-4 py-12 text-center">
                  <Spinner className="mx-auto h-5 w-5 text-ash" />
                </td>
              </tr>
            ) : list.length === 0 ? (
              <tr>
                <td colSpan={8}>
                  <EmptyState
                    icon="⟳"
                    title="暂无执行记录"
                    hint={hasFilter ? '当前过滤条件下没有记录，试试清除过滤' : '去工作流页执行一次，或等待定时/接口触发'}
                  />
                </td>
              </tr>
            ) : (
              list.map((run) => {
                const waiting = run.status === 'waiting_approval';
                const expanded = expandedId === run.id;
                return (
                  <tr
                    key={run.id}
                    className={clsx(
                      'cursor-pointer border-b border-smoke/40 align-top transition-colors last:border-b-0 hover:bg-graphite/40',
                      // HITL 待审批行：荧光绿左边框 + 微弱底色高亮
                      waiting && 'border-l-2 border-l-lime/40 bg-lime/[0.04]',
                    )}
                    onClick={() => void toggleExpand(run)}
                  >
                    {/* 展开指示箭头 */}
                    <td className="px-4 py-3 text-xs text-ash">{expanded ? '▾' : '▸'}</td>
                    <td className="px-4 py-3 font-mono text-xs text-fog">#{run.id}</td>
                    <td className="px-4 py-3 font-mono text-xs text-fog">#{run.workflow_id}</td>
                    <td className="px-4 py-3 text-xs text-fog">{run.trigger_type}</td>
                    <td className="px-4 py-3"><StatusBadge status={run.status} /></td>
                    <td className="px-4 py-3 text-xs text-ash">{fullTime(run.created_at)}</td>
                    <td className="max-w-[14rem] px-4 py-3 text-xs text-coral" title={run.error}>
                      {run.error ? truncate(run.error, 30) : '—'}
                    </td>
                    <td className="px-4 py-3 text-right">
                      {/* 审批入口：阻断行点击冒泡，避免同时触发展开 */}
                      {waiting && (
                        <button
                          className="text-xs text-lime transition-colors hover:underline"
                          onClick={(e) => {
                            e.stopPropagation();
                            openApprove(run);
                          }}
                        >
                          审批
                        </button>
                      )}
                    </td>
                  </tr>
                );
              })
            )}
          </tbody>
        </table>

        {/* 行展开区：单独渲染在表格下方，避免破坏表格行结构 */}
        {expandedId !== null && (
          <div className="border-t border-smoke/60 bg-void/40 px-6 py-4">
            {detailLoading ? (
              <div className="flex items-center gap-2 text-xs text-ash">
                <Spinner className="h-3.5 w-3.5" />
                加载执行轨迹…
              </div>
            ) : detail ? (
              <div className="space-y-3">
                <div className="flex items-center gap-2">
                  <span className="section-title">执行轨迹 · #{detail.id}</span>
                  <StatusBadge status={detail.status} />
                  {detail.status === 'waiting_approval' && (
                    <button
                      className="text-xs text-lime transition-colors hover:underline"
                      onClick={() => openApprove(detail)}
                    >
                      去审批
                    </button>
                  )}
                </div>
                <StepTimeline steps={detail.steps ?? []} />
              </div>
            ) : null}
          </div>
        )}
      </div>

      {/* 分页 */}
      <div className="mt-4 flex items-center justify-between text-xs text-ash">
        <span>共 {total} 条 · 第 {page} 页</span>
        <div className="flex gap-2">
          <Button variant="ghost" className="px-3 py-1.5 text-xs" disabled={page <= 1} onClick={() => setPage((p) => p - 1)}>
            上一页
          </Button>
          <Button
            variant="ghost"
            className="px-3 py-1.5 text-xs"
            disabled={page * PAGE_SIZE >= total}
            onClick={() => setPage((p) => p + 1)}
          >
            下一页
          </Button>
        </div>
      </div>

      {/* HITL 审批 Modal：通过 / 驳回，意见可选 */}
      <Modal
        open={approveTarget !== null}
        title="人工审批"
        onClose={() => setApproveTarget(null)}
        footer={
          <>
            <Button variant="ghost" onClick={() => setApproveTarget(null)}>
              取消
            </Button>
            {/* 驳回为危险操作；任一操作进行中时全部禁用，防止并发提交 */}
            <Button
              variant="danger"
              loading={approving === 'reject'}
              disabled={approving !== null}
              onClick={() => void doApprove(false)}
            >
              驳回
            </Button>
            <Button loading={approving === 'pass'} disabled={approving !== null} onClick={() => void doApprove(true)}>
              通过
            </Button>
          </>
        }
      >
        {approveTarget && (
          <div className="space-y-3">
            {/* run 概要 */}
            <div className="space-y-1.5 text-xs text-fog">
              <p>
                执行 <span className="font-mono text-bone">#{approveTarget.id}</span>
                <span className="mx-2 text-ash">·</span>
                工作流 <span className="font-mono text-bone">#{approveTarget.workflow_id}</span>
              </p>
              <p>
                状态：<StatusBadge status={approveTarget.status} />
                <span className="mx-2 text-ash">·</span>
                触发方式：{approveTarget.trigger_type}
              </p>
              <p>创建时间：{fullTime(approveTarget.created_at)}</p>
              {approveTarget.error && <p className="text-coral">✕ {approveTarget.error}</p>}
              {/* 入参回显，辅助审批人判断 */}
              {Object.keys(approveTarget.input).length > 0 && (
                <pre className="code-block max-h-32">{JSON.stringify(approveTarget.input, null, 2)}</pre>
              )}
            </div>
            <div>
              <label className="field-label">审批意见（可选）</label>
              <Textarea
                rows={3}
                placeholder="补充说明会随审批结果记录"
                value={comment}
                onChange={(e) => setComment(e.target.value)}
              />
            </div>
          </div>
        )}
      </Modal>
    </div>
  );
}
