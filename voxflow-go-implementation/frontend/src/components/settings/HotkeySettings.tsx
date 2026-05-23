import SettingsSection from "../ui/SettingsSection";

interface Config {
  hands_free_hotkey: string;
  push_to_talk_hotkey: string;
}

interface HotkeySettingsProps {
  config: Config;
  saving: string | null;
  success: string | null;
  openHotkeyModal: (action: "handsFree" | "ptt", currentHotkey: string) => void;
  formatHotkey: (hotkey: string) => string;
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
        className="w-full flex items-center justify-between gap-3 px-3 py-2.5 rounded-md border border-border bg-background hover:border-border-hover hover:bg-surface-hover transition-colors text-left"
      >
        <span className="font-mono text-sm text-text">
          {value || "Not set"}
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
  formatHotkey,
}: HotkeySettingsProps) {
  return (
    <SettingsSection
      title="Keyboard shortcuts"
      description="Configure how you start and stop dictation."
    >
      <div className="space-y-6">
        <HotkeyRow
          title="Push-to-talk"
          description="Hold to record; release to process."
          value={formatHotkey(config.push_to_talk_hotkey)}
          onEdit={() => openHotkeyModal("ptt", config.push_to_talk_hotkey)}
          saving={saving}
          success={success}
          saveKey="ptt"
        />
        <HotkeyRow
          title="Hands-free"
          description="Press once to start, again to stop."
          value={formatHotkey(config.hands_free_hotkey)}
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
