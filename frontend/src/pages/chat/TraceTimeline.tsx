/**
 * 决策链时间线 —— Agent "思考过程"可视化。
 *
 * 数据来源两路（同构渲染）：
 * 1. 历史消息：messages[].metadata.agent_trace（TraceStep[]，一次性渲染）；
 * 2. 流式消息：tool_call/tool_result 事件实时追加（由 ChatPage 组装成 TraceStep 形状）。
 *
 * 设计三原则：
 * - 默认折叠：一行摘要 + 展开箭头，思考过程对普通用户是噪音；
 * - 失败醒目：status 为 error/timeout 的节点标红；
 * - 渐进展开：进行中显示"调用中"，结束后回填结果。
 */
import { useEffect, useRef, useState, type ReactNode } from 'react';
import { clsx } from 'clsx';
import { expandToggle } from '@/lib/gsap';
import type { TraceStep } from '@/types/chat';

/** 步骤类型 → 中文标签 */
const LABEL: Record<string, string> = {
  plan: '计划',
  thought: '思考',
  action: '调用工具',
  observe: '观察',
  delegate: '委派',
  merge: '聚合',
  answer: '回答',
};

const TYPE_ICON: Record<string, string> = {
  plan: '❏',
  thought: '…',
  action: '⚡',
  observe: '◉',
  delegate: '⇢',
  merge: '⊕',
  answer: '✓',
};

/** 展开/收起的 GSAP 高度动画 hook（首渲染不动画） */
function useRefForExpand(open: boolean) {
  const ref = useRef<HTMLDivElement>(null);
  const first = useRef(true);
  useEffect(() => {
    if (first.current) {
      first.current = false;
      return;
    }
    const el = ref.current;
    if (el) expandToggle(el, open);
  }, [open]);
  return ref;
}

function StepRow({ step }: { step: TraceStep }) {
  const [open, setOpen] = useState(false);
  const bodyRef = useRefForExpand(open);
  const failed = step.status === 'error' || step.status === 'timeout';
  const running = step.status === 'running';

  return (
    <li className={clsx('relative pl-6', failed && 'text-coral')}>
      {/* 时间线圆点 */}
      <span
        className={clsx(
          'absolute left-0 top-1 flex h-4 w-4 items-center justify-center rounded-full border text-[9px]',
          failed
            ? 'border-coral/60 bg-coral/15 text-coral'
            : running
              ? 'border-lime/60 bg-lime/15 text-lime'
              : 'border-smoke bg-graphite text-fog',
        )}
      >
        {TYPE_ICON[step.type] ?? '·'}
      </span>

      {/* 标题行：类型 · 工具/Agent · 耗时 —— 可点击展开 */}
      <button
        className="flex w-full items-baseline gap-2 text-left text-xs transition-colors hover:text-bone"
        onClick={() => setOpen((v) => !v)}
      >
        <span className={clsx('font-medium', failed ? 'text-coral' : 'text-mist')}>
          {LABEL[step.type] ?? step.type}
        </span>
        {(step.tool || step.agent) && (
          <span className="font-mono text-fog">{step.tool ?? step.agent}</span>
        )}
        {step.elapsed_ms > 0 && (
          <span className={clsx('tabular-nums', failed ? 'text-coral/70' : 'text-ash')}>
            {step.elapsed_ms}ms
          </span>
        )}
        {running && <span className="animate-pulse text-lime">调用中…</span>}
        {failed && <span className="text-coral">✕ {step.status === 'timeout' ? '超时' : '失败'}</span>}
      </button>

      {/* 展开体：思考内容 / 参数 / 结果 */}
      <div ref={bodyRef} className="overflow-hidden" style={{ height: open ? 'auto' : 0 }}>
        {step.content && (
          <p className="mt-1 whitespace-pre-wrap text-xs leading-relaxed text-fog">{step.content}</p>
        )}
        {step.args && Object.keys(step.args).length > 0 && (
          <pre className="code-block mt-1 text-[11px]">{JSON.stringify(step.args, null, 2)}</pre>
        )}
        {step.result && (
          <pre className="code-block mt-1 max-h-40 text-[11px]">{step.result.slice(0, 400)}</pre>
        )}
      </div>
    </li>
  );
}

export function TraceTimeline({ steps }: { steps: TraceStep[] }) {
  if (!steps?.length) return null;
  return (
    <ol className="space-y-2.5">
      {steps.map((s) => (
        <StepRow key={s.seq ?? `${s.time}-${s.type}`} step={s} />
      ))}
    </ol>
  );
}

/** 供流式消息使用：把 tools map 转为 TraceStep 数组 */
export function streamingToSteps(
  tools: Record<
    string,
    { name: string; args: string; result?: string; ms?: number; status?: string }
  >,
): TraceStep[] {
  return Object.entries(tools).map(([id, t], i) => ({
    seq: i,
    type: 'action',
    agent: '',
    content: '',
    tool: t.name,
    args: safeParse(t.args),
    result: t.result,
    elapsed_ms: t.ms ?? 0,
    status: t.status ?? 'running',
    time: id,
  }));
}

function safeParse(s?: string): Record<string, unknown> | undefined {
  if (!s) return undefined;
  try {
    return JSON.parse(s) as Record<string, unknown>;
  } catch {
    return { raw: s };
  }
}

/** 折叠容器：带 GSAP 展开动画（"思考过程"折叠区用） */
export function Collapse({
  title,
  badge,
  children,
}: {
  title: string;
  badge?: string;
  children: ReactNode;
}) {
  const [open, setOpen] = useState(false);
  const ref = useRefForExpand(open);
  return (
    <div className="rounded-lg border border-smoke/60 bg-carbon/60">
      <button
        className="flex w-full items-center gap-2 px-3 py-2 text-xs text-fog transition-colors hover:text-mist"
        onClick={() => setOpen((v) => !v)}
      >
        <span className={clsx('transition-transform duration-200', open && 'rotate-90')}>▸</span>
        <span>{title}</span>
        {badge && <span className="text-ash">{badge}</span>}
      </button>
      <div ref={ref} className="overflow-hidden" style={{ height: open ? 'auto' : 0 }}>
        <div className="px-3 pb-3">{children}</div>
      </div>
    </div>
  );
}
