import { useCallback, useId, useState } from "react";
import { useDialog } from "../lib/useDialog";

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
  const titleId = useId();
  const messageId = useId();
  const panelRef = useDialog<HTMLDivElement>(isOpen, (e) => {
    if (e.key === "Escape") {
      e.preventDefault();
      onCancel();
    } else if (e.key === "Enter" && !(e.target instanceof HTMLButtonElement)) {
      e.preventDefault();
      onConfirm();
    }
  });

  if (!isOpen) return null;

  return (
    <div className="modal-overlay">
      <div
        ref={panelRef}
        className="modal-panel"
        role="dialog"
        aria-modal="true"
        aria-labelledby={titleId}
        aria-describedby={messageId}
        tabIndex={-1}
      >
        <h3 id={titleId} className="text-lg font-semibold text-text mb-2">
          {title}
        </h3>
        <p id={messageId} className="text-sm text-secondary mb-5">
          {message}
        </p>

        <div className="flex gap-2 justify-end">
          <button type="button" onClick={onCancel} className="btn btn-secondary">
            {cancelText}
          </button>
          <button
            type="button"
            onClick={onConfirm}
            className={`btn ${isDestructive ? "btn-danger" : "btn-primary"}`}
            data-autofocus
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

  // Stable while the dialog state is unchanged, so a re-render of the caller
  // doesn't remount the dialog and reset its focus.
  const ConfirmModalComponent = useCallback(() => (
    <ConfirmModal
      isOpen={state.isOpen}
      title={state.title}
      message={state.message}
      confirmText={state.confirmText}
      isDestructive={state.isDestructive}
      onConfirm={handleConfirm}
      onCancel={handleCancel}
    />
  ), [state]);

  return { confirm, ConfirmModalComponent };
}
