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

export default function PipelineSettings({
  config,
  saving,
  success,
  handleRefinementModeChange,
  handleMuteSystemAudioChange,
}: PipelineSettingsProps) {
  return (
    <section className="card p-6 brutal-card">
      <h3 className="font-black text-xl uppercase tracking-tighter text-primary mb-4">
        Pipeline & Audio Settings
      </h3>
      <p className="text-sm text-tertiary mb-6 font-bold">
        Configure VoxFlow's transcription behavior, text injection pipeline, and system interaction.
      </p>

      <div className="space-y-6">
        {/* Refinement Mode Toggle */}
        <div>
          <label className="block text-xs font-black uppercase tracking-tighter text-tertiary mb-2">
            Transcription Pipeline Mode
          </label>
          <div className="grid grid-cols-3 gap-3">
            {[
              {
                id: "refine",
                name: "Refine (Default)",
                desc: "Whisper → LLM Polish → Paste",
              },
              {
                id: "raw",
                name: "Raw Transcription",
                desc: "Whisper → Direct Paste",
              },
              {
                id: "copy-only",
                name: "Copy Only",
                desc: "Whisper → Clipboard Only",
              },
            ].map((mode) => (
              <button
                key={mode.id}
                onClick={() => handleRefinementModeChange(mode.id)}
                disabled={saving === "refinementMode"}
                className={`p-4 rounded-xl border-4 text-left transition-all focus:outline-none flex flex-col justify-between ${
                  (config.refinement_mode || "refine") === mode.id
                    ? "border-primary bg-primary/5 text-primary shadow-[4px_4px_0px_var(--primary)]"
                    : "border-border bg-secondary text-text hover:border-tertiary"
                }`}
              >
                <span className="font-bold text-sm uppercase tracking-tight">
                  {mode.name}
                </span>
                <span className="text-[10px] text-tertiary mt-2 font-bold leading-tight">
                  {mode.desc}
                </span>
              </button>
            ))}
          </div>
          {saving === "refinementMode" && (
            <span className="text-xs text-tertiary mt-2 block font-bold animate-pulse">
              Saving...
            </span>
          )}
          {success === "refinementMode" && (
            <span className="text-xs text-green-500 mt-2 block font-bold">
              ✓ Saved
            </span>
          )}
        </div>

        {/* Mute System Audio Switch */}
        <div className="flex items-start justify-between pt-6 border-t-4 border-border">
          <div className="flex-1 pr-4">
            <label className="block text-sm font-bold text-text mb-1">
              Mute System Audio During Recording
            </label>
            <p className="text-xs text-tertiary font-bold leading-normal">
              Automatically mute speakers/headphones while recording starts to prevent microphone feedback or background sound capture, then restore previous volume upon completion.
            </p>
            {saving === "muteSystemAudio" && (
              <span className="text-xs text-tertiary mt-2 block font-bold animate-pulse">
                Saving...
              </span>
            )}
            {success === "muteSystemAudio" && (
              <span className="text-xs text-green-500 mt-2 block font-bold">
                ✓ Saved
              </span>
            )}
          </div>
          <button
            onClick={() =>
              handleMuteSystemAudioChange(!config.mute_system_audio)
            }
            disabled={saving === "muteSystemAudio"}
            className={`w-12 h-6 flex items-center rounded-full p-1 cursor-pointer transition-colors duration-200 border-2 border-border focus:outline-none ${
              config.mute_system_audio ? "bg-primary" : "bg-secondary"
            }`}
          >
            <div
              className={`bg-white w-4 h-4 rounded-full shadow-md transform transition-transform duration-200 ${
                config.mute_system_audio ? "translate-x-6" : "translate-x-0"
              }`}
            />
          </button>
        </div>
      </div>
    </section>
  );
}
