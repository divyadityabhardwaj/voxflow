import { useTheme, type ThemePreference } from "../../contexts/ThemeContext";
import { onRadioGroupKeyDown } from "../../lib/radioGroup";
import SettingsSection from "./SettingsSection";

const THEMES: { id: ThemePreference; label: string; swatch: string }[] = [
  {
    id: "system",
    label: "System",
    swatch: "bg-[linear-gradient(135deg,#f5f5f5_50%,#1e1e1e_50%)]",
  },
  { id: "light", label: "Light", swatch: "bg-[#f5f5f5]" },
  { id: "dark", label: "Dark", swatch: "bg-[#1e1e1e]" },
  { id: "midnight", label: "Midnight", swatch: "bg-[#080b12]" },
];

export default function AppearanceSettings() {
  const { preference, setTheme } = useTheme();

  return (
    <SettingsSection
      title="Appearance"
      description="System follows your Mac's light or dark setting. Buttons and highlights use your macOS accent colour."
    >
      <div
        role="radiogroup"
        aria-label="Theme"
        className="settings-pad grid grid-cols-2 sm:grid-cols-4 gap-2"
        onKeyDown={(e) =>
          onRadioGroupKeyDown(e, THEMES.map((t) => t.id), preference, setTheme)
        }
      >
        {THEMES.map((mode) => (
          <button
            key={mode.id}
            type="button"
            role="radio"
            aria-checked={preference === mode.id}
            tabIndex={preference === mode.id ? 0 : -1}
            onClick={() => setTheme(mode.id)}
            className="segmented-item flex flex-col items-start gap-2"
            data-active={preference === mode.id ? "true" : "false"}
          >
            <span
              className={`w-full h-10 rounded-md border border-border ${mode.swatch}`}
            />
            <span className="text-[13px] font-medium">{mode.label}</span>
          </button>
        ))}
      </div>
    </SettingsSection>
  );
}
