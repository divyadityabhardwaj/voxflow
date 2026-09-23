import SettingsSection from "../ui/SettingsSection";
import { formatShortcut } from "../../lib/shortcut";

interface Config {
  hands_free_hotkey: string;
  push_to_talk_hotkey: string;
}

interface HotkeySettingsProps {
  config: Config;
  saving: string | null;
  success: string | null;
  openHotkeyModal: (action: "handsFree" | "ptt", currentHotkey: string) => void;
}

function HotkeyRow({
  title,
  description,
  value,
  onEdit,
  saving,
  success,
  saveKey,
}: {
  title: string;
  description: string;
  value: string;
  onEdit: () => void;
  saving: string | null;
  success: string | null;
  saveKey: string;
}) {
  return (
    <div>
      <p className="text-sm font-medium text-text">{title}</p>
      <p className="text-sm text-secondary mt-0.5 mb-3">{description}</p>
      <button
        type="button"
        onClick={onEdit}
        aria-label={`${title}: ${value ? formatShortcut(value) : "not set"}. Change`}
        className="w-full flex items-center justify-between gap-3 px-3 py-2.5 rounded-md border border-border bg-surface hover:border-border-hover hover:bg-surface-hover transition-colors text-left"
      >
        <span className="text-sm font-medium tracking-wide text-text">
          {value ? formatShortcut(value) : "Not set"}
        </span>
        <span className="text-xs text-tertiary shrink-0">Change</span>
      </button>
      {saving === saveKey && (
        <p className="hint animate-pulse-soft">Saving…</p>
      )}
      {success === saveKey && (
        <p className="hint text-[var(--success)]">Saved</p>
      )}
    </div>
  );
}

export default function HotkeySettings({
  config,
  saving,
  success,
  openHotkeyModal,
}: HotkeySettingsProps) {
  return (
    <SettingsSection
      title="Keyboard shortcuts"
      description="Configure how you start and stop dictation."
    >
      <div className="space-y-6">
        <HotkeyRow
          title="Hold to talk"
          description="Hold while you speak; let go to paste."
          value={config.push_to_talk_hotkey}
          onEdit={() => openHotkeyModal("ptt", config.push_to_talk_hotkey)}
          saving={saving}
          success={success}
          saveKey="ptt"
        />
        <HotkeyRow
          title="Start/stop"
          description="Press once to start, again to finish."
          value={config.hands_free_hotkey}
          onEdit={() =>
            openHotkeyModal("handsFree", config.hands_free_hotkey)
          }
          saving={saving}
          success={success}
          saveKey="handsFree"
        />
      </div>
    </SettingsSection>
  );
}
