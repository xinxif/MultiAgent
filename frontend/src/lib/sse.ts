/**
 * SSE 流式解析 —— POST /chat/stream 的字节流 → 事件回调。
 *
 * 为什么不用 EventSource：浏览器原生 EventSource 只支持 GET，
 * 而对话是 POST + JSON body，因此用 fetch + ReadableStream 手动解析。
 * 帧格式与 handler_chat.go writeSSE 完全对齐：
 *
 *   event: <type>\n
 *   data: <json>\n
 *   \n            ← 空行即帧边界
 *
 * 关键坑：TCP 分包可能把一帧劈成两半，必须缓冲半帧（split 后 pop 留尾）。
 */
import type { SSEEventType } from '@/types/chat';

export interface SSEEvent {
  event: SSEEventType;
  data: Record<string, unknown>;
}

export interface StreamHandle {
  /** 中断流（用户点停止按钮） */
  abort: () => void;
  /** 流结束信号（done/error/中断/网络断 都会 resolve） */
  done: Promise<void>;
}

/**
 * 发起流式对话并逐事件回调。
 *
 * @param body   对话请求 {session_id?, agent_id?, content, model_config_id?}
 * @param onEvent 每解析出一个完整帧回调一次
 * @param token  access token（SSE 无法走 401 重放逻辑，需显式传入；
 *               调用方保证 token 新鲜——进入聊天页时已由请求层刷新过）
 */
export async function runChatStream(
  body: {
    session_id?: string;
    agent_id?: number;
    content: string;
    model_config_id?: number;
  },
  onEvent: (e: SSEEvent) => void,
  token: string,
): Promise<StreamHandle> {
  const controller = new AbortController();
  let resolveDone!: () => void;
  const done = new Promise<void>((r) => {
    resolveDone = r;
  });

  // 流处理主体：独立异步任务，结束/异常时都 resolve done
  void (async () => {
    try {
      const res = await fetch('/api/v1/chat/stream', {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
          Authorization: `Bearer ${token}`,
        },
        body: JSON.stringify(body),
        signal: controller.signal,
      });

      // 非 200：可能是 token 过期/无权限，读 JSON 包络抛给 UI
      if (res.status !== 200) {
        let msg = `连接失败（HTTP ${res.status}）`;
        try {
          const env = await res.json();
          if (env?.message) msg = env.message;
        } catch {
          /* body 不是 JSON，用默认文案 */
        }
        onEvent({ event: 'error', data: { code: res.status, message: msg } });
        return;
      }

      const reader = res.body!.getReader();
      const decoder = new TextDecoder();
      let buf = '';
      // 终态事件（done/error）是否已送达：决定流结束后要不要补发"连接中断"
      let terminal = false;

      for (;;) {
        const { done: eof, value } = await reader.read();
        if (eof) break;
        buf += decoder.decode(value, { stream: true });

        // 帧以空行分隔；最后一段可能是半帧，留到下一轮
        const frames = buf.split('\n\n');
        buf = frames.pop() ?? '';

        for (const frame of frames) {
          // 一个帧内 event: 与 data: 各一行（后端固定格式）
          const event = frame.match(/^event: (.+)$/m)?.[1] ?? '';
          const raw = frame.match(/^data: (.+)$/m)?.[1] ?? '{}';
          if (!event) continue;
          if (event === 'done' || event === 'error') terminal = true;
          try {
            onEvent({
              event: event as SSEEventType,
              data: JSON.parse(raw) as Record<string, unknown>,
            });
          } catch {
            // 单帧 JSON 损坏不影响整个流，跳过
          }
        }
      }

      // 流结束但没等到 done/error 才算异常断开（此前无条件补发，
      // 导致每次正常回答完也弹"连接中断，回答可能不完整"）
      if (!terminal) {
        onEvent({
          event: 'error',
          data: { code: -1, message: '连接中断，回答可能不完整，请重发' },
        });
      }
    } catch (e) {
      // 用户主动中断：不作为错误，静默结束（部分内容不落库，以服务端为准）
      if (!(e instanceof DOMException && e.name === 'AbortError')) {
        onEvent({
          event: 'error',
          data: { code: -1, message: '网络异常，请检查后端服务' },
        });
      }
    } finally {
      resolveDone();
    }
  })();

  return { abort: () => controller.abort(), done };
}
