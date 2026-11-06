/**
 * 输入区 —— Enter 发送 / Shift+Enter 换行；流式中显示停止按钮。
 * 左下角提供"本次发送使用的模型"选择器：默认跟随 Agent 配置，
 * 选择具体模型后仅对本次发送生效（不改 Agent 绑定）。
 */
import { useEffect, useRef, useState, type KeyboardEvent } from 'react';
import { clsx } from 'clsx';
import { listChatModels } from '@/api/chat';
import { Select } from '@/components/ui/primitives';
import { useChat } from '@/stores/chat';
import type { ChatModelItem } from '@/types/chat';

export function Composer({ disabled }: { disabled?: boolean }) {
  const { send, stop, streaming, activeModelId, setActiveModel } = useChat();
  const [value, setValue] = useState('');
  const [models, setModels] = useState<ChatModelItem[]>([]);
  const taRef = useRef<HTMLTextAreaElement>(null);
  const busy = streaming !== null || disabled;

  // 拉取可用对话模型（启用中的模型+供应商，后端已过滤）
  useEffect(() => {
    void listChatModels()
      .then(setModels)
      .catch(() => setModels([]));
  }, []);

  const submit = () => {
    const text = value.trim();
    if (!text || busy) return;
    setValue('');
    void send(text);
  };

  const onKeyDown = (e: KeyboardEvent<HTMLTextAreaElement>) => {
    if (e.key === 'Enter' && !e.shiftKey && !e.nativeEvent.isComposing) {
      // 中文输入法组词中的回车不发送
      e.preventDefault();
      submit();
    }
  };

  return (
    <div className="border-t border-smoke/60 bg-carbon/50 p-4">
      {/* 模型选择行：让用户明确知道本次发送用哪个模型 */}
      <div className="mx-auto mb-2 flex w-full max-w-3xl items-center gap-2">
        <span className="section-title shrink-0">本次模型</span>
        <Select
          className="h-7 w-auto min-w-56 py-0 text-xs"
          value={activeModelId ?? ''}
          disabled={busy}
          onChange={(e) => {
            const v = e.target.value;
            setActiveModel(v === '' ? undefined : Number(v));
          }}
        >
          <option value="">跟随 Agent 配置</option>
          {models.map((m) => (
            <option key={m.id} value={m.id}>
              {m.label} · {m.provider}
              {m.is_default ? '（默认）' : ''}
            </option>
          ))}
        </Select>
        {models.length === 0 && (
          <span className="text-[10px] text-ash">暂无可用模型，请在「模型管理」中配置</span>
        )}
      </div>
      <div
        className={clsx(
          'mx-auto flex w-full max-w-3xl items-end gap-2 rounded-xl border bg-graphite p-2 transition-colors',
          busy ? 'border-smoke/60' : 'border-smoke focus-within:border-lime/50',
        )}
      >
        <textarea
          ref={taRef}
          rows={1}
          value={value}
          disabled={busy}
          placeholder={busy ? '对话生成中…' : '输入消息，Enter 发送，Shift+Enter 换行'}
          className="max-h-40 min-h-[38px] flex-1 resize-none bg-transparent px-2 py-2 text-sm text-bone outline-none placeholder:text-ash disabled:opacity-50"
          onChange={(e) => setValue(e.target.value)}
          onKeyDown={onKeyDown}
          // 自适应高度
          onInput={(e) => {
            const el = e.currentTarget;
            el.style.height = 'auto';
            el.style.height = `${Math.min(el.scrollHeight, 160)}px`;
          }}
        />
        {streaming ? (
          <button
            className="btn-danger h-9 shrink-0 px-3"
            onClick={stop}
            title="停止生成"
          >
            ■ 停止
          </button>
        ) : (
          <button
            className="btn-primary h-9 shrink-0 px-4"
            disabled={!value.trim()}
            onClick={submit}
          >
            发送 ➤
          </button>
        )}
      </div>
    </div>
  );
}
