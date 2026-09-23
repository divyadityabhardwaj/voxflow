import { useTheme, type ThemePreference } from "../../contexts/ThemeContext";
import { onRadioGroupKeyDown } from "../../lib/radioGroup";
import SettingsSection from "./SettingsSection";

const THEMES: { id: ThemePreference; label: string; swatch: string }[] = [
  {
    id: "system",
    label: "System",
    swatch: "bg-[linear-gradient(135deg,#f8f9fa_50%,#0c0d0e_50%)]",
  },
  {
    id: "light",
    label: "Light",
    swatch: "bg-[#f8f9fa]",
  },
  {
    id: "dark",
    label: "Dark",
    swatch: "bg-[#0c0d0e]",
  },
  {
    id: "midnight",
    label: "Midnight",
    swatch: "bg-[#080b12]",
  },
];

export default function AppearanceSettings() {
  const { preference, setTheme } = useTheme();

  return (
    <SettingsSection
      title="Appearance"
      description="Theme is saved on this device."
    >
      <div
        role="radiogroup"
        aria-label="Theme"
        className="grid grid-cols-2 sm:grid-cols-4 gap-2"
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
            <span className="text-sm font-medium">{mode.label}</span>
            <span
              className={`w-full h-9 rounded-md border border-border ${mode.swatch}`}
            />
          </button>
        ))}
      </div>
    </SettingsSection>
  );
}
