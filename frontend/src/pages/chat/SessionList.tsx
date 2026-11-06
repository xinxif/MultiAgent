/**
 * 会话列表 —— 侧栏：新对话按钮 + 会话项（改名/删除）。
 */
import { useEffect, useRef, useState } from 'react';
import { clsx } from 'clsx';
import { relativeTime } from '@/lib/format';
import { useChat } from '@/stores/chat';
import { ConfirmDialog } from '@/components/ui/ConfirmDialog';

export function SessionList() {
  const {
    sessions,
    activeSessionId,
    selectSession,
    newSession,
    renameSession,
    removeSession,
    loadSessions,
  } = useChat();

  const [editingId, setEditingId] = useState<string | null>(null);
  const [draft, setDraft] = useState('');
  const [delId, setDelId] = useState<string | null>(null);
  const listRef = useRef<HTMLDivElement>(null);

  // 首次挂载拉列表 + 列表入场 stagger
  useEffect(() => {
    void loadSessions();
  }, [loadSessions]);

  const commitRename = async () => {
    if (editingId && draft.trim()) {
      await renameSession(editingId, draft.trim());
    }
    setEditingId(null);
  };

  return (
    <div className="flex h-full w-60 shrink-0 flex-col border-r border-smoke/60 bg-carbon/50">
      {/* 新对话 */}
      <div className="p-3">
        <button
          className="btn-ghost w-full justify-center border-dashed text-sm"
          onClick={newSession}
        >
          ＋ 新对话
        </button>
      </div>

      {/* 会话项 */}
      <div ref={listRef} className="min-h-0 flex-1 space-y-0.5 overflow-y-auto px-2 pb-3">
        {sessions.map((s) => {
          const active = s.id === activeSessionId;
          return (
            <div
              key={s.id}
              className={clsx(
                'group cursor-pointer rounded-lg px-3 py-2 transition-colors duration-150',
                active ? 'bg-obsidian text-mist' : 'text-fog hover:bg-obsidian/50',
              )}
              onClick={() => editingId !== s.id && void selectSession(s.id)}
            >
              {editingId === s.id ? (
                <input
                  autoFocus
                  className="field h-7 px-2 py-0 text-xs"
                  value={draft}
                  onChange={(e) => setDraft(e.target.value)}
                  onBlur={commitRename}
                  onKeyDown={(e) => e.key === 'Enter' && void commitRename()}
                  onClick={(e) => e.stopPropagation()}
                />
              ) : (
                <div className="flex items-center gap-2">
                  <div className="min-w-0 flex-1">
                    <p className="truncate text-xs leading-5">{s.title}</p>
                    <p className="text-[10px] text-ash">
                      {s.message_count} 条 · {relativeTime(s.last_msg_at ?? s.created_at)}
                    </p>
                  </div>
                  {/* 悬浮操作 */}
                  <div className="hidden shrink-0 gap-1 group-hover:flex">
                    <button
                      className="text-[10px] text-ash hover:text-lime"
                      title="重命名"
                      onClick={(e) => {
                        e.stopPropagation();
                        setEditingId(s.id);
                        setDraft(s.title);
                      }}
                    >
                      ✎
                    </button>
                    <button
                      className="text-[10px] text-ash hover:text-coral"
                      title="删除"
                      onClick={(e) => {
                        e.stopPropagation();
                        setDelId(s.id);
                      }}
                    >
                      ✕
                    </button>
                  </div>
                </div>
              )}
            </div>
          );
        })}
        {sessions.length === 0 && (
          <p className="px-3 py-8 text-center text-xs text-ash">暂无会话</p>
        )}
      </div>

      <ConfirmDialog
        open={delId !== null}
        title="删除会话"
        content="删除后该会话的全部消息不可恢复，确认删除？"
        danger
        confirmText="删除"
        onCancel={() => setDelId(null)}
        onConfirm={async () => {
          if (delId) await removeSession(delId);
          setDelId(null);
        }}
      />
    </div>
  );
}
