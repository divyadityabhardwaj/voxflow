import { useTheme } from "../../contexts/ThemeContext";
import SettingsSection from "./SettingsSection";

const THEMES = [
  {
    id: "light" as const,
    label: "Light",
    swatch: "bg-[#f8f9fa]",
  },
  {
    id: "dark" as const,
    label: "Dark",
    swatch: "bg-[#0c0d0e]",
  },
  {
    id: "midnight" as const,
    label: "Midnight",
    swatch: "bg-[#080b12]",
  },
];

export default function AppearanceSettings() {
  const { theme, setTheme } = useTheme();

  return (
    <SettingsSection
      title="Appearance"
      description="Theme is saved on this device."
    >
      <div className="grid grid-cols-3 gap-2">
        {THEMES.map((mode) => (
          <button
            key={mode.id}
            type="button"
            onClick={() => setTheme(mode.id)}
            className="segmented-item flex flex-col items-start gap-2"
            data-active={theme === mode.id ? "true" : "false"}
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
