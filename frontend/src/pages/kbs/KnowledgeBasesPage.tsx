/**
 * 知识库列表页 —— 卡片网格展示全部知识库，支持新建与删除。
 * 数据流：listKBs 一次性拉取（page_size=100）；创建/删除成功后整表刷新。
 */
import { useCallback, useEffect, useRef, useState } from 'react';
import { useNavigate } from 'react-router-dom';
import {
  Button,
  Card,
  EmptyState,
  Input,
  PageHeader,
  Spinner,
  StatusBadge,
  Textarea,
} from '@/components/ui/primitives';
import { Modal } from '@/components/ui/Modal';
import { ConfirmDialog } from '@/components/ui/ConfirmDialog';
import { createKB, deleteKB, listKBs } from '@/api/kb';
import { relativeTime } from '@/lib/format';
import { gsap, staggerEnter } from '@/lib/gsap';
import { errMsg } from '@/lib/request';
import { toast } from '@/stores/ui';
import type { KnowledgeBase } from '@/types/kb';

export function KnowledgeBasesPage() {
  const navigate = useNavigate();

  const [kbs, setKbs] = useState<KnowledgeBase[]>([]);
  const [loading, setLoading] = useState(true);

  /* ---- 新建弹窗表单 ---- */
  const [createOpen, setCreateOpen] = useState(false);
  const [name, setName] = useState('');
  const [description, setDescription] = useState('');
  const [chunkSize, setChunkSize] = useState('512');
  const [chunkOverlap, setChunkOverlap] = useState('64');
  const [creating, setCreating] = useState(false);

  /* ---- 删除确认（持有待删除对象，弹窗内展示名称） ---- */
  const [delKB, setDelKB] = useState<KnowledgeBase | null>(null);
  const [deleting, setDeleting] = useState(false);

  const gridRef = useRef<HTMLDivElement>(null);

  /** 拉取知识库列表（单页 100 条，控制台场景足够） */
  const load = useCallback(async () => {
    setLoading(true);
    try {
      const data = await listKBs({ page: 1, page_size: 100 });
      setKbs(data.list);
    } catch (e) {
      toast.err(errMsg(e));
    } finally {
      setLoading(false);
    }
  }, []);

  // 首次挂载拉取列表
  useEffect(() => {
    void load();
  }, [load]);

  // 列表卡片交错入场（动效克制：仅位移 10px + 淡入）
  useEffect(() => {
    if (loading || kbs.length === 0) return;
    const grid = gridRef.current;
    if (!grid) return;
    const ctx = gsap.context(() => staggerEnter(Array.from(grid.children)));
    return () => ctx.revert();
  }, [loading, kbs]);

  /** 打开新建弹窗前重置表单为默认值 */
  const openCreate = () => {
    setName('');
    setDescription('');
    setChunkSize('512');
    setChunkOverlap('64');
    setCreateOpen(true);
  };

  /** 提交新建：name 必填，分块参数转数字（非法输入回退默认值） */
  const submitCreate = async () => {
    const n = name.trim();
    if (!n) {
      toast.err('请填写知识库名称');
      return;
    }
    setCreating(true);
    try {
      await createKB({
        name: n,
        description: description.trim(),
        chunk_size: Number(chunkSize) || 512,
        chunk_overlap: Number(chunkOverlap) || 64,
      });
      toast.ok('知识库创建成功');
      setCreateOpen(false);
      await load();
    } catch (e) {
      toast.err(errMsg(e));
    } finally {
      setCreating(false);
    }
  };

  /** 确认删除知识库 */
  const confirmDelete = async () => {
    if (!delKB) return;
    setDeleting(true);
    try {
      await deleteKB(delKB.id);
      toast.ok('知识库已删除');
      setDelKB(null);
      await load();
    } catch (e) {
      toast.err(errMsg(e));
    } finally {
      setDeleting(false);
    }
  };

  return (
    <div className="page-container">
      <PageHeader
        title="知识库"
        desc="上传文档构建 RAG 知识库，供智能体检索引用"
        action={<Button onClick={openCreate}>＋ 新建知识库</Button>}
      />

      {loading ? (
        // 首屏加载
        <div className="flex justify-center py-20">
          <Spinner className="h-6 w-6 text-lime" />
        </div>
      ) : kbs.length === 0 ? (
        // 空态：引导创建
        <EmptyState
          icon="📚"
          title="还没有知识库"
          hint="创建一个知识库并上传文档，即可在对话中引用其中的知识"
          action={<Button onClick={openCreate}>新建知识库</Button>}
        />
      ) : (
        <div ref={gridRef} className="grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-3">
          {kbs.map((kb) => (
            <Card
              key={kb.id}
              className="group relative"
              onClick={() => navigate(`/kbs/${kb.id}`)}
            >
              {/* 标题行：名称 + 状态徽标 + 悬浮删除按钮 */}
              <div className="mb-2 flex items-start justify-between gap-2">
                <h3 className="min-w-0 truncate text-sm font-semibold text-bone" title={kb.name}>
                  {kb.name}
                </h3>
                <div className="flex shrink-0 items-center gap-1.5">
                  <StatusBadge status={kb.status} />
                  {/* 悬浮才出现，避免视觉噪音；阻止冒泡避免触发卡片跳转 */}
                  <button
                    className="hidden text-xs text-ash transition-colors hover:text-coral group-hover:block"
                    title="删除知识库"
                    onClick={(e) => {
                      e.stopPropagation();
                      setDelKB(kb);
                    }}
                  >
                    ✕
                  </button>
                </div>
              </div>

              {/* 描述：最多两行 */}
              <p className="mb-3 line-clamp-2 min-h-[2rem] text-xs leading-4 text-fog">
                {kb.description || '暂无描述'}
              </p>

              {/* 元信息：文档数 / 分块参数 / 更新时间 */}
              <div className="flex flex-wrap items-center gap-x-3 gap-y-1 text-[11px] text-ash">
                <span className="text-lime">{kb.doc_count} 篇文档</span>
                <span>chunk_size {kb.chunk_size}</span>
                <span>overlap {kb.chunk_overlap}</span>
                <span className="ml-auto">{relativeTime(kb.updated_at)}</span>
              </div>
            </Card>
          ))}
        </div>
      )}

      {/* 新建知识库弹窗 */}
      <Modal
        open={createOpen}
        title="新建知识库"
        onClose={() => setCreateOpen(false)}
        footer={
          <>
            <Button variant="ghost" onClick={() => setCreateOpen(false)}>
              取消
            </Button>
            <Button loading={creating} onClick={() => void submitCreate()}>
              创建
            </Button>
          </>
        }
      >
        <form
          className="space-y-4"
          onSubmit={(e) => {
            e.preventDefault();
            void submitCreate();
          }}
        >
          <div>
            <label className="field-label">名称 *</label>
            <Input
              value={name}
              onChange={(e) => setName(e.target.value)}
              placeholder="如：产品文档库"
              autoFocus
            />
          </div>
          <div>
            <label className="field-label">描述</label>
            <Textarea
              rows={3}
              value={description}
              onChange={(e) => setDescription(e.target.value)}
              placeholder="知识库用途说明（可选）"
            />
          </div>
          <div className="grid grid-cols-2 gap-3">
            <div>
              <label className="field-label">分块大小（chunk_size）</label>
              <Input
                type="number"
                min={64}
                value={chunkSize}
                onChange={(e) => setChunkSize(e.target.value)}
              />
            </div>
            <div>
              <label className="field-label">分块重叠（chunk_overlap）</label>
              <Input
                type="number"
                min={0}
                value={chunkOverlap}
                onChange={(e) => setChunkOverlap(e.target.value)}
              />
            </div>
          </div>
        </form>
      </Modal>

      {/* 删除知识库二次确认 */}
      <ConfirmDialog
        open={delKB !== null}
        title="删除知识库"
        content={delKB ? `确认删除「${delKB.name}」？其中全部文档与向量数据将被一并移除，不可恢复。` : ''}
        danger
        confirmText="删除"
        loading={deleting}
        onCancel={() => setDelKB(null)}
        onConfirm={() => void confirmDelete()}
      />
    </div>
  );
}
