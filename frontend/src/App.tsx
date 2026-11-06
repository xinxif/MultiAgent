/**
 * 应用根组件：全局只做一次 bootstrap（refresh_token 会话恢复）。
 * 路由守卫依赖 auth.bootstrapped，恢复完成前显示启动屏。
 */
import { useEffect } from 'react';
import { useAuth } from '@/stores/auth';
import { AppRoutes } from '@/router';
import { ToastHost } from '@/components/ui/ToastHost';

export default function App() {
  const bootstrap = useAuth((s) => s.bootstrap);

  useEffect(() => {
    void bootstrap();
  }, [bootstrap]);

  return (
    <>
      <AppRoutes />
      <ToastHost />
    </>
  );
}
