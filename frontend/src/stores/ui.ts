/**
 * 全局 UI 状态：Toast 通知队列。
 * 不引第三方组件库 —— 轻量自实现，样式与设计系统一致。
 */
import { create } from 'zustand';

export type ToastTone = 'ok' | 'err' | 'info';

export interface Toast {
  id: number;
  tone: ToastTone;
  message: string;
}

interface UIState {
  toasts: Toast[];
  push: (tone: ToastTone, message: string) => void;
  remove: (id: number) => void;
}

let seq = 0;

export const useUI = create<UIState>((set) => ({
  toasts: [],
  push: (tone, message) => {
    const id = ++seq;
    set((s) => ({ toasts: [...s.toasts, { id, tone, message }] }));
    // 3.2s 自动消失
    setTimeout(() => {
      set((s) => ({ toasts: s.toasts.filter((t) => t.id !== id) }));
    }, 3200);
  },
  remove: (id) => set((s) => ({ toasts: s.toasts.filter((t) => t.id !== id) })),
}));

/** 便捷函数：组件里直接 toast.ok('...') */
export const toast = {
  ok: (msg: string) => useUI.getState().push('ok', msg),
  err: (msg: string) => useUI.getState().push('err', msg),
  info: (msg: string) => useUI.getState().push('info', msg),
};
