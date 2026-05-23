import { useTheme } from "../../contexts/ThemeContext";
import SettingsSection from "./SettingsSection";

export default function AppearanceSettings() {
  const { theme, setTheme } = useTheme();

  return (
    <SettingsSection
      title="Appearance"
      description="Choose light or dark mode. Your preference is saved locally."
    >
      <div className="grid grid-cols-2 gap-2">
        {(["light", "dark"] as const).map((mode) => (
          <button
            key={mode}
            type="button"
            onClick={() => setTheme(mode)}
            className={`segmented-item flex flex-col items-start gap-2 ${
              theme === mode ? "" : ""
            }`}
            data-active={theme === mode ? "true" : "false"}
          >
            <span className="text-sm font-medium capitalize">{mode}</span>
            <span
              className={`w-full h-10 rounded-md border ${
                mode === "light"
                  ? "bg-[#f8f9fb] border-border"
                  : "bg-[#18181b] border-border"
              }`}
            />
          </button>
        ))}
      </div>
    </SettingsSection>
  );
}
