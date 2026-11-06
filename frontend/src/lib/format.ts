/**
 * 展示格式化工具。
 */

/** 相对时间：刚刚 / n 分钟前 / n 小时前 / 昨天 / 日期 */
export function relativeTime(iso?: string | null): string {
  if (!iso) return '';
  const t = new Date(iso).getTime();
  if (Number.isNaN(t)) return '';
  const diff = Date.now() - t;
  const min = 60_000;
  const hour = 60 * min;
  const day = 24 * hour;
  if (diff < min) return '刚刚';
  if (diff < hour) return `${Math.floor(diff / min)} 分钟前`;
  if (diff < day) return `${Math.floor(diff / hour)} 小时前`;
  if (diff < 2 * day) return '昨天';
  return new Date(iso).toLocaleDateString('zh-CN');
}

/** 完整时间：2026-09-05 14:30 */
export function fullTime(iso?: string | null): string {
  if (!iso) return '';
  const d = new Date(iso);
  if (Number.isNaN(d.getTime())) return '';
  const pad = (n: number) => String(n).padStart(2, '0');
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())} ${pad(d.getHours())}:${pad(d.getMinutes())}`;
}

/** 字节数人性化：1.2 KB / 3.4 MB */
export function fileSize(bytes?: number | null): string {
  if (!bytes || bytes <= 0) return '0 B';
  const units = ['B', 'KB', 'MB', 'GB'];
  let i = 0;
  let v = bytes;
  while (v >= 1024 && i < units.length - 1) {
    v /= 1024;
    i++;
  }
  return `${v.toFixed(v >= 100 || i === 0 ? 0 : 1)} ${units[i]}`;
}

/** Token 数缩写：12345 → 12.3k */
export function tokenCount(n?: number | null): string {
  if (!n || n <= 0) return '0';
  if (n < 1000) return String(n);
  return `${(n / 1000).toFixed(1)}k`;
}

/** 截断文本 */
export function truncate(s: string, max = 80): string {
  return s.length > max ? `${s.slice(0, max)}…` : s;
}

/** 状态 → 展示映射（文档 ETL / 工作流 Run 通用） */
export const STATUS_LABEL: Record<string, { text: string; tone: 'ok' | 'warn' | 'err' | 'info' }> = {
  ready: { text: '就绪', tone: 'ok' },
  succeeded: { text: '成功', tone: 'ok' },
  active: { text: '活跃', tone: 'ok' },
  connected: { text: '已连接', tone: 'ok' },
  pending: { text: '排队中', tone: 'info' },
  parsing: { text: '解析中', tone: 'info' },
  chunking: { text: '分块中', tone: 'info' },
  embedding: { text: '向量化中', tone: 'info' },
  running: { text: '执行中', tone: 'info' },
  waiting_approval: { text: '待审批', tone: 'warn' },
  failed: { text: '失败', tone: 'err' },
  error: { text: '异常', tone: 'err' },
  canceled: { text: '已取消', tone: 'err' },
  disconnected: { text: '未连接', tone: 'warn' },
  unknown: { text: '未知', tone: 'warn' },
};
