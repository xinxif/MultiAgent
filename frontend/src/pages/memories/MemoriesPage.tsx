/**
 * 长期记忆管理页 —— 记忆分页列表 + 归档 / 删除 + 手动触发记忆抽取。
 *
 * 职责：
 * 1. 分页展示长期记忆：内容、范围、重要度、命中统计、状态与时间线；
 * 2. 生效中的记忆可归档，任意记忆可删除（ConfirmDialog 二次确认）；
 * 3. 支持指定会话手动触发一次长期记忆抽取。
 */
import { useCallback, useEffect, useRef, useState } from 'react';
import { clsx } from 'clsx';
import {
  Badge,
  Button,
  EmptyState,
  Input,
  PageHeader,
  Spinner,
} from '@/components/ui/primitives';
import { Modal } from '@/components/ui/Modal';
import { ConfirmDialog } from '@/components/ui/ConfirmDialog';
import { archiveMemory, deleteMemory, extractMemory, listMemories } from '@/api/memory';
import type { Memory } from '@/types/memory';
import { fullTime, relativeTime } from '@/lib/format';
import { errMsg } from '@/lib/request';
import { toast } from '@/stores/ui';
import { staggerEnter } from '@/lib/gsap';

const PAGE_SIZE = 20;

/** 记忆范围 → Badge（user → teal / agent → violet） */
const SCOPE_BADGE: Record<Memory['scope'], { text: string; tone: 'teal' | 'violet' }> = {
  user: { text: '用户', tone: 'teal' },
  agent: { text: 'Agent', tone: 'violet' },
};

/** 重要度 1-10 → 5 段小方块（每格代表 2 级）+ 数字 */
function ImportanceBar({ value }: { value: number }) {
  const filled = Math.min(5, Math.max(0, Math.ceil(value / 2)));
  return (
    <span className="inline-flex items-center gap-1.5">
      <span className="flex gap-0.5">
        {[1, 2, 3, 4, 5].map((i) => (
          <span
            key={i}
            className={clsx('h-1.5 w-2.5 rounded-sm', i <= filled ? 'bg-pulse' : 'bg-smoke')}
          />
        ))}
      </span>
      <span className="text-xs text-ash">{value}</span>
    </span>
  );
}

export function MemoriesPage() {
  const [list, setList] = useState<Memory[]>([]);
  const [total, setTotal] = useState(0);
  const [page, setPage] = useState(1);
  const [loading, setLoading] = useState(true);
  const [extractOpen, setExtractOpen] = useState(false);
  const [sessionId, setSessionId] = useState('');
  const [extracting, setExtracting] = useState(false);
  const [archivingId, setArchivingId] = useState<number | null>(null);
  const [delTarget, setDelTarget] = useState<Memory | null>(null);
  const tbodyRef = useRef<HTMLTableSectionElement>(null);

  const totalPages = Math.max(1, Math.ceil(total / PAGE_SIZE));

  /** 拉取指定页记忆 */
  const load = useCallback(async (p: number) => {
    setLoading(true);
    try {
      const data = await listMemories({ page: p, page_size: PAGE_SIZE });
      setList(data.list);
      setTotal(data.total);
    } catch (e) {
      toast.err(errMsg(e));
    } finally {
      setLoading(false);
    }
  }, []);

  // 翻页 / 增删后的刷新统一走这里
  useEffect(() => {
    void load(page);
  }, [load, page]);

  // 列表数据变化 → 行级交错入场（GSAP）
  useEffect(() => {
    const els = tbodyRef.current ? Array.from(tbodyRef.current.children) : [];
    if (els.length === 0) return;
    const tween = staggerEnter(els);
    return () => {
      tween.kill();
    };
  }, [list]);

  /** 归档（仅生效中的记忆可操作：status 1 → 2） */
  const doArchive = async (m: Memory) => {
    setArchivingId(m.id);
    try {
      await archiveMemory(m.id);
      toast.ok('记忆已归档');
      await load(page);
    } catch (e) {
      toast.err(errMsg(e));
    } finally {
      setArchivingId(null);
    }
  };

  /** 删除：当前页删空且非首页时回退一页（触发 useEffect 重新拉取） */
  const confirmDelete = async () => {
    if (!delTarget) return;
    try {
      await deleteMemory(delTarget.id);
      toast.ok('记忆已删除');
      setDelTarget(null);
      if (list.length === 1 && page > 1) {
        setPage(page - 1);
      } else {
        await load(page);
      }
    } catch (e) {
      toast.err(errMsg(e));
    }
  };

  /** 手动触发一次长期记忆抽取 */
  const doExtract = async () => {
    const sid = sessionId.trim();
    if (!sid) {
      toast.err('请填写会话 ID');
      return;
    }
    setExtracting(true);
    try {
      await extractMemory({ session_id: sid });
      toast.ok('已触发记忆抽取');
      setExtractOpen(false);
      setSessionId('');
      await load(1); // 回到第一页查看新抽取的记忆
    } catch (e) {
      toast.err(errMsg(e));
    } finally {
      setExtracting(false);
    }
  };

  return (
    <div className="page-container">
      <PageHeader
        title="长期记忆"
        desc="跨会话沉淀的用户与 Agent 记忆，可归档或删除"
        action={<Button onClick={() => setExtractOpen(true)}>记住一次对话</Button>}
      />

      {loading ? (
        <div className="flex justify-center py-20">
          <Spinner className="h-6 w-6 text-lime" />
        </div>
      ) : list.length === 0 ? (
        <EmptyState
          title="暂无长期记忆"
          hint="与 Agent 对话积累的记忆会出现在这里，也可点击右上角手动触发抽取"
        />
      ) : (
        <>
          {/* 记忆列表：窄屏横向滚动 */}
          <div className="panel overflow-hidden">
            <div className="overflow-x-auto">
              <table className="w-full text-left text-sm">
                <thead>
                  <tr className="border-b border-smoke/60 text-xs tracking-wider text-ash">
                    <th className="px-4 py-3 font-medium">记忆内容</th>
                    <th className="px-4 py-3 font-medium">范围</th>
                    <th className="px-4 py-3 font-medium">重要度</th>
                    <th className="px-4 py-3 font-medium">命中</th>
                    <th className="px-4 py-3 font-medium">最近命中</th>
                    <th className="px-4 py-3 font-medium">状态</th>
                    <th className="px-4 py-3 font-medium">创建时间</th>
                    <th className="px-4 py-3 text-right font-medium">操作</th>
                  </tr>
                </thead>
                <tbody ref={tbodyRef}>
                  {list.map((m) => (
                    <tr
                      key={m.id}
                      className="border-b border-smoke/40 align-top transition-colors last:border-0 hover:bg-carbon/50"
                    >
                      {/* content 全文展示 */}
                      <td className="max-w-md whitespace-pre-wrap break-words px-4 py-3 text-mist">
                        {m.content}
                      </td>
                      <td className="px-4 py-3">
                        <Badge tone={SCOPE_BADGE[m.scope].tone}>
                          {SCOPE_BADGE[m.scope].text}
                        </Badge>
                      </td>
                      <td className="px-4 py-3">
                        <ImportanceBar value={m.importance} />
                      </td>
                      <td className="px-4 py-3 text-xs text-fog">{m.hit_count}</td>
                      <td className="px-4 py-3 text-xs text-fog">
                        {relativeTime(m.last_hit_at) || '—'}
                      </td>
                      {/* 状态：1 生效 / 2 已归档 */}
                      <td className="px-4 py-3">
                        {m.status === 1 ? (
                          <Badge tone="pulse">生效</Badge>
                        ) : (
                          <Badge tone="gray">已归档</Badge>
                        )}
                      </td>
                      <td className="px-4 py-3 text-xs text-fog">{fullTime(m.created_at)}</td>
                      <td className="px-4 py-3">
                        <div className="flex justify-end gap-2">
                          {m.status === 1 && (
                            <Button
                              variant="ghost"
                              className="px-2.5 py-1 text-xs"
                              loading={archivingId === m.id}
                              onClick={() => void doArchive(m)}
                            >
                              归档
                            </Button>
                          )}
                          <Button
                            variant="danger"
                            className="px-2.5 py-1 text-xs"
                            onClick={() => setDelTarget(m)}
                          >
                            删除
                          </Button>
                        </div>
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          </div>

          {/* 分页条 */}
          <div className="mt-4 flex items-center justify-between text-xs text-ash">
            <span>共 {total} 条</span>
            <div className="flex items-center gap-3">
              <Button
                variant="ghost"
                className="px-3 py-1 text-xs"
                disabled={page <= 1 || loading}
                onClick={() => setPage((p) => p - 1)}
              >
                上一页
              </Button>
              <span>
                第 {page} / {totalPages} 页
              </span>
              <Button
                variant="ghost"
                className="px-3 py-1 text-xs"
                disabled={page >= totalPages || loading}
                onClick={() => setPage((p) => p + 1)}
              >
                下一页
              </Button>
            </div>
          </div>
        </>
      )}

      {/* 手动抽取 Modal */}
      <Modal
        open={extractOpen}
        title="记住一次对话"
        onClose={() => setExtractOpen(false)}
        width="max-w-md"
        footer={
          <>
            <Button variant="ghost" onClick={() => setExtractOpen(false)}>
              取消
            </Button>
            <Button loading={extracting} onClick={() => void doExtract()}>
              开始抽取
            </Button>
          </>
        }
      >
        <form
          onSubmit={(e) => {
            e.preventDefault();
            void doExtract();
          }}
        >
          <label className="field-label">会话 ID *</label>
          <Input
            value={sessionId}
            onChange={(e) => setSessionId(e.target.value)}
            placeholder="输入要抽取记忆的会话 ID"
          />
          <p className="mt-1 text-xs text-ash">
            将对指定会话执行长期记忆抽取，结果稍后出现在列表中
          </p>
        </form>
      </Modal>

      {/* 删除二次确认 */}
      <ConfirmDialog
        open={delTarget !== null}
        title="删除记忆"
        content="删除后该条长期记忆不可恢复，确认删除？"
        danger
        confirmText="删除"
        onCancel={() => setDelTarget(null)}
        onConfirm={() => void confirmDelete()}
      />
    </div>
  );
}
