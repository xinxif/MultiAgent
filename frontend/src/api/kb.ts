/**
 * 知识库接口 —— 对应 router.go 知识库分组。
 * 上传为 multipart（c.FormFile("file")）。
 */
import { http } from '@/lib/request';
import type { Paged, PageQuery } from '@/types/api';
import type { RAGRef } from '@/types/chat';
import type { Document, DocumentChunk, KBReq, KnowledgeBase } from '@/types/kb';

/** 知识库列表（分页） */
export const listKBs = (q?: PageQuery) => http.get<Paged<KnowledgeBase>>('/kbs', q);

/** 知识库详情 */
export const getKB = (id: number) => http.get<KnowledgeBase>(`/kbs/${id}`);

/** 创建知识库 */
export const createKB = (req: KBReq) => http.post<KnowledgeBase>('/kbs', req);

/** 删除知识库 */
export const deleteKB = (id: number) => http.delete(`/kbs/${id}`);

/** 上传文档（multipart，字段名必须是 file） */
export const uploadDoc = (kbId: number, file: File) => {
  const fd = new FormData();
  fd.append('file', file);
  return http.upload<Document>(`/kbs/${kbId}/documents`, fd);
};

/** 文档列表（分页） */
export const listDocs = (kbId: number, q?: PageQuery) =>
  http.get<Paged<Document>>(`/kbs/${kbId}/documents`, q);

/** 删除文档 */
export const deleteDoc = (kbId: number, docId: number) =>
  http.delete(`/kbs/${kbId}/documents/${docId}`);

/** 检索测试台：POST /kbs/:id/retrieve {query} → Ref[] */
export const testRetrieve = (kbId: number, query: string) =>
  http.post<RAGRef[]>(`/kbs/${kbId}/retrieve`, { query });

/** 分块列表 */
export const listChunks = (docId: number, q?: PageQuery) =>
  http.get<Paged<DocumentChunk>>(`/documents/${docId}/chunks`, q);
