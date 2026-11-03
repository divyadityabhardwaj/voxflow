import { CancelRecording, ToggleRecording } from "../../wailsjs/go/main/App";
import { useRecordingState, isBusy } from "../hooks/useRecordingState";
import { formatClock, useAudioLevel, useRecordingSeconds } from "../hooks/useDictation";
import LevelBars from "./dictation/LevelBars";

// Keep recording controls available when navigating away from Dictate.
export default function RecordingPill() {
  const status = useRecordingState();
  const recording = status === "Recording";
  const level = useAudioLevel(recording);
  const seconds = useRecordingSeconds(status);

  if (!recording && !isBusy(status)) return null;

  return (
    <div className="recording-overlay" role="status">
      <div className="flex items-center gap-3 pl-4 pr-2 py-1.5 bg-surface border border-border rounded-full shadow-soft-md">
        {recording ? <LevelBars level={level} maxHeight={16} color="var(--recording)" /> : <span className="size-3 rounded-full border-2 border-current border-t-transparent animate-spin text-secondary" aria-hidden="true" />}
        <span className="text-[13px] font-semibold text-text tabular-nums">
          {recording ? formatClock(seconds) : status === "Refining" ? "Cleaning up…" : "Transcribing…"}
        </span>
        <button
          type="button"
          onClick={() => CancelRecording()}
          className="btn-ghost !py-1 !px-2 !text-xs"
          aria-label="Cancel dictation"
          title="Cancel (Esc)"
        >
          Cancel
        </button>
        {recording && <button
          type="button"
          onClick={() => ToggleRecording()}
          className="size-7 rounded-full bg-recording text-[var(--danger-foreground)] flex items-center justify-center hover:opacity-90"
          aria-label="Stop dictation"
          title="Stop dictation"
        >
          <span className="size-2.5 rounded-[2px] bg-current" />
        </button>}
      </div>
    </div>
  );
}
