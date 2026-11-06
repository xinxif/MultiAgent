/**
 * 登录 / 注册页 —— 单页双模式切换。
 * 动效：卡片入场、切换过渡、失败抖动（GSAP）。
 */
import { useEffect, useRef, useState, type FormEvent } from 'react';
import { Navigate, useNavigate } from 'react-router-dom';
import { gsap, shake } from '@/lib/gsap';
import { useAuth } from '@/stores/auth';
import { toast } from '@/stores/ui';
import { errMsg } from '@/lib/request';
import { Button, Input } from '@/components/ui/primitives';

type Mode = 'login' | 'register';

export function LoginPage() {
  const navigate = useNavigate();
  const { user, bootstrapped, login, register } = useAuth();

  const cardRef = useRef<HTMLDivElement>(null);
  const [mode, setMode] = useState<Mode>('login');
  const [username, setUsername] = useState('');
  const [email, setEmail] = useState('');
  const [password, setPassword] = useState('');
  const [loading, setLoading] = useState(false);

  // 入场动画：卡片上浮 + 标题辉光呼吸
  useEffect(() => {
    const ctx = gsap.context(() => {
      gsap.fromTo(
        cardRef.current,
        { opacity: 0, y: 24, scale: 0.98 },
        { opacity: 1, y: 0, scale: 1, duration: 0.55, ease: 'power3.out' },
      );
    });
    return () => ctx.revert();
  }, []);

  // 已登录直接回对话页
  if (bootstrapped && user) return <Navigate to="/chat" replace />;

  const submit = async (e: FormEvent) => {
    e.preventDefault();
    if (!username || !password) {
      if (cardRef.current) shake(cardRef.current);
      toast.err('请填写用户名与密码');
      return;
    }
    setLoading(true);
    try {
      if (mode === 'register') {
        await register({ username, email, password });
        toast.ok('注册成功，已自动登录');
      }
      await login({ username, password });
      navigate('/chat');
    } catch (err) {
      if (cardRef.current) shake(cardRef.current);
      toast.err(errMsg(err));
    } finally {
      setLoading(false);
    }
  };

  return (
    <div className="flex h-full items-center justify-center bg-void p-4">
      {/* 背景氛围光 */}
      <div className="pointer-events-none fixed inset-0 overflow-hidden">
        <div className="absolute -top-40 left-1/2 h-96 w-96 -translate-x-1/2 rounded-full bg-lime/5 blur-3xl" />
      </div>

      <div ref={cardRef} className="panel w-full max-w-sm p-8 shadow-2xl">
        {/* Logo */}
        <div className="mb-8 text-center">
          <div className="text-2xl text-glow text-lime">◆</div>
          <h1 className="mt-2 text-lg font-bold tracking-wide text-bone">
            <span className="text-lime">Agent</span> 控制台
          </h1>
          <p className="mt-1 text-xs text-ash">企业级全能 AI 智能体平台</p>
        </div>

        <form onSubmit={submit} className="space-y-4">
          <div>
            <label className="field-label">用户名</label>
            <Input
              value={username}
              onChange={(e) => setUsername(e.target.value)}
              placeholder="username"
              autoComplete="username"
            />
          </div>
          {mode === 'register' && (
            <div>
              <label className="field-label">邮箱</label>
              <Input
                type="email"
                value={email}
                onChange={(e) => setEmail(e.target.value)}
                placeholder="you@example.com"
                autoComplete="email"
              />
            </div>
          )}
          <div>
            <label className="field-label">密码</label>
            <Input
              type="password"
              value={password}
              onChange={(e) => setPassword(e.target.value)}
              placeholder="••••••••"
              autoComplete={mode === 'login' ? 'current-password' : 'new-password'}
            />
          </div>

          <Button type="submit" loading={loading} className="w-full">
            {mode === 'login' ? '登 录' : '注 册'}
          </Button>
        </form>

        <div className="mt-5 text-center text-xs text-ash">
          {mode === 'login' ? '还没有账号？' : '已有账号？'}
          <button
            className="ml-1 text-lime transition-opacity hover:opacity-80"
            onClick={() => setMode(mode === 'login' ? 'register' : 'login')}
          >
            {mode === 'login' ? '立即注册' : '去登录'}
          </button>
        </div>
      </div>
    </div>
  );
}
