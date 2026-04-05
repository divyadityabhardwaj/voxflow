import { useState, useEffect, useCallback, useRef, useMemo } from "react";

interface HotkeyRecorderModalProps {
  isOpen: boolean;
  onClose: () => void;
  onSave: (hotkey: string) => void;
  initialValue?: string;
}

const MODIFIERS = new Set(["cmd", "ctrl", "alt", "shift", "win", "super"]);

const isMac =
  typeof navigator !== "undefined" &&
  navigator.platform.toLowerCase().includes("mac");

export default function HotkeyRecorderModal({
  isOpen,
  onClose,
  onSave,
  initialValue = "",
}: HotkeyRecorderModalProps) {
  const [currentKeys, setCurrentKeys] = useState<Set<string>>(new Set());
  const [displayKeys, setDisplayKeys] = useState<string[]>([]);
  const keysRef = useRef<Set<string>>(new Set());
  const [hasStartedRecording, setHasStartedRecording] = useState(false);

  const validation = useMemo(() => {
    if (displayKeys.length === 0) {
      return { isValid: true, message: "" };
    }

    const modifierKeys = displayKeys.filter((k) => MODIFIERS.has(k));
    const regularKeys = displayKeys.filter((k) => !MODIFIERS.has(k));

    if (modifierKeys.length === 0) {
      return {
        isValid: false,
        message: isMac
          ? "Global shortcuts require a modifier (⌘ Cmd, ⌃ Ctrl, ⌥ Alt, or ⇧ Shift)"
          : "Global shortcuts require a modifier (Ctrl, Alt, Shift, or Win)",
      };
    }
    if (regularKeys.length === 0) {
      return {
        isValid: false,
        message: "Add a regular key (like D, Space, etc.) after your modifier",
      };
    }
    if (regularKeys.length > 1) {
      return {
        isValid: false,
        message:
          "Global shortcuts can only have ONE regular key (e.g., ⌘+D, not ⌘+D+E)",
      };
    }
    return { isValid: true, message: "" };
  }, [displayKeys]);

  useEffect(() => {
    if (isOpen) {
      setCurrentKeys(new Set());
      keysRef.current = new Set();
      setHasStartedRecording(false);
      if (initialValue) {
        const parts = initialValue.split("+");
        setDisplayKeys(parts);
      } else {
        setDisplayKeys([]);
      }
    }
  }, [isOpen, initialValue]);

  const mapKey = (key: string, code: string): string => {
    const codeMap: Record<string, string> = {
      Space: "space",
      Enter: "enter",
      Escape: "escape",
      Tab: "tab",
      ArrowUp: "up",
      ArrowDown: "down",
      ArrowLeft: "left",
      ArrowRight: "right",
      Backspace: "backspace",
      Delete: "delete",
    };

    if (codeMap[code]) return codeMap[code];
    if (codeMap[key]) return codeMap[key];

    const lowerKey = key.toLowerCase();
    if (lowerKey === "meta" || lowerKey === "os") return "cmd";
    if (lowerKey === "control") return "ctrl";
    if (lowerKey === "alt") return "alt";
    if (lowerKey === "shift") return "shift";

    if (key.length === 1) return lowerKey;

    return lowerKey;
  };

  const handleKeyDown = useCallback(
    (e: KeyboardEvent) => {
      e.preventDefault();
      e.stopPropagation();

      const key = mapKey(e.key, e.code);

      if (key === "enter" || key === "return") {
        if (displayKeys.length > 0 && validation.isValid) {
          onSave(displayKeys.join("+"));
          onClose();
        }
        return;
      }

      if (key === "escape") {
        onClose();
        return;
      }

      if (keysRef.current.size < 3 || keysRef.current.has(key)) {
        keysRef.current.add(key);
      }

      const newSet = new Set(keysRef.current);
      setCurrentKeys(newSet);

      const sorted = Array.from(newSet).sort((a, b) => {
        const order = { cmd: 1, ctrl: 2, alt: 3, shift: 4 };
        const orderA = order[a as keyof typeof order] || 99;
        const orderB = order[b as keyof typeof order] || 99;
        return orderA - orderB;
      });

      setDisplayKeys(sorted);
      setHasStartedRecording(true);
    },
    [onSave, onClose, displayKeys, validation.isValid]
  );

  const handleKeyUp = useCallback((e: KeyboardEvent) => {
    e.preventDefault();
    e.stopPropagation();

    const key = mapKey(e.key, e.code);

    keysRef.current.delete(key);
    setCurrentKeys(new Set(keysRef.current));
  }, []);

  useEffect(() => {
    if (isOpen) {
      window.addEventListener("keydown", handleKeyDown);
      window.addEventListener("keyup", handleKeyUp);
      return () => {
        window.removeEventListener("keydown", handleKeyDown);
        window.removeEventListener("keyup", handleKeyUp);
      };
    }
  }, [isOpen, handleKeyDown, handleKeyUp]);

  if (!isOpen) return null;

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/40 backdrop-blur-sm">
      <div className="bg-background border-2 border-border rounded-[1.5rem] p-6 w-full max-w-md shadow-soft-lg animate-scale-in">
        <div className="text-center space-y-5">
          <h3 className="text-xl font-bold text-text">
            Record Shortcut
          </h3>

          <div
            className={`py-6 flex items-center justify-center min-h-[100px] bg-secondary rounded-xl border-2 border-dashed ${
              !validation.isValid && displayKeys.length > 0
                ? "border-red-500"
                : "border-border"
            }`}
          >
            {displayKeys.length > 0 ? (
              <div className="flex flex-wrap gap-2 justify-center">
                {displayKeys.map((k, i) => (
                  <div key={i} className="flex items-center">
                    <kbd className="px-3 py-1.5 bg-background border-2 border-border rounded-lg text-lg font-mono font-bold text-primary">
                      {k === "cmd" || k === "win" || k === "super"
                        ? isMac
                          ? "⌘"
                          : "Win"
                        : k === "shift"
                        ? isMac
                          ? "⇧"
                          : "Shift"
                        : k === "ctrl"
                        ? isMac
                          ? "⌃"
                          : "Ctrl"
                        : k === "opt" || k === "alt"
                        ? isMac
                          ? "⌥"
                          : "Alt"
                        : k.toUpperCase()}
                    </kbd>
                    {i < displayKeys.length - 1 && (
                      <span className="mx-2 text-text font-bold">
                        +
                      </span>
                    )}
                  </div>
                ))}
              </div>
            ) : (
              <p className="text-tertiary italic font-medium">Press keys...</p>
            )}
          </div>

          {!validation.isValid && displayKeys.length > 0 && (
            <div className="p-3 bg-red-500/10 border border-red-500 rounded-xl">
              <p className="text-sm font-bold text-red-500">{validation.message}</p>
            </div>
          )}

          <div className="space-y-2">
            <p className="text-sm text-secondary font-medium">
              Press the desired key combination (max 3 keys).
            </p>
            <p className="text-xs text-primary font-bold">
              Press{" "}
              <kbd className="font-mono bg-secondary px-2 py-0.5 rounded-lg border border-border">Enter</kbd> to
              save •{" "}
              <kbd className="font-mono bg-secondary px-2 py-0.5 rounded-lg border border-border">Esc</kbd> to
              cancel
            </p>
          </div>
        </div>
      </div>
    </div>
  );
}
