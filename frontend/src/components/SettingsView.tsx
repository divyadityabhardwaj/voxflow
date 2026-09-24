import { useEffect, useState, type ComponentType } from "react";
import { onRadioGroupKeyDown } from "../lib/radioGroup";
import GeneralSettings from "./settings/GeneralSettings";
import CleanupSettings from "./settings/CleanupSettings";
import DictionarySettings from "./settings/DictionarySettings";
import AppRulesSettings from "./settings/AppRulesSettings";
import AppearanceSettings from "./ui/AppearanceSettings";
import AdvancedSettings from "./settings/AdvancedSettings";
import AboutSettings from "./settings/AboutSettings";

const TABS: { id: string; label: string; Panel: ComponentType }[] = [
  { id: "general", label: "General", Panel: GeneralSettings },
  { id: "cleanup", label: "Clean-up (AI)", Panel: CleanupSettings },
  { id: "dictionary", label: "Dictionary", Panel: DictionarySettings },
  { id: "apps", label: "Apps", Panel: AppRulesSettings },
  { id: "appearance", label: "Appearance", Panel: AppearanceSettings },
  { id: "advanced", label: "Advanced", Panel: AdvancedSettings },
  { id: "about", label: "About", Panel: AboutSettings },
];

// Deep links from toasts and the backend name a permission, not a tab.
const ALIASES: Record<string, string> = {
  microphone: "general",
  accessibility: "general",
};

const resolve = (tab?: string) => {
  const id = (tab && ALIASES[tab]) || tab;
  return TABS.some((t) => t.id === id) ? id! : "general";
};

export default function SettingsView({ initialTab }: { initialTab?: string }) {
  const [tab, setTab] = useState(() => resolve(initialTab));

  useEffect(() => {
    if (initialTab) setTab(resolve(initialTab));
  }, [initialTab]);

  const { Panel } = TABS.find((t) => t.id === tab)!;

  return (
    <div className="settings-page">
      <header className="shrink-0 px-6 pt-6 pb-3">
        <h2 className="text-xl font-semibold text-text">Settings</h2>
      </header>

      <div
        role="tablist"
        aria-label="Settings sections"
        className="settings-tabs"
        onKeyDown={(e) =>
          onRadioGroupKeyDown(e, TABS.map((t) => t.id), tab, setTab)
        }
      >
        {TABS.map((t) => (
          <button
            key={t.id}
            type="button"
            role="tab"
            id={`settings-tab-${t.id}`}
            aria-selected={tab === t.id}
            aria-controls="settings-panel"
            tabIndex={tab === t.id ? 0 : -1}
            className="settings-tab"
            onClick={() => setTab(t.id)}
          >
            {t.label}
          </button>
        ))}
      </div>

      <div
        id="settings-panel"
        role="tabpanel"
        aria-labelledby={`settings-tab-${tab}`}
        className="settings-scroll px-6 py-5"
      >
        <div className="max-w-2xl mx-auto pb-8">
          <Panel key={tab} />
        </div>
      </div>
    </div>
  );
}
