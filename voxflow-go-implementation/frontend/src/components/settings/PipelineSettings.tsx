import SettingsSection from "../ui/SettingsSection";

interface Config {
  refinement_mode: string;
  mute_system_audio: boolean;
}

interface PipelineSettingsProps {
  config: Config;
  saving: string | null;
  success: string | null;
  handleRefinementModeChange: (value: string) => Promise<void>;
  handleMuteSystemAudioChange: (value: boolean) => Promise<void>;
}

const MODES = [
  {
    id: "refine",
    name: "Refine",
    desc: "Whisper → LLM polish (Cloud/Local) → paste",
  },
  {
    id: "raw",
    name: "Raw",
    desc: "Whisper → paste as-is (100% on-device)",
  },
  {
    id: "copy-only",
    name: "Copy only",
    desc: "Whisper → clipboard (100% on-device)",
  },
] as const;

export default function PipelineSettings({
  config,
  saving,
  success,
  handleRefinementModeChange,
  handleMuteSystemAudioChange,
}: PipelineSettingsProps) {
  const active = config.refinement_mode || "refine";

  return (
    <SettingsSection
      title="Pipeline & audio"
      description="Control how recordings are processed and whether system audio is muted while recording."
    >
      <div className="space-y-6">
        <div>
          <span className="label">Pipeline mode</span>
          <div className="segmented grid-cols-1 sm:grid-cols-3">
            {MODES.map((mode) => (
              <button
                key={mode.id}
                type="button"
                onClick={() => handleRefinementModeChange(mode.id)}
                disabled={saving === "refinementMode"}
                className="segmented-item"
                data-active={active === mode.id ? "true" : "false"}
              >
                <span className="text-sm font-medium block">{mode.name}</span>
                <span className="text-xs text-secondary mt-1 block">
                  {mode.desc}
                </span>
              </button>
            ))}
          </div>
          {saving === "refinementMode" && (
            <p className="hint animate-pulse-soft">Saving…</p>
          )}
          {success === "refinementMode" && (
            <p className="hint text-[var(--success)]">Saved</p>
          )}
        </div>

        <div className="flex items-start justify-between gap-4 pt-5 border-t border-border">
          <div className="min-w-0">
            <p className="text-sm font-medium text-text">
              Mute system audio while recording
            </p>
            <p className="text-sm text-secondary mt-1 leading-relaxed">
              Temporarily mute speakers to reduce feedback, then restore volume
              when done.
            </p>
            {saving === "muteSystemAudio" && (
              <p className="hint animate-pulse-soft">Saving…</p>
            )}
            {success === "muteSystemAudio" && (
              <p className="hint text-[var(--success)]">Saved</p>
            )}
          </div>
          <button
            type="button"
            role="switch"
            aria-checked={config.mute_system_audio}
            disabled={saving === "muteSystemAudio"}
            className="toggle"
            data-on={config.mute_system_audio ? "true" : "false"}
            onClick={() =>
              handleMuteSystemAudioChange(!config.mute_system_audio)
            }
          >
            <span className="toggle-thumb" />
          </button>
        </div>
      </div>
    </SettingsSection>
  );
}
