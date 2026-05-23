import { useState } from "react";

interface ConfirmModalProps {
  isOpen: boolean;
  title: string;
  message: string;
  confirmText?: string;
  cancelText?: string;
  onConfirm: () => void;
  onCancel: () => void;
  isDestructive?: boolean;
}

export default function ConfirmModal({
  isOpen,
  title,
  message,
  confirmText = "Confirm",
  cancelText = "Cancel",
  onConfirm,
  onCancel,
  isDestructive = false,
}: ConfirmModalProps) {
  if (!isOpen) return null;

  return (
    <div className="modal-overlay">
      <div className="modal-panel" role="dialog" aria-modal="true">
        <h3 className="text-lg font-semibold text-text mb-2">{title}</h3>
        <p className="text-sm text-secondary mb-5">{message}</p>

        <div className="flex gap-2 justify-end">
          <button type="button" onClick={onCancel} className="btn btn-secondary">
            {cancelText}
          </button>
          <button
            type="button"
            onClick={onConfirm}
            className={`btn ${isDestructive ? "btn-danger" : "btn-primary"}`}
          >
            {confirmText}
          </button>
        </div>
      </div>
    </div>
  );
}

export function useConfirmModal() {
  const [state, setState] = useState<{
    isOpen: boolean;
    title: string;
    message: string;
    confirmText?: string;
    isDestructive?: boolean;
    resolve?: (value: boolean) => void;
  }>({
    isOpen: false,
    title: "",
    message: "",
  });

  const confirm = (options: {
    title: string;
    message: string;
    confirmText?: string;
    isDestructive?: boolean;
  }): Promise<boolean> => {
    return new Promise((resolve) => {
      setState({
        isOpen: true,
        ...options,
        resolve,
      });
    });
  };

  const handleConfirm = () => {
    state.resolve?.(true);
    setState((s) => ({ ...s, isOpen: false }));
  };

  const handleCancel = () => {
    state.resolve?.(false);
    setState((s) => ({ ...s, isOpen: false }));
  };

  const ConfirmModalComponent = () => (
    <ConfirmModal
      isOpen={state.isOpen}
      title={state.title}
      message={state.message}
      confirmText={state.confirmText}
      isDestructive={state.isDestructive}
      onConfirm={handleConfirm}
      onCancel={handleCancel}
    />
  );

  return { confirm, ConfirmModalComponent };
}
