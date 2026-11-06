/**
 * 对话与会话接口 —— 对应 router.go 对话分组。
 */
import { http } from '@/lib/request';
import type { Paged, PageQuery } from '@/types/api';
import type { ChatModelItem, Message, Session } from '@/types/chat';

/** 对话模型选择列表（GET /models，对话页下拉用） */
export const listChatModels = () => http.get<ChatModelItem[]>('/models');

/** 会话列表 */
export const listSessions = (q?: PageQuery) =>
  http.get<Paged<Session>>('/sessions', q);

/** 会话消息历史（含 metadata.agent_trace 决策链） */
export const listMessages = (sessionId: string) =>
  http.get<Message[]>(`/sessions/${sessionId}/messages`);

/** 会话改名 */
export const renameSession = (id: string, title: string) =>
  http.put(`/sessions/${id}`, { title });

/** 删除会话 */
export const deleteSession = (id: string) =>
  http.delete(`/sessions/${id}`);
