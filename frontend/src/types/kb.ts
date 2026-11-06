/**
 * 知识库域类型 —— 对应 model/rag.go。
 */

/** 文档 ETL 状态机 */
export type DocStatus =
  | 'pending'
  | 'parsing'
  | 'chunking'
  | 'embedding'
  | 'ready'
  | 'failed';

/** 知识库 */
export interface KnowledgeBase {
  id: number;
  name: string;
  description: string;
  user_id: number;
  embedding_config_id: number;
  chunk_size: number;
  chunk_overlap: number;
  parser_config: Record<string, unknown>;
  doc_count: number;
  milvus_collection: string;
  status: string;
  created_at: string;
  updated_at: string;
}

/** 创建知识库请求 */
export type KBReq = Partial<
  Pick<KnowledgeBase, 'name' | 'description' | 'chunk_size' | 'chunk_overlap' | 'embedding_config_id'>
>;

/** 文档 */
export interface Document {
  id: number;
  kb_id: number;
  filename: string;
  file_type: string;
  object_key: string;
  file_size: number;
  source_url: string;
  status: DocStatus;
  error: string;
  chunk_count: number;
  token_count: number;
  enable_graph: boolean;
  metadata: Record<string, unknown>;
  created_at: string;
  updated_at: string;
}

/** 分块 */
export interface DocumentChunk {
  id: number;
  kb_id: number;
  document_id: number;
  seq: number;
  content: string;
  token_count: number;
  /** {"page":3,"heading":"..."} */
  meta: Record<string, unknown>;
  vector_id: string;
  status: number;
  created_at: string;
}
