import { useState, useEffect, useCallback, useRef } from "react";

interface HotkeyRecorderModalProps {
  isOpen: boolean;
  onClose: () => void;
  onSave: (hotkey: string) => void;
  initialValue?: string;
}

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

      // Save on Enter/Return
      if (key === "enter" || key === "return") {
        // Use displayKeys instead of keysRef.current because
        // the user may have released modifier keys before pressing Enter
        if (displayKeys.length > 0) {
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
    [onSave, onClose, displayKeys]
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

          <div className="py-8 flex items-center justify-center min-h-[120px] bg-dark-800 rounded-lg border-2 border-dashed border-dark-600">
            {displayKeys.length > 0 ? (
              <div className="flex flex-wrap gap-2 justify-center">
                {displayKeys.map((k, i) => (
                  <div key={i} className="flex items-center">
                    <kbd className="px-3 py-1.5 bg-dark-700 border-b-4 border-dark-600 rounded-lg text-lg font-mono text-accent-400 min-w-[3rem] text-center shadow-sm">
                      {k === "cmd"
                        ? "⌘"
                        : k === "shift"
                        ? "⇧"
                        : k === "opt" || k === "alt"
                        ? "⌥"
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
