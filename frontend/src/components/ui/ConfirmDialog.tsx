/**
 * 确认对话框 —— 删除等危险操作的二次确认。
 */
import type { ReactNode } from 'react';
import { Modal } from './Modal';
import { Button } from './primitives';

interface ConfirmProps {
  open: boolean;
  title: string;
  content: ReactNode;
  confirmText?: string;
  danger?: boolean;
  loading?: boolean;
  onCancel: () => void;
  onConfirm: () => void;
}

export function ConfirmDialog({
  open,
  title,
  content,
  confirmText = '确认',
  danger,
  loading,
  onCancel,
  onConfirm,
}: ConfirmProps) {
  return (
    <Modal
      open={open}
      title={title}
      onClose={onCancel}
      width="max-w-sm"
      footer={
        <>
          <Button variant="ghost" onClick={onCancel}>
            取消
          </Button>
          <Button variant={danger ? 'danger' : 'primary'} loading={loading} onClick={onConfirm}>
            {confirmText}
          </Button>
        </>
      }
    >
      <div className="text-sm text-fog">{content}</div>
    </Modal>
  );
}
