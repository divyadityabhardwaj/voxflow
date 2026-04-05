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
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/40 backdrop-blur-sm">
      <div className="bg-background border-2 border-border rounded-[1.5rem] p-5 w-full max-w-md shadow-soft-lg animate-scale-in">
        <h3 className="text-lg font-bold text-text mb-2">{title}</h3>
        <p className="text-sm text-secondary font-medium mb-5">{message}</p>

        <div className="flex gap-3 justify-end">
          <button
            onClick={onCancel}
            className="px-4 py-2 text-sm font-bold text-text hover:bg-secondary rounded-xl transition-colors"
          >
            {cancelText}
          </button>
          <button
            onClick={onConfirm}
            className={`px-4 py-2 text-sm font-bold rounded-xl transition-all ${
              isDestructive
                ? "bg-red-500 hover:bg-red-400 text-white"
                : "bg-primary hover:bg-accent-hover text-white"
            }`}
          >
            {confirmText}
          </button>
        </div>
      </div>
    </div>
  );
}

// Hook for easier usage
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
