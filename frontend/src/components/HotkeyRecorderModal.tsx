import { useState, useEffect, useId } from "react";
import {
  formatShortcut,
  hasShortcutKey,
  keyFromCode,
  modifiersOf,
  normalizeShortcut,
  validateShortcut,
} from "../lib/shortcut";
import { SuspendHotkeys } from "../../wailsjs/go/main/App";
import { useDialog } from "../lib/useDialog";

interface HotkeyRecorderModalProps {
  isOpen: boolean;
  onClose: () => void;
  // Reject to keep the modal open; the error message is shown inline.
  onSave: (hotkey: string) => void | Promise<void>;
  initialValue?: string;
  otherHotkeys?: string[];
}

const MODIFIER_CODES = /^(Meta|Control|Alt|Shift)(Left|Right)$/;

export default function HotkeyRecorderModal({
  isOpen,
  onClose,
  onSave,
  initialValue = "",
  otherHotkeys = [],
}: HotkeyRecorderModalProps) {
  const [combo, setCombo] = useState("");
  const [error, setError] = useState<string | null>(null);
  const [saving, setSaving] = useState(false);
  const titleId = useId();

  useEffect(() => {
    if (isOpen) {
      setCombo(normalizeShortcut(initialValue));
      setError(null);
      setSaving(false);
    }
  }, [isOpen, initialValue]);

  // Otherwise pressing the current shortcut triggers recording instead of being captured.
  useEffect(() => {
    if (!isOpen) return;
    SuspendHotkeys(true).catch(() => {});
    return () => void SuspendHotkeys(false).catch(() => {});
  }, [isOpen]);

  const problem = combo ? validateShortcut(combo, otherHotkeys) : null;
  const canSave = !!combo && !problem && !saving;

  const save = async () => {
    if (!canSave) return;
    if (combo === normalizeShortcut(initialValue)) {
      onClose();
      return;
    }
    setSaving(true);
    setError(null);
    try {
      // Resume first so registering the new shortcut can fail visibly.
      await SuspendHotkeys(false);
      await onSave(combo);
      onClose();
    } catch (err) {
      SuspendHotkeys(true).catch(() => {});
      setError(err instanceof Error ? err.message : String(err));
    } finally {
      setSaving(false);
    }
  };

  const handleKeyDown = (e: KeyboardEvent) => {
    const mods = modifiersOf(e);
    // Bare Tab/Space/Return/Esc can't be shortcuts (a modifier is required),
    // so they drive the dialog; with a modifier they're recorded.
    if (mods.length === 0) {
      if (e.code === "Tab") return;
      const onButton = document.activeElement instanceof HTMLButtonElement;
      if ((e.code === "Enter" || e.code === "Space") && onButton) return;
      if (e.code === "Escape") {
        e.preventDefault();
        return onClose();
      }
      if (e.code === "Enter") {
        e.preventDefault();
        return void save();
      }
    }
    e.preventDefault();
    if (saving) return;

    setError(null);
    if (MODIFIER_CODES.test(e.code)) {
      setCombo(mods.join("+"));
      return;
    }
    const key = keyFromCode(e.code);
    if (!key) {
      setError(
        `${e.code || e.key} can't be used. Pick a letter, number, Space, Return, Esc or Tab.`,
      );
      return;
    }
    setCombo([...mods, key].join("+"));
  };

  const panelRef = useDialog<HTMLDivElement>(isOpen, handleKeyDown);

  useEffect(() => {
    if (!isOpen) return;
    // macOS drops keyup for keys released while ⌘ is held, so only track
    // modifier releases, and only while no key has been chosen yet.
    const handleKeyUp = (e: KeyboardEvent) =>
      setCombo((c) => (hasShortcutKey(c) ? c : modifiersOf(e).join("+")));
    window.addEventListener("keyup", handleKeyUp);
    return () => window.removeEventListener("keyup", handleKeyUp);
  }, [isOpen]);

  if (!isOpen) return null;

  const message = error ?? problem;

  return (
    <div className="modal-overlay">
      <div
        ref={panelRef}
        className="modal-panel max-w-md"
        role="dialog"
        aria-modal="true"
        aria-labelledby={titleId}
        tabIndex={-1}
      >
        <div className="text-center space-y-5">
          <h3 id={titleId} className="text-lg font-semibold text-text">
            Record shortcut
          </h3>

          <div
            className={`py-6 flex items-center justify-center min-h-[100px] bg-background rounded-md border border-dashed ${
              message ? "border-danger" : "border-border"
            }`}
            aria-live="polite"
          >
            {combo ? (
              <kbd className="px-3 py-1.5 bg-surface border border-border rounded-md text-xl font-medium text-primary tracking-wide">
                {formatShortcut(combo)}
              </kbd>
            ) : (
              <p className="text-tertiary text-sm">Press keys…</p>
            )}
          </div>

          {message && (
            <div className="p-3 rounded-md border border-danger/30 bg-danger/10" role="alert">
              <p className="text-sm text-danger">{message}</p>
            </div>
          )}

          <p className="text-sm text-secondary">
            Use one key plus modifiers, like ⇧⌘D.
          </p>

          <div className="flex justify-end gap-2">
            <button type="button" onClick={onClose} className="btn btn-secondary">
              Cancel
            </button>
            <button
              type="button"
              onClick={save}
              disabled={!canSave}
              className="btn btn-primary"
            >
              {saving ? "Saving…" : "Save"}
            </button>
          </div>
        </div>
      </div>
    </div>
  );
}
