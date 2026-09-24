import { useEffect, useRef, useState } from "react";
import { EventsOn } from "../../../wailsjs/runtime/runtime";
import { ToggleRecording } from "../../../wailsjs/go/main/App";
import { Events } from "../../constants/events";
import { useRecordingState, isBusy } from "../../hooks/useRecordingState";
import {
  formatClock,
  useAudioLevel,
  useRecordingSeconds,
  type ProcessingResult,
  type Shortcuts,
} from "../../hooks/useDictation";
import Keycap from "../dictation/Keycap";
import LevelBars from "../dictation/LevelBars";
import { countWords } from "../home/stats";

const TYPING_WPM = 40;
const NO_SPEECH = ["no speech", "no audio", "microphone access is off"];

type Outcome =
  | { kind: "success"; words: number; seconds: number }
  | { kind: "missed" };

interface Props {
  shortcuts: Shortcuts | null;
  modelReady: boolean;
  downloadPercent: number;
}

export default function TryIt({ shortcuts, modelReady, downloadPercent }: Props) {
  const status = useRecordingState();
  const recording = status === "Recording";
  const busy = isBusy(status);
  const level = useAudioLevel(recording);
  const seconds = useRecordingSeconds(status);
  const [text, setText] = useState("");
  const [outcome, setOutcome] = useState<Outcome | null>(null);
  const boxRef = useRef<HTMLTextAreaElement>(null);

  useEffect(() => {
    boxRef.current?.focus();
  }, [modelReady]);

  useEffect(() => {
    const unsubs = [
      EventsOn(Events.StateChanged, (s: string) => {
        if (s === "Recording") setOutcome(null);
      }),
      EventsOn(Events.ProcessingComplete, (r: ProcessingResult) => {
        setText((prev) => (prev.trim() ? `${prev.trimEnd()} ${r.polished}` : r.polished));
        const words = countWords(r.polished);
        const spoken = r.details?.audio ?? 0;
        setOutcome({ kind: "success", words, seconds: Math.max(1, Math.round(spoken)) });
      }),
      EventsOn(Events.Toast, (d: { message?: string }) => {
        const msg = (d?.message || "").toLowerCase();
        if (NO_SPEECH.some((k) => msg.includes(k))) setOutcome({ kind: "missed" });
      }),
    ];
    return () => unsubs.forEach((u) => u());
  }, []);

  const tryAgain = () => {
    setOutcome(null);
    boxRef.current?.focus();
  };

  if (!modelReady) {
    return (
      <div className="card p-4 text-[13px] text-secondary" role="status">
        Getting the speech engine ready{downloadPercent > 0 ? ` — ${downloadPercent}%` : "…"}. You can try it as soon as it's done.
      </div>
    );
  }

  const speedup =
    outcome?.kind === "success" ? outcome.words / (outcome.seconds / 60) / TYPING_WPM : 0;

  return (
    <div className="space-y-3">
      <div className="relative">
        <textarea
          ref={boxRef}
          value={text}
          onChange={(e) => setText(e.target.value)}
          rows={4}
          aria-label="Try dictating here"
          placeholder={`Hold ${shortcuts?.hold ?? "your dictation key"} and say: “Hi, this is my first dictation with VoxFlow.”`}
          className="input resize-none pr-12"
        />
        <button
          type="button"
          onClick={() => ToggleRecording()}
          disabled={busy}
          aria-label={recording ? "Stop dictation" : "Start dictating"}
          title={recording ? "Stop" : "Or click to start"}
          className={`absolute right-2 bottom-3 size-8 rounded-full flex items-center justify-center disabled:opacity-60 ${
            recording ? "bg-recording text-white" : "bg-secondary text-text hover:bg-surface-hover"
          }`}
        >
          {recording ? (
            <span className="size-2.5 rounded-[2px] bg-white" />
          ) : (
            <svg className="w-4 h-4" fill="currentColor" viewBox="0 0 24 24" aria-hidden="true">
              <path d="M12 14c1.66 0 3-1.34 3-3V5c0-1.66-1.34-3-3-3S9 3.34 9 5v6c0 1.66 1.34 3 3 3zm5-3c0 2.76-2.24 5-5 5s-5-2.24-5-5H5c0 3.53 2.61 6.43 6 6.92V21h2v-3.08c3.39-.49 6-3.39 6-6.92h-2z" />
            </svg>
          )}
        </button>
      </div>

      <div className="min-h-[2.5rem] text-[13px]" role="status" aria-live="polite">
        {recording && (
          <div className="flex items-center gap-2 text-secondary">
            <LevelBars level={level} maxHeight={16} color="var(--recording)" />
            <span className="tabular-nums text-text">{formatClock(seconds)}</span>
            <span>Listening… let go (or press again) to finish · Esc to cancel</span>
          </div>
        )}
        {busy && <p className="text-secondary">{status === "Refining" ? "Cleaning up…" : "Turning speech into text…"}</p>}
        {!recording && !busy && outcome?.kind === "success" && (
          <p className="text-text">
            <span className="text-[var(--success)] font-semibold" aria-hidden="true">✓ </span>
            Nice — {outcome.words} {outcome.words === 1 ? "word" : "words"} in {outcome.seconds}{" "}
            {outcome.seconds === 1 ? "second" : "seconds"}.
            {speedup >= 1.5 && ` That's about ${Math.round(speedup)}× faster than typing.`}{" "}
            <button
              type="button"
              className="underline text-primary"
              onClick={() => {
                setText("");
                tryAgain();
              }}
            >
              Try again
            </button>
          </p>
        )}
        {!recording && !busy && outcome?.kind === "missed" && (
          <p className="text-text">
            Didn't catch that. Is the right microphone selected? You can pick one in Settings later.{" "}
            <button type="button" className="underline text-primary" onClick={tryAgain}>
              Try again
            </button>
          </p>
        )}
        {!recording && !busy && !outcome && shortcuts && (
          <p className="text-secondary flex flex-wrap items-center gap-1.5">
            Hold <Keycap>{shortcuts.hold}</Keycap> while you talk, or press <Keycap>{shortcuts.toggle}</Keycap> to
            start and stop.
          </p>
        )}
      </div>
    </div>
  );
}
