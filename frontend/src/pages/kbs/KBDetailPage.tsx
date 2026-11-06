/**
 * 知识库详情页 —— 文档管理（上传/删除/查看分块）+ 检索测试台。
 * 职责：
 * 1. 顶部展示知识库元信息（getKB），含返回列表入口；
 * 2. 文档表格（listDocs）：上传（uploadDoc）、删除（deleteDoc）、查看分块（listChunks）；
 * 3. 底部检索测试台（testRetrieve）：输入 query 即时验证检索效果。
 */
import { useCallback, useEffect, useRef, useState, type ChangeEvent } from 'react';
import { Link, useParams } from 'react-router-dom';
import {
  Button,
  EmptyState,
  Input,
  PageHeader,
  Spinner,
  StatusBadge,
} from '@/components/ui/primitives';
import { Modal } from '@/components/ui/Modal';
import { ConfirmDialog } from '@/components/ui/ConfirmDialog';
import { deleteDoc, getKB, listChunks, listDocs, testRetrieve, uploadDoc } from '@/api/kb';
import { fileSize, relativeTime, truncate } from '@/lib/format';
import { gsap, pageEnter } from '@/lib/gsap';
import { errMsg } from '@/lib/request';
import { toast } from '@/stores/ui';
import type { Document, DocumentChunk, KnowledgeBase } from '@/types/kb';
import type { RAGRef } from '@/types/chat';

export function KBDetailPage() {
  const { id } = useParams();
  // 路由参数是字符串，转数字；NaN 视为非法 ID
  const kbId = Number(id);
  const validId = !Number.isNaN(kbId);

  /* ---- 知识库详情 ---- */
  const [kb, setKb] = useState<KnowledgeBase | null>(null);
  const [kbLoading, setKbLoading] = useState(true);

  /* ---- 文档列表 ---- */
  const [docs, setDocs] = useState<Document[]>([]);
  const [docsLoading, setDocsLoading] = useState(true);
  const [uploading, setUploading] = useState(false);

  /* ---- 删除文档确认 ---- */
  const [delDoc, setDelDoc] = useState<Document | null>(null);
  const [deleting, setDeleting] = useState(false);

  /* ---- 分块查看弹窗（按文档懒加载） ---- */
  const [chunkDoc, setChunkDoc] = useState<Document | null>(null);
  const [chunks, setChunks] = useState<DocumentChunk[]>([]);
  const [chunksLoading, setChunksLoading] = useState(false);

  /* ---- 检索测试台 ---- */
  const [query, setQuery] = useState('');
  const [retrieving, setRetrieving] = useState(false);
  // null 表示尚未检索过，与"检索到 0 条"区分开
  const [refs, setRefs] = useState<RAGRef[] | null>(null);

  const pageRef = useRef<HTMLDivElement>(null);

  /** 刷新文档列表（上传/删除后复用；ETL 异步，用户可点"刷新"看状态变化） */
  const refreshDocs = useCallback(async () => {
    setDocsLoading(true);
    try {
      const data = await listDocs(kbId, { page: 1, page_size: 100 });
      setDocs(data.list);
    } catch (e) {
      toast.err(errMsg(e));
    } finally {
      setDocsLoading(false);
    }
  }, [kbId]);

  // 首次挂载：并行拉取知识库详情与文档列表
  useEffect(() => {
    if (!validId) return;
    void (async () => {
      setKbLoading(true);
      try {
        setKb(await getKB(kbId));
      } catch (e) {
        toast.err(errMsg(e));
      } finally {
        setKbLoading(false);
      }
    })();
    void refreshDocs();
  }, [kbId, validId, refreshDocs]);

  // 页面容器入场动效（仅挂载时执行一次）
  useEffect(() => {
    const el = pageRef.current;
    if (!el) return;
    const ctx = gsap.context(() => pageEnter(el));
    return () => ctx.revert();
  }, []);

  /** 上传文档：multipart 直传，成功后刷新列表（ETL 为异步处理） */
  const onUpload = async (e: ChangeEvent<HTMLInputElement>) => {
    const file = e.target.files?.[0];
    e.target.value = ''; // 清空 value，允许重复选择同一文件
    if (!file) return;
    setUploading(true);
    try {
      await uploadDoc(kbId, file);
      toast.ok('已上传，ETL 处理中');
      await refreshDocs();
    } catch (err) {
      toast.err(errMsg(err));
    } finally {
      setUploading(false);
    }
  };

  /** 确认删除文档（级联清理分块与向量） */
  const confirmDeleteDoc = async () => {
    if (!delDoc) return;
    setDeleting(true);
    try {
      await deleteDoc(kbId, delDoc.id);
      toast.ok('文档已删除');
      setDelDoc(null);
      await refreshDocs();
    } catch (e) {
      toast.err(errMsg(e));
    } finally {
      setDeleting(false);
    }
  };

  /** 打开分块弹窗并按文档拉取前 50 条分块 */
  const openChunks = (doc: Document) => {
    setChunkDoc(doc);
    setChunks([]); // 先清空上一文档的分块，避免串数据
    setChunksLoading(true);
    listChunks(doc.id, { page: 1, page_size: 50 })
      .then((data) => setChunks(data.list))
      .catch((e: unknown) => toast.err(errMsg(e)))
      .finally(() => setChunksLoading(false));
  };

  /** 从分块 meta 中安全提取页码（meta 为自由 JSON） */
  const pageOf = (chunk: DocumentChunk): number | null => {
    const p = chunk.meta?.page;
    return typeof p === 'number' ? p : null;
  };

  /** 执行检索测试 */
  const retrieve = async () => {
    const q = query.trim();
    if (!q) {
      toast.err('请输入检索内容');
      return;
    }
    setRetrieving(true);
    try {
      setRefs(await testRetrieve(kbId, q));
    } catch (e) {
      toast.err(errMsg(e));
    } finally {
      setRetrieving(false);
    }
  };

  // 非法路由参数：不发起任何请求，直接给出提示
  if (!validId) {
    return (
      <div className="page-container">
        <EmptyState
          icon="⚠️"
          title="无效的知识库 ID"
          hint="请从知识库列表重新进入"
          action={
            <Link to="/kbs" className="btn-ghost">
              返回列表
            </Link>
          }
        />
      </div>
    );
  }

  // 详情首屏加载
  if (kbLoading && !kb) {
    return (
      <div className="flex h-full items-center justify-center">
        <Spinner className="h-6 w-6 text-lime" />
      </div>
    );
  }

  return (
    <div ref={pageRef} className="page-container">
      {/* 顶部：名称/描述 + 返回列表 */}
      <PageHeader
        title={kb?.name ?? '知识库详情'}
        desc={kb?.description || undefined}
        action={
          <Link to="/kbs" className="btn-ghost">
            ← 返回列表
          </Link>
        }
      />

      {/* 参数元信息行 */}
      {kb && (
        <div className="mb-6 flex flex-wrap items-center gap-x-4 gap-y-1 text-xs text-ash">
          <span>
            chunk_size <span className="font-mono text-mist">{kb.chunk_size}</span>
          </span>
          <span>
            chunk_overlap <span className="font-mono text-mist">{kb.chunk_overlap}</span>
          </span>
          <span className="text-lime">{kb.doc_count} 篇文档</span>
          <span>更新于 {relativeTime(kb.updated_at)}</span>
        </div>
      )}

      {/* ==================== 文档区 ==================== */}
      <section className="panel p-5">
        <div className="mb-1 flex items-center justify-between gap-3">
          <h2 className="section-title">文档列表</h2>
          <div className="flex items-center gap-2">
            {/* ETL 异步处理，手动刷新查看状态变化 */}
            <Button variant="ghost" loading={docsLoading} onClick={() => void refreshDocs()}>
              刷新
            </Button>
            {/* 文件选择：label 触发隐藏 input，保持按钮样式统一 */}
            <label className="btn-primary cursor-pointer">
              {uploading && <Spinner className="h-3.5 w-3.5" />}
              {uploading ? '上传中…' : '上传文档'}
              <input
                type="file"
                accept=".txt,.md,.pdf,.docx"
                className="hidden"
                disabled={uploading}
                onChange={(e) => void onUpload(e)}
              />
            </label>
          </div>
        </div>
        <p className="mb-4 text-xs text-ash">
          支持 .txt / .md / .pdf / .docx；上传后 ETL 异步处理，可点"刷新"查看状态变化
        </p>

        {docsLoading && docs.length === 0 ? (
          <div className="flex justify-center py-10">
            <Spinner />
          </div>
        ) : docs.length === 0 ? (
          <EmptyState
            icon="📄"
            title="暂无文档"
            hint="上传文档后系统将自动解析、分块并向量化"
          />
        ) : (
          <div className="overflow-x-auto">
            <table className="w-full min-w-[720px] text-left text-sm">
              <thead>
                <tr className="border-b border-smoke/60 text-xs text-ash">
                  <th className="px-2 py-2 font-medium">文件</th>
                  <th className="px-2 py-2 font-medium">大小</th>
                  <th className="px-2 py-2 font-medium">状态</th>
                  <th className="px-2 py-2 font-medium">分块</th>
                  <th className="px-2 py-2 font-medium">Tokens</th>
                  <th className="px-2 py-2 font-medium">更新时间</th>
                  <th className="px-2 py-2 text-right font-medium">操作</th>
                </tr>
              </thead>
              <tbody>
                {docs.map((doc) => (
                  <tr
                    key={doc.id}
                    className="border-b border-smoke/40 transition-colors last:border-0 hover:bg-carbon/40"
                  >
                    <td className="max-w-xs px-2 py-2.5">
                      <p className="truncate font-medium text-bone" title={doc.filename}>
                        {doc.filename}
                      </p>
                      <p className="text-[11px] text-ash">{doc.file_type}</p>
                      {/* ETL 失败时用珊瑚红展示错误原因 */}
                      {doc.status === 'failed' && doc.error && (
                        <p className="mt-0.5 truncate text-[11px] text-coral" title={doc.error}>
                          {doc.error}
                        </p>
                      )}
                    </td>
                    <td className="px-2 py-2.5 text-xs text-fog">{fileSize(doc.file_size)}</td>
                    <td className="px-2 py-2.5">
                      <StatusBadge status={doc.status} />
                    </td>
                    <td className="px-2 py-2.5 text-xs text-fog">{doc.chunk_count}</td>
                    <td className="px-2 py-2.5 text-xs text-fog">{doc.token_count}</td>
                    <td className="px-2 py-2.5 text-xs text-fog">{relativeTime(doc.updated_at)}</td>
                    <td className="px-2 py-2.5 text-right">
                      <div className="flex justify-end gap-3 text-xs">
                        <button
                          className="text-teal transition-opacity hover:opacity-80"
                          onClick={() => openChunks(doc)}
                        >
                          查看分块
                        </button>
                        <button
                          className="text-coral transition-opacity hover:opacity-80"
                          onClick={() => setDelDoc(doc)}
                        >
                          删除
                        </button>
                      </div>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}
      </section>

      {/* ==================== 检索测试台 ==================== */}
      <section className="panel mt-6 p-5">
        <h2 className="section-title mb-4">检索测试台</h2>
        <div className="flex gap-2">
          <Input
            value={query}
            onChange={(e) => setQuery(e.target.value)}
            placeholder="输入查询内容，测试知识库检索效果"
            onKeyDown={(e) => e.key === 'Enter' && void retrieve()}
          />
          <Button loading={retrieving} onClick={() => void retrieve()}>
            检索
          </Button>
        </div>

        {/* 检索结果：null=未检索过；空数组=无命中 */}
        <div className="mt-4 space-y-2">
          {refs === null ? null : refs.length === 0 ? (
            <EmptyState
              title="未检索到相关内容"
              hint="请调整查询词，或确认文档已完成向量化"
            />
          ) : (
            refs.map((ref, i) => (
              <div
                key={`${ref.chunk_id}-${i}`}
                className="rounded-lg border border-smoke/50 bg-carbon/40 p-3"
              >
                <div className="mb-1.5 flex flex-wrap items-center gap-x-3 gap-y-1 text-xs">
                  <span className="font-mono text-lime">{i + 1}</span>
                  <span className="font-medium text-bone" title={ref.filename}>
                    {ref.filename}
                  </span>
                  {ref.page > 0 && <span className="text-ash">第 {ref.page} 页</span>}
                  {/* 相似度得分：Teal 功能色 */}
                  <span className="ml-auto font-mono text-teal">{ref.score.toFixed(3)}</span>
                </div>
                {/* 摘要：只展示前 200 字 */}
                <p className="text-xs leading-relaxed text-fog">{truncate(ref.content, 200)}</p>
              </div>
            ))
          )}
        </div>
      </section>

      {/* ==================== 分块查看弹窗 ==================== */}
      <Modal
        open={chunkDoc !== null}
        title={chunkDoc ? `分块列表 · ${chunkDoc.filename}` : '分块列表'}
        onClose={() => setChunkDoc(null)}
        width="max-w-2xl"
      >
        {chunksLoading ? (
          // 分块加载中
          <div className="flex justify-center py-10">
            <Spinner />
          </div>
        ) : chunks.length === 0 ? (
          <EmptyState title="暂无分块" hint="文档可能尚未完成 ETL 处理" />
        ) : (
          <div className="space-y-2">
            {chunks.map((c) => {
              const page = pageOf(c);
              return (
                <div key={c.id} className="rounded-lg border border-smoke/50 bg-carbon/40 p-3">
                  <div className="mb-1.5 flex items-center gap-3 text-[11px] text-ash">
                    <span className="font-mono text-lime">#{c.seq}</span>
                    {page !== null && <span>第 {page} 页</span>}
                    <span>{c.token_count} tokens</span>
                  </div>
                  {/* 分块原文：保留换行 */}
                  <p className="whitespace-pre-wrap break-words text-xs leading-relaxed text-mist">
                    {c.content}
                  </p>
                </div>
              );
            })}
          </div>
        )}
      </Modal>

      {/* 删除文档二次确认 */}
      <ConfirmDialog
        open={delDoc !== null}
        title="删除文档"
        content={
          delDoc
            ? `确认删除「${delDoc.filename}」？该文档的全部分块与向量将被一并移除，不可恢复。`
            : ''
        }
        danger
        confirmText="删除"
        loading={deleting}
        onCancel={() => setDelDoc(null)}
        onConfirm={() => void confirmDeleteDoc()}
      />
    </div>
  );
}
