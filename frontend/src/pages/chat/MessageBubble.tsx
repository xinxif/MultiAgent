/**
 * 消息气泡 —— 三种形态统一渲染：
 * 1. 用户消息：右侧荧光绿描边；
 * 2. 历史 assistant：Markdown 正文 + 引用角标 + 决策链（metadata.agent_trace）+ 用量脚注；
 * 3. 流式 assistant：与历史同构，内容由 SSE 增量驱动，正文带打字机光标。
 */
import { memo, useEffect, useRef, type ReactNode } from 'react';
import ReactMarkdown from 'react-markdown';
import remarkGfm from 'remark-gfm';
import { clsx } from 'clsx';
import { bubbleIn } from '@/lib/gsap';
import type { Message } from '@/types/chat';
import type { TraceStep } from '@/types/chat';
import type { StreamingMsg } from '@/stores/chat';
import { Collapse, TraceTimeline, streamingToSteps } from './TraceTimeline';

/* ---- 引用列表（refs 事件 / 消息 refs 字段） ---- */

function RefsList({ refs }: { refs: { filename: string; page: number; content: string; score: number }[] }) {
  if (!refs?.length) return null;
  return (
    <Collapse title={`引用来源`} badge={`${refs.length} 条`}>
      <ul className="space-y-2">
        {refs.map((r, i) => (
          <li key={`${r.filename}-${i}`} className="rounded-md border border-smoke/50 bg-void/60 p-2 text-xs">
            <div className="mb-1 flex items-center gap-2 text-fog">
              <span className="font-mono text-lime">[{i + 1}]</span>
              <span className="truncate">{r.filename}</span>
              {r.page > 0 && <span className="text-ash">p.{r.page}</span>}
              <span className="ml-auto tabular-nums text-ash">{r.score.toFixed(3)}</span>
            </div>
            <p className="line-clamp-3 leading-relaxed text-ash">{r.content}</p>
          </li>
        ))}
      </ul>
    </Collapse>
  );
}

/* ---- 决策链折叠区 ---- */

function TraceCollapse({ steps }: { steps: TraceStep[] }) {
  if (!steps?.length) return null;
  const errCount = steps.filter((s) => s.status === 'error' || s.status === 'timeout').length;
  return (
    <Collapse
      title="决策链"
      badge={`${steps.length} 步${errCount > 0 ? ` · ${errCount} 失败` : ''}`}
    >
      <TraceTimeline steps={steps} />
    </Collapse>
  );
}

/* ---- Markdown 正文 ---- */

const MD_COMPONENTS = {
  // 代码块：设计系统风格
  pre: ({ children }: { children?: ReactNode }) => (
    <pre className="code-block my-2">{children}</pre>
  ),
  a: ({ href, children }: { href?: string; children?: ReactNode }) => (
    <a href={href} target="_blank" rel="noreferrer" className="text-lime underline-offset-2 hover:underline">
      {children}
    </a>
  ),
};

function Markdown({ content }: { content: string }) {
  return (
    <div className="prose-invert space-y-2 text-sm leading-relaxed text-mist [&_li]:ml-4 [&_ol]:list-decimal [&_p]:my-1 [&_ul]:list-disc">
      <ReactMarkdown remarkPlugins={[remarkGfm]} components={MD_COMPONENTS}>
        {content}
      </ReactMarkdown>
    </div>
  );
}

/* ---- 用量脚注 ---- */

function UsageFoot({
  promptTokens,
  completionTokens,
  model,
  latencyMs,
}: {
  promptTokens?: number;
  completionTokens?: number;
  model?: string;
  latencyMs?: number;
}) {
  const parts = [
    model,
    promptTokens ? `入 ${promptTokens}` : '',
    completionTokens ? `出 ${completionTokens}` : '',
    latencyMs ? `${(latencyMs / 1000).toFixed(1)}s` : '',
  ].filter(Boolean);
  if (parts.length === 0) return null;
  return (
    <div className="mt-2 flex gap-3 border-t border-smoke/40 pt-2 font-mono text-[10px] text-ash">
      {parts.map((p) => (
        <span key={p}>{p}</span>
      ))}
    </div>
  );
}

/* ---- 气泡主体 ---- */

interface BubbleProps {
  role: 'user' | 'assistant';
  content: string;
  reasoning?: string;
  refs?: Message['refs'];
  traceSteps?: TraceStep[];
  usage?: { promptTokens?: number; completionTokens?: number; model?: string };
  latencyMs?: number;
  /** 流式中的打字机光标 */
  streaming?: boolean;
  expert?: string;
}

export const MessageBubble = memo(function MessageBubble({
  role,
  content,
  reasoning,
  refs,
  traceSteps,
  usage,
  latencyMs,
  streaming,
  expert,
}: BubbleProps) {
  const isUser = role === 'user';
  const rootRef = useRef<HTMLDivElement>(null);

  // 入场动画：仅在挂载时执行一次（流式更新重渲染不会重播）
  useEffect(() => {
    const el = rootRef.current;
    if (el) bubbleIn(el);
  }, []);

  return (
    <div
      ref={rootRef}
      className={clsx('flex w-full gap-3', isUser ? 'justify-end' : 'justify-start')}
    >
      <div
        className={clsx(
          'max-w-[82%] rounded-xl border px-4 py-3',
          isUser
            ? 'border-lime/30 bg-lime/5'
            : 'border-smoke/60 bg-obsidian',
        )}
      >
        {/* 多智能体命中专家标记 */}
        {!isUser && expert && (
          <div className="mb-2 text-[10px] text-violet">⇢ 由「{expert}」处理</div>
        )}

        {/* 正文 */}
        {content ? (
          isUser ? (
            <p className="whitespace-pre-wrap text-sm leading-relaxed text-bone">{content}</p>
          ) : (
            <div className={streaming ? 'caret' : ''}>
              <Markdown content={content} />
            </div>
          )
        ) : (
          streaming && (
            <p className="text-sm text-fog">
              <span className="inline-block h-3 w-1.5 animate-pulse bg-lime align-middle" />
              思考中…
            </p>
          )
        )}

        {/* 推理过程（reasoning 模型才有） */}
        {!isUser && reasoning && (
          <div className="mt-2">
            <Collapse title="思考过程">{reasoning}</Collapse>
          </div>
        )}

        {/* 决策链（工具调用时间线） */}
        {!isUser && traceSteps && traceSteps.length > 0 && (
          <div className="mt-2">
            <TraceCollapse steps={traceSteps} />
          </div>
        )}

        {/* RAG 引用 */}
        {!isUser && refs && refs.length > 0 && (
          <div className="mt-2">
            <RefsList refs={refs} />
          </div>
        )}

        {/* 用量脚注 */}
        {!isUser && !streaming && (
          <UsageFoot
            promptTokens={usage?.promptTokens}
            completionTokens={usage?.completionTokens}
            model={usage?.model}
            latencyMs={latencyMs}
          />
        )}
      </div>
    </div>
  );
});

/** 历史消息 → 气泡 props 适配 */
export function HistoryBubble({ msg }: { msg: Message }) {
  const isUser = msg.role === 'user';
  return (
    <MessageBubble
      role={isUser ? 'user' : 'assistant'}
      content={msg.content}
      reasoning={msg.reasoning_content || undefined}
      refs={msg.refs ?? undefined}
      traceSteps={msg.metadata?.agent_trace}
      usage={
        isUser
          ? undefined
          : {
              promptTokens: msg.prompt_tokens,
              completionTokens: msg.completion_tokens,
              model: msg.model_name,
            }
      }
      latencyMs={msg.latency_ms}
    />
  );
}

/** 流式消息 → 气泡 props 适配 */
export function StreamingBubble({ s }: { s: StreamingMsg }) {
  return (
    <MessageBubble
      role="assistant"
      content={s.content}
      reasoning={s.reasoning || undefined}
      refs={s.refs.length ? s.refs : undefined}
      traceSteps={streamingToSteps(s.tools)}
      usage={
        s.usage
          ? {
              promptTokens: s.usage.prompt_tokens,
              completionTokens: s.usage.completion_tokens,
              model: s.usage.model,
            }
          : undefined
      }
      streaming={s.status === 'streaming'}
      expert={s.expert}
    />
  );
}
