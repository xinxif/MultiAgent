/**
 * Toast 通知宿主 —— 渲染 ui store 的消息队列，顶部居中滑入。
 */
import { useEffect, useRef } from 'react';
import { createPortal } from 'react-dom';
import { gsap } from '@/lib/gsap';
import { useUI, type Toast } from '@/stores/ui';
import { clsx } from 'clsx';

const TONE_CLASS = {
  ok: 'border-pulse/50 text-pulse',
  err: 'border-coral/50 text-coral',
  info: 'border-teal/50 text-teal',
} as const;

function ToastItem({ t }: { t: Toast }) {
  const ref = useRef<HTMLDivElement>(null);
  useEffect(() => {
    // 入场：从顶部滑入
    gsap.fromTo(
      ref.current,
      { opacity: 0, y: -14 },
      { opacity: 1, y: 0, duration: 0.3, ease: 'power3.out' },
    );
  }, []);
  return (
    <div
      ref={ref}
      className={clsx(
        'pointer-events-auto rounded-lg border bg-obsidian/95 px-4 py-2.5 text-sm text-mist shadow-xl backdrop-blur',
        TONE_CLASS[t.tone],
      )}
    >
      {t.message}
    </div>
  );
}

export function ToastHost() {
  const toasts = useUI((s) => s.toasts);
  if (toasts.length === 0) return null;
  return createPortal(
    <div className="pointer-events-none fixed left-1/2 top-5 z-[100] flex -translate-x-1/2 flex-col items-center gap-2">
      {toasts.map((t) => (
        <ToastItem key={t.id} t={t} />
      ))}
    </div>,
    document.body,
  );
}
