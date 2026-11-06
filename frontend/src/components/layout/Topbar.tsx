/**
 * 顶栏 —— 页面标题区 + 用户信息/登出。
 */
import { useNavigate } from 'react-router-dom';
import { useAuth } from '@/stores/auth';
import { Badge } from '@/components/ui/primitives';

export function Topbar() {
  const user = useAuth((s) => s.user);
  const logout = useAuth((s) => s.logout);
  const navigate = useNavigate();

  const handleLogout = async () => {
    await logout();
    navigate('/login');
  };

  return (
    <header className="flex h-14 shrink-0 items-center justify-between border-b border-smoke/60 bg-carbon/80 px-6 backdrop-blur">
      <div className="text-xs text-ash">企业级全能 AI 智能体平台</div>
      {user && (
        <div className="flex items-center gap-3">
          <Badge tone={user.role === 'admin' ? 'lime' : 'gray'}>{user.role}</Badge>
          <span className="text-sm text-mist">{user.username}</span>
          <button
            onClick={handleLogout}
            className="text-xs text-ash transition-colors hover:text-coral"
          >
            退出
          </button>
        </div>
      )}
    </header>
  );
}
