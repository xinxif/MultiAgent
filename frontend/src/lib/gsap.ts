/**
 * GSAP 动效预设 —— 统一全站动画语言，避免各页面随手写参。
 *
 * 原则：
 * 1. 快进缓出（out-expo / power3.out），动效"跟手"不拖沓；
 * 2. 入场动画统一 0.4~0.5s，位移小（12~16px），克制即高级；
 * 3. 列表入场用 stagger，形成节奏感；
 * 4. 组件卸载时清理（配合 gsap.context / useLayoutEffect cleanup）。
 */
import gsap from 'gsap';

/** 页面容器入场：轻微上浮 + 淡入 */
export function pageEnter(el: Element) {
  return gsap.fromTo(
    el,
    { opacity: 0, y: 14 },
    { opacity: 1, y: 0, duration: 0.45, ease: 'power3.out', overwrite: 'auto' },
  );
}

/** 列表子项交错入场：给每个子元素依次上浮淡入 */
export function staggerEnter(els: Element[] | NodeListOf<Element>, step = 0.05) {
  return gsap.fromTo(
    els,
    { opacity: 0, y: 10 },
    {
      opacity: 1,
      y: 0,
      duration: 0.4,
      ease: 'power3.out',
      stagger: step,
      overwrite: 'auto',
    },
  );
}

/** 消息气泡入场：新消息从下方弹入 */
export function bubbleIn(el: Element) {
  return gsap.fromTo(
    el,
    { opacity: 0, y: 12, scale: 0.98 },
    { opacity: 1, y: 0, scale: 1, duration: 0.35, ease: 'power3.out', overwrite: 'auto' },
  );
}

/** 决策链节点展开/收起 */
export function expandToggle(el: Element, open: boolean) {
  if (open) {
    return gsap.fromTo(
      el,
      { height: 0, opacity: 0 },
      { height: 'auto', opacity: 1, duration: 0.3, ease: 'power3.out', overwrite: 'auto' },
    );
  }
  return gsap.to(el, { height: 0, opacity: 0, duration: 0.25, ease: 'power2.in', overwrite: 'auto' });
}

/** 数字滚动：仪表盘统计数字从 0 滚到目标值 */
export function countUp(el: HTMLElement, target: number, duration = 0.8) {
  const obj = { v: 0 };
  return gsap.to(obj, {
    v: target,
    duration,
    ease: 'power2.out',
    onUpdate: () => {
      el.textContent = String(Math.round(obj.v));
    },
  });
}

/** 抖动：表单校验失败提示 */
export function shake(el: Element) {
  return gsap.fromTo(
    el,
    { x: -6 },
    { x: 0, duration: 0.4, ease: 'elastic.out(1, 0.3)', overwrite: 'auto' },
  );
}

export { gsap };
