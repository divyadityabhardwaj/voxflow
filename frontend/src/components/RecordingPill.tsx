import { CancelRecording, ToggleRecording } from "../../wailsjs/go/main/App";
import { useRecordingState } from "../hooks/useRecordingState";
import { formatClock, useAudioLevel, useRecordingSeconds } from "../hooks/useDictation";
import LevelBars from "./dictation/LevelBars";

// Shown over the full window while a dictation records.
export default function RecordingPill() {
  const status = useRecordingState();
  const recording = status === "Recording";
  const level = useAudioLevel(recording);
  const seconds = useRecordingSeconds(status);

  if (!recording) return null;

  return (
    <div className="fixed top-4 left-1/2 -translate-x-1/2 z-50" role="status">
      <div className="flex items-center gap-3 pl-4 pr-2 py-1.5 bg-surface border border-border rounded-full shadow-soft-md">
        <LevelBars level={level} maxHeight={16} color="var(--recording)" />
        <span className="text-[13px] font-semibold text-text tabular-nums">
          {formatClock(seconds)}
        </span>
        <span className="text-xs text-tertiary">Esc to cancel</span>
        <button
          type="button"
          onClick={() => CancelRecording()}
          className="btn-ghost !py-1 !px-2 !text-xs"
          aria-label="Cancel dictation"
        >
          Cancel
        </button>
        <button
          type="button"
          onClick={() => ToggleRecording()}
          className="size-7 rounded-full bg-recording text-white flex items-center justify-center hover:opacity-90"
          aria-label="Stop dictation"
          title="Stop dictation"
        >
          <span className="size-2.5 rounded-[2px] bg-white" />
        </button>
      </div>
    </div>
  );
}
