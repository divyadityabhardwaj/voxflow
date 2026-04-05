import { useState, useRef, KeyboardEvent, useEffect } from "react";

interface HotkeyInputProps {
  value: string;
  onChange: (value: string) => void;
  onBlur: (value: string) => void;
  placeholder?: string;
  className?: string;
  disabled?: boolean;
}

export default function HotkeyInput({
  value,
  onChange,
  onBlur,
  placeholder = "Click to record shortcut",
  className = "",
  disabled = false,
}: HotkeyInputProps) {
  const [recording, setRecording] = useState(false);
  const inputRef = useRef<HTMLInputElement>(null);

  const keyMap: Record<string, string> = {
    " ": "space",
    Enter: "return",
    ArrowUp: "up",
    ArrowDown: "down",
    ArrowLeft: "left",
    ArrowRight: "right",
    Escape: "escape",
    Tab: "tab",
  };

  const handleKeyDown = (e: KeyboardEvent<HTMLInputElement>) => {
    e.preventDefault();
    e.stopPropagation();

    if (disabled) return;

    const parts: string[] = [];

    if (e.metaKey) parts.push("cmd");
    if (e.ctrlKey) parts.push("ctrl");
    if (e.altKey) parts.push("alt");
    if (e.shiftKey) parts.push("shift");

    if (
      ["Meta", "Control", "Alt", "Shift", "Option", "Command"].includes(e.key)
    ) {
    } else {
      let key = e.key;

      if (keyMap[key]) {
        key = keyMap[key];
      } else if (key.length === 1) {
        key = key.toLowerCase();
      } else {
        key = key.toLowerCase();
      }

      parts.push(key);
    }

    const newValue = parts.join("+");

    if (parts.length > 0) {
      onChange(newValue);
    }
  };

  const handleFocus = () => {
    setRecording(true);
  };

  const handleBlur = () => {
    setRecording(false);
    onBlur(value);
  };

  return (
    <div className={`relative ${className}`}>
      <input
        ref={inputRef}
        type="text"
        value={value}
        readOnly
        onKeyDown={handleKeyDown}
        onFocus={handleFocus}
        onBlur={handleBlur}
        placeholder={recording ? "Recording..." : placeholder}
        disabled={disabled}
        className={`w-full px-4 py-2.5 bg-background border-2 border-border rounded-xl text-text font-bold
                   focus:outline-none focus:border-primary cursor-pointer
                   ${
                     recording
                       ? "border-primary"
                       : ""
                   }
                   ${
                     disabled
                       ? "opacity-50 cursor-not-allowed"
                       : "hover:border-border-hover"
                   }
                   transition-all duration-150`}
      />
      {recording && (
        <div className="absolute right-3 top-1/2 -translate-y-1/2">
          <span className="flex h-2 w-2">
            <span className="animate-ping absolute inline-flex h-full w-full rounded-full bg-red-400 opacity-75"></span>
            <span className="relative inline-flex rounded-full h-2 w-2 bg-red-500"></span>
          </span>
        </div>
      )}
    </div>
  );
}
