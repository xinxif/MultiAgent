/**
 * 侧边栏导航 —— 固定左侧，导航项 hover 微亮 + 激活态荧光绿指示条。
 */
import { NavLink } from 'react-router-dom';
import { clsx } from 'clsx';
import { useAuth } from '@/stores/auth';

/** 导航项配置：admin 专属项带 isAdmin 标记 */
interface NavItem {
  to: string;
  label: string;
  icon: string;
  isAdmin?: boolean;
}

const NAV: NavItem[] = [
  { to: '/chat', label: '对话', icon: '◆' },
  { to: '/agents', label: 'Agent', icon: '⬡' },
  { to: '/memories', label: '记忆', icon: '❖' },
  { to: '/kbs', label: '知识库', icon: '▤' },
  { to: '/workflows', label: '工作流', icon: '⇶' },
  { to: '/admin/models', label: '模型管理', icon: '⚙', isAdmin: true },
  { to: '/admin/integrations', label: '集成中心', icon: '◉', isAdmin: true },
];

export function Sidebar() {
  const user = useAuth((s) => s.user);

  return (
    <aside className="flex h-full w-56 shrink-0 flex-col border-r border-smoke/60 bg-carbon">
      {/* Logo */}
      <div className="flex h-14 items-center gap-2.5 px-5">
        <span className="text-lg text-glow text-lime">◆</span>
        <span className="text-sm font-bold tracking-wider text-bone">
          <span className="text-lime">Agent</span>
        </span>
      </div>

      {/* 导航项 */}
      <nav className="mt-2 flex-1 space-y-0.5 px-3">
        {NAV.filter((n) => !n.isAdmin || user?.role === 'admin').map((n) => (
          <NavLink
            key={n.to}
            to={n.to}
            className={({ isActive }) =>
              clsx(
                'group relative flex items-center gap-3 rounded-lg px-3 py-2 text-sm transition-colors duration-200',
                isActive
                  ? 'bg-obsidian text-lime'
                  : 'text-fog hover:bg-obsidian/60 hover:text-mist',
              )
            }
          >
            {/* 激活指示条 */}
            {({ isActive }) => (
              <>
                <span
                  className={clsx(
                    'absolute left-0 h-4 w-0.5 rounded-full bg-lime transition-opacity',
                    isActive ? 'opacity-100' : 'opacity-0',
                  )}
                />
                <span className="text-xs opacity-80">{n.icon}</span>
                <span>{n.label}</span>
              </>
            )}
          </NavLink>
        ))}
      </nav>

      {/* 版本脚注 */}
      <div className="px-5 py-4 text-[10px] leading-relaxed text-ash">
        Agent v1.0
        <br />
        Go · Eino · React
      </div>
    </aside>
  );
}
