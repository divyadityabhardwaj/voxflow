import { useState, useEffect, useCallback, useRef, useMemo } from "react";

interface HotkeyRecorderModalProps {
  isOpen: boolean;
  onClose: () => void;
  onSave: (hotkey: string) => void;
  initialValue?: string;
}

const MODIFIERS = new Set(["cmd", "ctrl", "alt", "shift", "win", "super"]);

// Detect platform for display purposes
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
  // Ref to keep track of keys synchronously for the event handler
  const keysRef = useRef<Set<string>>(new Set());

  // We keep track if the user has actively pressed a new combo to replace the initial one
  const [hasStartedRecording, setHasStartedRecording] = useState(false);

  // Validation: must have at least one modifier AND exactly one non-modifier
  const validation = useMemo(() => {
    if (displayKeys.length === 0) {
      return { isValid: true, message: "" }; // No input yet, no error
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

  // Initialize display keys from initialValue
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
    // Normalization map
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

    // Modifiers
    const lowerKey = key.toLowerCase();
    if (lowerKey === "meta" || lowerKey === "os") return "cmd";
    if (lowerKey === "control") return "ctrl";
    if (lowerKey === "alt") return "alt";
    if (lowerKey === "shift") return "shift";

    // Single characters
    if (key.length === 1) return lowerKey;

    // Fallback for function keys etc
    return lowerKey;
  };

  const handleKeyDown = useCallback(
    (e: KeyboardEvent) => {
      // Prevent default browser actions for common shortcuts while recording
      e.preventDefault();
      e.stopPropagation();

      const key = mapKey(e.key, e.code);

      // Save on Enter/Return (only if valid)
      if (key === "enter" || key === "return") {
        // Use displayKeys and check validation
        if (displayKeys.length > 0 && validation.isValid) {
          onSave(displayKeys.join("+"));
          onClose();
        }
        return;
      }

      // Cancel on Escape
      if (key === "escape") {
        onClose();
        return;
      }

      // Add key to set (limit 3)
      // Only add if we haven't reached the limit or if it's already there
      if (keysRef.current.size < 3 || keysRef.current.has(key)) {
        keysRef.current.add(key);
      }

      // Update state for re-render
      const newSet = new Set(keysRef.current);
      setCurrentKeys(newSet);

      // Update display keys based on this new set
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
    // Note: We DO NOT update displayKeys here.
    // We want the last pressed combination to remain visible
    // so the user can see what they are about to save.
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
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/60 backdrop-blur-sm">
      <div className="bg-dark-900 border border-dark-700 rounded-xl p-8 w-full max-w-md shadow-2xl transform transition-all">
        <div className="text-center space-y-6">
          <h3 className="text-xl font-semibold text-dark-100">
            Record Shortcut
          </h3>

          <div
            className={`py-8 flex items-center justify-center min-h-[120px] bg-dark-800 rounded-lg border-2 border-dashed ${
              !validation.isValid && displayKeys.length > 0
                ? "border-red-500"
                : "border-dark-600"
            }`}
          >
            {displayKeys.length > 0 ? (
              <div className="flex flex-wrap gap-2 justify-center">
                {displayKeys.map((k, i) => (
                  <div key={i} className="flex items-center">
                    <kbd className="px-3 py-1.5 bg-dark-700 border-b-4 border-dark-600 rounded-lg text-lg font-mono text-accent-400 min-w-[3rem] text-center shadow-sm">
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
                      <span className="mx-2 text-dark-500 text-xl font-bold">
                        +
                      </span>
                    )}
                  </div>
                ))}
              </div>
            ) : (
              <p className="text-dark-500 italic">Press keys...</p>
            )}
          </div>

          {/* Validation warning */}
          {!validation.isValid && displayKeys.length > 0 && (
            <div className="p-3 bg-red-900/20 border border-red-700 rounded-lg">
              <p className="text-sm text-red-400">{validation.message}</p>
            </div>
          )}

          <div className="space-y-2">
            <p className="text-sm text-dark-300">
              Press the desired key combination (max 3 keys).
            </p>
            <p className="text-xs text-accent-400 font-medium">
              Press{" "}
              <kbd className="font-mono bg-dark-800 px-1 rounded">Enter</kbd> to
              save •{" "}
              <kbd className="font-mono bg-dark-800 px-1 rounded">Esc</kbd> to
              cancel
            </p>
          </div>
        </div>
      </div>
    </div>
  );
}
