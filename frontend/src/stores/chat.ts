/**
 * 对话状态机 —— 前端的心脏。
 *
 * 职责：
 * 1. 会话列表管理（以服务端为唯一事实源，流结束后重新拉取）；
 * 2. SSE 流式渲染：12 种事件 → UI 状态（与 handler_chat.go 协议一一对应）；
 * 3. 中断（abort）、重发（同一 session 再发一条）。
 *
 * 设计要点（本文件的事件分支与后端事件类型一一对应）：
 * - meta       建气泡占位，记录 session_id；
 * - reasoning  折叠区灰字逐字上屏；
 * - plan       计划卡片逐条勾选；
 * - tool_call  决策链时间线追加"调用中"节点；
 * - tool_result节点回填结果与耗时；
 * - agent_event多智能体委派动效；
 * - refs       引用角标数据；
 * - usage      成本角标；
 * - delta      打字机正文；
 * - done/error 终态：解禁输入框；error 显示重试条。
 */
import { create } from 'zustand';
import { runChatStream, type StreamHandle } from '@/lib/sse';
import { deleteSession, listMessages, listSessions, renameSession as renameApi } from '@/api/chat';
import { toast } from '@/stores/ui';
import { useAuth } from '@/stores/auth';
import type {
  AgentEventData,
  Message,
  PlanData,
  RAGRef,
  Session,
  ToolCallData,
  ToolResultData,
  UsageData,
} from '@/types/chat';

/** 正在流式生成的助手消息（done 后由服务端 messages 替换） */
export interface StreamingMsg {
  sessionId: string;
  content: string;
  reasoning: string;
  plan?: PlanData;
  /** tool_call.id → 节点（call 与 result 按 id 配对） */
  tools: Record<string, ToolCallData & Partial<ToolResultData>>;
  agentEvents: AgentEventData[];
  refs: RAGRef[];
  usage?: UsageData;
  /** streaming / done / error */
  status: 'streaming' | 'done' | 'error';
  errorMessage?: string;
  /** meta.expert：多智能体命中专家 */
  expert?: string;
}

interface ChatState {
  sessions: Session[];
  sessionsTotal: number;
  /** 当前会话（undefined = 新对话） */
  activeSessionId?: string;
  /** 历史消息（服务端事实源） */
  messages: Message[];
  /** 流式中的消息（null = 空闲） */
  streaming: StreamingMsg | null;
  /** 对话中的 Agent（undefined = 平台默认） */
  activeAgentId?: number;
  /** 本次发送使用的模型（对话页选择器；undefined = 沿用 Agent 绑定模型） */
  activeModelId?: number;

  loadSessions: () => Promise<void>;
  selectSession: (id: string) => Promise<void>;
  newSession: () => void;
  setActiveAgent: (id?: number) => void;
  setActiveModel: (id?: number) => void;
  send: (content: string) => Promise<void>;
  stop: () => void;
  renameSession: (id: string, title: string) => Promise<void>;
  removeSession: (id: string) => Promise<void>;
  /** 消息列表页"记住这次对话"后调用方自行刷新 */
}

let handle: StreamHandle | null = null;

export const useChat = create<ChatState>((set, get) => ({
  sessions: [],
  sessionsTotal: 0,
  activeSessionId: undefined,
  messages: [],
  streaming: null,
  activeAgentId: undefined,
  activeModelId: undefined,

  loadSessions: async () => {
    const page = await listSessions({ page: 1, page_size: 100 });
    set({ sessions: page.list, sessionsTotal: page.total });
  },

  /** 切换会话：拉历史全量渲染（幂等 —— 服务端消息列表是唯一事实源） */
  selectSession: async (id) => {
    if (get().streaming) get().stop();
    set({ activeSessionId: id, messages: [], streaming: null });
    const msgs = await listMessages(id);
    set({ messages: msgs });
  },

  /** 开新对话：清空当前上下文 */
  newSession: () => {
    if (get().streaming) get().stop();
    set({ activeSessionId: undefined, messages: [], streaming: null });
  },

  setActiveAgent: (id) => set({ activeAgentId: id }),

  setActiveModel: (id) => set({ activeModelId: id }),

  /** 发送一条消息并消费 SSE 流 */
  send: async (content) => {
    if (get().streaming) return; // 流式中禁止并发发送

    // 乐观插入用户消息（服务端确认以 done 后的 reload 为准）
    const optimistic: Message = {
      id: -Date.now(),
      session_id: get().activeSessionId ?? '',
      role: 'user',
      content,
      prompt_tokens: 0,
      completion_tokens: 0,
      first_token_ms: 0,
      latency_ms: 0,
      status: 1,
      trace_id: '',
      created_at: new Date().toISOString(),
    };
    set((s) => ({ messages: [...s.messages, optimistic], streaming: emptyStreaming() }));

    const token = useAuth.getState().accessToken ?? '';
    const h = await runChatStream(
      { session_id: get().activeSessionId, agent_id: get().activeAgentId, content, model_config_id: get().activeModelId },
      (ev) => {
        // 每个事件 → 不可变状态更新
        set((s) => (s.streaming ? { streaming: reduce(s.streaming, ev.event, ev.data) } : s));
      },
      token,
    );
    handle = h;
    await h.done; // 等流结束（done/error/中断 都会 resolve）

    // 收尾：以服务端为准刷新事实源
    const finalStatus = get().streaming?.status;
    if (finalStatus === 'error') {
      toast.err(get().streaming?.errorMessage ?? '对话失败');
    }
    handle = null;
    // 首轮对话时 activeSessionId 为空，从 meta 事件里拿服务端分配的会话 ID
    const sid = get().activeSessionId ?? get().streaming?.sessionId;
    set({ streaming: null, ...(sid ? { activeSessionId: sid } : {}) });
    if (sid) {
      // 会话标题/条数/消息内容都可能已变，全部重拉
      const [msgs, sessions] = await Promise.all([
        listMessages(sid),
        listSessions({ page: 1, page_size: 100 }),
      ]);
      set({ messages: msgs, sessions: sessions.list, sessionsTotal: sessions.total });
    } else {
      await get().loadSessions();
    }
  },

  /** 用户中断：abort 后流处理会兜底 error 事件，走统一的收尾逻辑 */
  stop: () => {
    handle?.abort();
  },

  renameSession: async (id, title) => {
    await renameApi(id, title);
    set((s) => ({
      sessions: s.sessions.map((x) => (x.id === id ? { ...x, title } : x)),
    }));
  },

  removeSession: async (id) => {
    await deleteSession(id);
    set((s) => ({
      sessions: s.sessions.filter((x) => x.id !== id),
      activeSessionId: s.activeSessionId === id ? undefined : s.activeSessionId,
      messages: s.activeSessionId === id ? [] : s.messages,
    }));
  },
}));

/** 新建流式消息骨架 */
function emptyStreaming(): StreamingMsg {
  return {
    sessionId: '',
    content: '',
    reasoning: '',
    tools: {},
    agentEvents: [],
    refs: [],
    status: 'streaming',
  };
}

/** SSE 事件归约器：单事件 → 新状态（不可变） */
function reduce(
  cur: StreamingMsg,
  event: string,
  data: Record<string, unknown>,
): StreamingMsg {
  switch (event) {
    case 'meta':
      return {
        ...cur,
        sessionId: String(data.session_id ?? ''),
        expert: data.expert ? String(data.expert) : undefined,
      };

    case 'delta':
      return { ...cur, content: cur.content + String(data.content ?? '') };

    case 'reasoning':
      return { ...cur, reasoning: cur.reasoning + String(data.content ?? '') };

    case 'plan':
      return { ...cur, plan: data as unknown as PlanData };

    case 'tool_call': {
      const tc = data as unknown as ToolCallData;
      return { ...cur, tools: { ...cur.tools, [tc.id]: { ...tc } } };
    }

    case 'tool_result': {
      const tr = data as unknown as ToolResultData;
      const node = cur.tools[tr.id];
      if (!node) return cur;
      return {
        ...cur,
        tools: { ...cur.tools, [tr.id]: { ...node, ...tr } },
      };
    }

    case 'agent_event':
      return { ...cur, agentEvents: [...cur.agentEvents, data as unknown as AgentEventData] };

    case 'refs':
      return { ...cur, refs: (data.items as RAGRef[]) ?? [] };

    case 'usage':
      return { ...cur, usage: data as unknown as UsageData };

    case 'done':
      return { ...cur, status: 'done' };

    case 'error':
      return {
        ...cur,
        status: 'error',
        errorMessage: String(data.message ?? '对话失败'),
      };

    default:
      return cur; // 未知事件静默忽略（协议向后兼容）
  }
}
