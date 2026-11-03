import type { CSSProperties } from "react";
import { useTheme, type ThemePreference } from "../../contexts/ThemeContext";
import { onRadioGroupKeyDown } from "../../lib/radioGroup";
import SettingsSection from "./SettingsSection";

const THEMES: { id: ThemePreference; label: string; canvas: string; sidebar: string; surface: string; ink: string }[] = [
  { id: "system", label: "System", canvas: "linear-gradient(90deg, #f8f9fb 50%, #0b0b0c 50%)", sidebar: "#c9ced8", surface: "#9ca3af", ink: "#727782" },
  { id: "light", label: "Light", canvas: "#f8f9fb", sidebar: "#edf0f5", surface: "#ffffff", ink: "#c0c8d5" },
  { id: "dark", label: "Dark", canvas: "#0b0b0c", sidebar: "#080809", surface: "#1d1d21", ink: "#51515c" },
  { id: "midnight", label: "Midnight", canvas: "#0c1019", sidebar: "#090e17", surface: "#1c2639", ink: "#4e627f" },
];

export default function AppearanceSettings() {
  const { preference, setTheme } = useTheme();
  return (
    <SettingsSection title="Theme">
      <div role="radiogroup" aria-label="Theme" className="settings-pad grid grid-cols-2 sm:grid-cols-4 gap-3"
        onKeyDown={(e) => onRadioGroupKeyDown(e, THEMES.map((t) => t.id), preference, setTheme)}>
        {THEMES.map((mode) => (
          <button key={mode.id} type="button" role="radio" aria-checked={preference === mode.id}
            tabIndex={preference === mode.id ? 0 : -1} onClick={() => setTheme(mode.id)}
            className="theme-option" data-active={preference === mode.id ? "true" : "false"}>
            <span className="theme-thumbnail" aria-hidden="true" style={{
              "--preview-canvas": mode.canvas, "--preview-sidebar": mode.sidebar,
              "--preview-surface": mode.surface, "--preview-ink": mode.ink,
            } as CSSProperties}>
              <span className="theme-thumbnail-sidebar"><i /><i /><i /></span>
              <span className="theme-thumbnail-content"><i /><span><i /><i /></span></span>
            </span>
            <span className="flex items-center justify-between gap-2 mt-2.5 text-xs">
              {mode.label}
              <span className="theme-check" aria-hidden="true">{preference === mode.id ? "✓" : ""}</span>
            </span>
          </button>
        ))}
      </div>
    </SettingsSection>
  );
}
