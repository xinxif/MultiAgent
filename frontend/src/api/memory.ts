/**
 * 记忆接口 —— 对应 router.go 记忆分组。
 */
import { http } from '@/lib/request';
import type { Paged, PageQuery } from '@/types/api';
import type { ExtractMemoryReq, Memory } from '@/types/memory';

/** 记忆列表（分页） */
export const listMemories = (q?: PageQuery) =>
  http.get<Paged<Memory>>('/memories', q);

/** 删除记忆 */
export const deleteMemory = (id: number) => http.delete(`/memories/${id}`);

/** 归档记忆（status 1 生效 → 2 归档） */
export const archiveMemory = (id: number) =>
  http.post(`/memories/${id}/archive`);

/** 手动抽取："记住这次对话"（对指定会话跑长期记忆抽取） */
export const extractMemory = (req: ExtractMemoryReq) =>
  http.post('/memories/extract', req);
