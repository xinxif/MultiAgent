/**
 * 对话页 —— 主界面。
 * 布局：会话列表 | 消息流 + 计划卡片 + 输入区 | 右侧 Agent 选择。
 *
 * SSE 事件 → 状态的全部逻辑在 stores/chat.ts，这里只做展示；
 * 历史消息与流式消息共用 MessageBubble（同构渲染）。
 */
import { useEffect, useRef, useState } from 'react';
import { useChat } from '@/stores/chat';
import { listAgents } from '@/api/agent';
import type { Agent } from '@/types/agent';
import { SessionList } from './SessionList';
import { Composer } from './Composer';
import { HistoryBubble, StreamingBubble } from './MessageBubble';
import { Badge, EmptyState } from '@/components/ui/primitives';
import { clsx } from 'clsx';

/** 计划卡片（plan 事件）：目标 + 步骤清单 */
function PlanCard() {
  const plan = useChat((s) => s.streaming?.plan);
  if (!plan) return null;
  return (
    <div className="panel mx-auto w-full max-w-3xl border-lime/20 p-4">
      <div className="mb-2 flex items-center gap-2">
        <Badge tone="lime">执行计划</Badge>
        <span className="text-sm font-medium text-bone">{plan.goal}</span>
      </div>
      <ol className="space-y-1.5">
        {plan.steps?.map((step, i) => (
          <li key={step.id} className="flex items-baseline gap-2 text-sm text-fog">
            <span className="font-mono text-xs text-lime">{i + 1}.</span>
            <span>{step.description}</span>
            {step.depends_on?.length > 0 && (
              <span className="text-[10px] text-ash">依赖 {step.depends_on.join(', ')}</span>
            )}
          </li>
        ))}
      </ol>
    </div>
  );
}

/** 右侧：Agent 选择器 */
function AgentPanel() {
  const { activeAgentId, setActiveAgent } = useChat();
  const [agents, setAgents] = useState<Agent[]>([]);

  useEffect(() => {
    void listAgents({ page: 1, page_size: 100 })
      .then((p) => setAgents(p.list))
      .catch(() => setAgents([]));
  }, []);

  return (
    <aside className="hidden h-full w-56 shrink-0 flex-col gap-2 overflow-y-auto border-l border-smoke/60 bg-carbon/50 p-3 xl:flex">
      <p className="section-title px-1 pb-1">选择 Agent</p>
      <button
        className={clsx(
          'rounded-lg px-3 py-2 text-left text-xs transition-colors',
          activeAgentId === undefined
            ? 'bg-obsidian text-lime'
            : 'text-fog hover:bg-obsidian/60',
        )}
        onClick={() => setActiveAgent(undefined)}
      >
        平台默认
      </button>
      {agents.map((a) => (
        <button
          key={a.id}
          className={clsx(
            'rounded-lg px-3 py-2 text-left transition-colors',
            activeAgentId === a.id
              ? 'bg-obsidian text-lime'
              : 'text-fog hover:bg-obsidian/60',
          )}
          onClick={() => setActiveAgent(a.id)}
        >
          <span className="block truncate text-xs">{a.name}</span>
          <span className="mt-0.5 block text-[10px] text-ash">
            {a.type === 'host' ? 'Host · 智能路由' : 'Expert · 专项'}
          </span>
        </button>
      ))}
    </aside>
  );
}

export function ChatPage() {
  const { messages, streaming } = useChat();
  const bottomRef = useRef<HTMLDivElement>(null);

  // 流式内容变化：滚动到底（打字机跟随）
  useEffect(() => {
    bottomRef.current?.scrollIntoView({ behavior: 'smooth', block: 'end' });
  }, [messages.length, streaming?.content, streaming?.tools, streaming?.plan]);

  const hasContent = messages.length > 0 || streaming !== null;

  return (
    <div className="flex h-full">
      <SessionList />

      {/* 中间：消息流 */}
      <div className="flex min-w-0 flex-1 flex-col">
        <div className="min-h-0 flex-1 space-y-4 overflow-y-auto px-6 py-6">
          {!hasContent ? (
            <div className="flex h-full items-center justify-center">
              <EmptyState
                icon="◆"
                title="开始一段新对话"
                hint="左侧选择历史会话，或直接在下方输入；右侧可切换绑定了工具与知识库的 Agent"
              />
            </div>
          ) : (
            <div className="mx-auto w-full max-w-3xl space-y-4">
              {messages.map((m) => (
                <HistoryBubble key={m.id} msg={m} />
              ))}
              {streaming?.plan && <PlanCard />}
              {streaming && <StreamingBubble s={streaming} />}
              <div ref={bottomRef} />
            </div>
          )}
        </div>
        <Composer />
      </div>

      <AgentPanel />
    </div>
  );
}
