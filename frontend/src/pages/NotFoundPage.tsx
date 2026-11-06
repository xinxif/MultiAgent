import { useNavigate } from 'react-router-dom';
import { EmptyState, Button } from '@/components/ui/primitives';

export function NotFoundPage() {
  const navigate = useNavigate();
  return (
    <div className="page-container">
      <EmptyState
        icon="⌀"
        title="页面不存在"
        hint="你可能访问了一个被删除或从未存在的路由"
        action={<Button onClick={() => navigate('/chat')}>回到对话</Button>}
      />
    </div>
  );
}
