/**
 * 应用布局壳 —— 侧边栏 + 顶栏 + 内容区。
 * 路由切换时对内容区做 GSAP 淡入（页面过渡动画）。
 */
import { useEffect, useRef } from 'react';
import { Outlet, useLocation } from 'react-router-dom';
import { Sidebar } from './Sidebar';
import { Topbar } from './Topbar';
import { pageEnter } from '@/lib/gsap';

export function AppLayout() {
  const location = useLocation();
  const mainRef = useRef<HTMLDivElement>(null);

  // 每次路由变化：内容区入场动画
  useEffect(() => {
    if (mainRef.current) pageEnter(mainRef.current);
  }, [location.pathname]);

  return (
    <div className="flex h-full overflow-hidden">
      <Sidebar />
      <div className="flex min-w-0 flex-1 flex-col">
        <Topbar />
        <main ref={mainRef} className="min-h-0 flex-1 overflow-y-auto">
          <Outlet />
        </main>
      </div>
    </div>
  );
}
