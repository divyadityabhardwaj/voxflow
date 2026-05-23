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

export default function HotkeySettings({
  config,
  saving,
  success,
  openHotkeyModal,
  formatHotkey,
}: HotkeySettingsProps) {
  return (
    <section className="space-y-6">
      {/* Push to Talk */}
      <div className="card p-6 brutal-card">
        <h3 className="font-black text-xl uppercase tracking-tighter text-primary mb-4">
          Push-to-Talk Hotkey
        </h3>
        <p className="text-sm text-tertiary mb-4 font-bold">
          Hold this shortcut to record. Release to process.
        </p>
        <div className="flex gap-3 items-center">
          <div
            onClick={() =>
              openHotkeyModal("ptt", config.push_to_talk_hotkey)
            }
            className="flex-1 px-4 py-2.5 bg-secondary border-4 border-border rounded-[2rem]
                     text-text cursor-pointer hover:border-primary transition-colors
                     flex items-center justify-between group"
          >
            <span className="font-mono font-bold text-sm">
              {formatHotkey(config.push_to_talk_hotkey || "None")}
            </span>
            <span className="text-xs text-tertiary group-hover:text-primary transition-colors font-bold">
              Click to edit
            </span>
          </div>

          {saving === "ptt" && (
            <span className="flex items-center text-xs text-tertiary font-bold animate-pulse">
              Saving...
            </span>
          )}
          {success === "ptt" && (
            <span className="flex items-center text-xs text-green-500 font-bold">
              ✓ Saved
            </span>
          )}
        </div>
      </div>

      {/* Hands Free */}
      <div className="card p-6 brutal-card">
        <h3 className="font-black text-xl uppercase tracking-tighter text-primary mb-4">
          Hands-Free Hotkey
        </h3>
        <p className="text-sm text-tertiary mb-4 font-bold">
          Press once to start recording. Press again to stop.
        </p>
        <div className="flex gap-3 items-center">
          <div
            onClick={() =>
              openHotkeyModal("handsFree", config.hands_free_hotkey)
            }
            className="flex-1 px-4 py-2.5 bg-secondary border-4 border-border rounded-[2rem]
                     text-text cursor-pointer hover:border-primary transition-colors
                     flex items-center justify-between group"
          >
            <span className="font-mono font-bold text-sm">
              {formatHotkey(config.hands_free_hotkey || "None")}
            </span>
            <span className="text-xs text-tertiary group-hover:text-primary transition-colors font-bold">
              Click to edit
            </span>
          </div>

          {saving === "handsFree" && (
            <span className="flex items-center text-xs text-tertiary font-bold animate-pulse">
              Saving...
            </span>
          )}
          {success === "handsFree" && (
            <span className="flex items-center text-xs text-green-500 font-bold">
              ✓ Saved
            </span>
          )}
        </div>
      </div>
    </section>
  );
}
