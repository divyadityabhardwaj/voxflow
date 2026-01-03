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

  // Map special key values to what the backend expects
  const keyMap: Record<string, string> = {
    " ": "space",
    Enter: "return", // or "enter"
    ArrowUp: "up", // backend might not support yet, but good to have
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

    // Check modifiers
    if (e.metaKey) parts.push("cmd");
    if (e.ctrlKey) parts.push("ctrl");
    if (e.altKey) parts.push("alt");
    if (e.shiftKey) parts.push("shift");

    // Check main key
    // Ignore if the pressed key is just a modifier
    if (
      ["Meta", "Control", "Alt", "Shift", "Option", "Command"].includes(e.key)
    ) {
      // Just modifiers pressed so far
    } else {
      let key = e.key;

      // Handle special keys
      if (keyMap[key]) {
        key = keyMap[key];
      } else if (key.length === 1) {
        key = key.toLowerCase();
      } else {
        // Fn keys etc - keep as is (lowercase)
        key = key.toLowerCase();
      }

      parts.push(key);
    }

    // Construct string
    const newValue = parts.join("+");

    // Call onChange with the new value
    // Only if we actually have something (even just modifiers)
    // Actually standard UX: show the modifiers while holding
    // But we only want to "save" valid ones?
    // For now, let's just push whatever the user types.
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

  // formatting for display: valid hotkeys usually have at least one modifier + key, or just special keys
  // For display purposes, we might want to prettify symbols?
  // e.g. "cmd" -> "⌘", "shift" -> "⇧" etc.
  // But for the *value* in the input, let's keep it raw string for now to match backend expectations.
  // We can add a "display" layer later if needed. The user wanted "like VS Code".
  // VS Code shows the text like "Cmd+Shift+P".

  return (
    <div className={`relative ${className}`}>
      <input
        ref={inputRef}
        type="text"
        value={value}
        readOnly // Prevent typing
        onKeyDown={handleKeyDown}
        onFocus={handleFocus}
        onBlur={handleBlur}
        placeholder={recording ? "Recording..." : placeholder}
        disabled={disabled}
        className={`w-full px-4 py-2.5 bg-dark-800 border rounded-lg text-dark-200 
                   focus:outline-none focus:ring-2 focus:ring-accent-600 cursor-pointer
                   ${
                     recording
                       ? "border-accent-600 bg-dark-900 ring-1 ring-accent-600"
                       : "border-dark-700"
                   }
                   ${
                     disabled
                       ? "opacity-50 cursor-not-allowed"
                       : "hover:border-dark-600"
                   }
                   transition-all duration-200`}
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
