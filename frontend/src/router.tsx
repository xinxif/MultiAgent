/**
 * 路由表 —— 含登录守卫与 admin 守卫。
 *
 * 守卫策略：bootstrap 完成前渲染启动屏（防止刷新闪跳登录页）；
 * 未登录 → 重定向 /login；非 admin 访问 /admin/* → 重定向 /chat。
 */
import { Navigate, Route, Routes, useLocation } from 'react-router-dom';
import type { ReactNode } from 'react';
import { useAuth } from '@/stores/auth';
import { AppLayout } from '@/components/layout/AppLayout';
import { Spinner } from '@/components/ui/primitives';
import { LoginPage } from '@/pages/LoginPage';
import { ChatPage } from '@/pages/chat/ChatPage';
import { AgentsPage } from '@/pages/agents/AgentsPage';
import { MemoriesPage } from '@/pages/memories/MemoriesPage';
import { KnowledgeBasesPage } from '@/pages/kbs/KnowledgeBasesPage';
import { KBDetailPage } from '@/pages/kbs/KBDetailPage';
import { WorkflowsPage } from '@/pages/workflows/WorkflowsPage';
import { WorkflowRunsPage } from '@/pages/workflows/WorkflowRunsPage';
import { ModelsAdminPage } from '@/pages/admin/ModelsAdminPage';
import { IntegrationsPage } from '@/pages/admin/IntegrationsPage';
import { NotFoundPage } from '@/pages/NotFoundPage';

/** 登录守卫 */
function RequireAuth({ children }: { children: ReactNode }) {
  const { user, bootstrapped } = useAuth();
  const location = useLocation();
  if (!bootstrapped) return <BootScreen />;
  if (!user) return <Navigate to="/login" state={{ from: location.pathname }} replace />;
  return <>{children}</>;
}

/** admin 守卫 */
function RequireAdmin({ children }: { children: ReactNode }) {
  const user = useAuth((s) => s.user);
  if (user?.role !== 'admin') return <Navigate to="/chat" replace />;
  return <>{children}</>;
}

/** 启动屏：会话恢复中 */
function BootScreen() {
  return (
    <div className="flex h-full items-center justify-center gap-3 text-fog">
      <Spinner />
      <span className="text-sm">正在恢复会话…</span>
    </div>
  );
}

export function AppRoutes() {
  return (
    <Routes>
      <Route path="/login" element={<LoginPage />} />

      <Route
        element={
          <RequireAuth>
            <AppLayout />
          </RequireAuth>
        }
      >
        <Route path="/chat" element={<ChatPage />} />
        <Route path="/agents" element={<AgentsPage />} />
        <Route path="/memories" element={<MemoriesPage />} />
        <Route path="/kbs" element={<KnowledgeBasesPage />} />
        <Route path="/kbs/:id" element={<KBDetailPage />} />
        <Route path="/workflows" element={<WorkflowsPage />} />
        <Route path="/workflows/runs" element={<WorkflowRunsPage />} />
        <Route
          path="/admin/models"
          element={
            <RequireAdmin>
              <ModelsAdminPage />
            </RequireAdmin>
          }
        />
        <Route
          path="/admin/integrations"
          element={
            <RequireAdmin>
              <IntegrationsPage />
            </RequireAdmin>
          }
        />
        <Route path="/" element={<Navigate to="/chat" replace />} />
        <Route path="*" element={<NotFoundPage />} />
      </Route>
    </Routes>
  );
}
