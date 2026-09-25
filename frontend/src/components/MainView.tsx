import { useState, useEffect, useCallback } from "react";
import { EventsOn } from "../../wailsjs/runtime/runtime";
import { GetHistory, OpenHistoryWindow, ToggleRecording } from "../../wailsjs/go/main/App";
import { Events } from "../constants/events";
import { useRecordingState, isBusy } from "../hooks/useRecordingState";
import {
  deliveryLabel,
  formatClock,
  pasteLabel,
  useAudioLevel,
  usePasteTarget,
  useRecordingSeconds,
  useShortcuts,
  useTextActions,
  type ProcessingResult,
} from "../hooks/useDictation";
import Keycap from "./dictation/Keycap";
import LevelBars from "./dictation/LevelBars";
import VersionToggle, { type Version } from "./dictation/VersionToggle";
import { relativeTime } from "./dictation/time";
import SetupChecklist from "./home/SetupChecklist";
import { computeStats, countWords } from "./home/stats";

interface Transcript {
  id: number;
  timestamp: string;
  app_name: string;
  raw_text: string;
  polished_text: string;
  words_per_second?: number;
}

// ponytail: stats read the latest 500 dictations; add a GetStats query if a week holds more.
const STATS_LIMIT = 500;
const RECENT_COUNT = 5;

const MIC_PATH =
  "M12 14c1.66 0 3-1.34 3-3V5c0-1.66-1.34-3-3-3S9 3.34 9 5v6c0 1.66 1.34 3 3 3zm5-3c0 2.76-2.24 5-5 5s-5-2.24-5-5H5c0 3.53 2.61 6.43 6 6.92V21h2v-3.08c3.39-.49 6-3.39 6-6.92h-2z";
const COPY_PATH =
  "M8 5H6a2 2 0 00-2 2v12a2 2 0 002 2h10a2 2 0 002-2v-1M8 5a2 2 0 002 2h2a2 2 0 002-2M8 5a2 2 0 012-2h2a2 2 0 012 2m0 0h2a2 2 0 012 2v3";
const PASTE_PATH =
  "M8 4H6a2 2 0 00-2 2v12a2 2 0 002 2h12a2 2 0 002-2V6a2 2 0 00-2-2h-2m-4-1v8m0 0l3-3m-3 3L9 8m-5 5h2.586a1 1 0 01.707.293l2.414 2.414a1 1 0 00.707.293h3.172a1 1 0 00.707-.293l2.414-2.414a1 1 0 01.707-.293H20";

const StrokeIcon = ({ d, className = "w-3.5 h-3.5" }: { d: string; className?: string }) => (
  <svg className={className} fill="none" stroke="currentColor" viewBox="0 0 24 24" strokeWidth={2} aria-hidden="true">
    <path strokeLinecap="round" strokeLinejoin="round" d={d} />
  </svg>
);

const ICON_BTN =
  "p-1.5 rounded-md text-secondary hover:text-text hover:bg-secondary transition-colors";

const TITLES = {
  Idle: "Ready to dictate",
  Recording: "Listening…",
  Processing: "Turning speech into text…",
  Refining: "Cleaning up your text…",
};

export default function MainView() {
  const status = useRecordingState();
  const busy = isBusy(status);
  const recording = status === "Recording";
  const level = useAudioLevel(recording);
  const seconds = useRecordingSeconds(status);
  const shortcuts = useShortcuts();
  const pasteApp = usePasteTarget();
  const { copy, paste } = useTextActions(pasteApp);

  const [history, setHistory] = useState<Transcript[]>([]);
  const [result, setResult] = useState<ProcessingResult | null>(null);
  const [resultVersion, setResultVersion] = useState<Version>("cleaned");
  const [partialText, setPartialText] = useState("");
  const [error, setError] = useState<string | null>(null);
  const [expandedId, setExpandedId] = useState<number | null>(null);

  const loadHistory = useCallback(() => {
    GetHistory(STATS_LIMIT)
      .then((items) => setHistory((items as unknown as Transcript[]) || []))
      .catch((err) => console.error("Failed to load history:", err));
  }, []);

  useEffect(() => {
    loadHistory();
    const unsubs = [
      // Reset here rather than in an effect on status: the Error event for a
      // failed start arrives right after "Recording" and must survive.
      EventsOn(Events.StateChanged, (s: string) => {
        if (s === "Recording") {
          setError(null);
          setPartialText("");
        }
      }),
      EventsOn(Events.ProcessingComplete, (r: ProcessingResult) => {
        setResult(r);
        setResultVersion("cleaned");
        setPartialText("");
        loadHistory();
      }),
      EventsOn(Events.Error, (err: string) => setError(String(err))),
      EventsOn(Events.PartialTranscript, (d: { text: string }) => {
        if (d?.text) setPartialText(d.text);
      }),
    ];
    return () => unsubs.forEach((u) => u());
  }, [loadHistory]);

  const toggle = async () => {
    try {
      await ToggleRecording();
    } catch (err) {
      setError(String(err));
    }
  };

  const stats = computeStats(history);
  const recents = history.slice(0, RECENT_COUNT);
  const resultText = result ? (resultVersion === "raw" ? result.raw : result.polished) : "";
  const resultWords = result ? countWords(result.polished) : 0;
  const spokenSec = result?.details?.audio;

  const micLabel = recording ? "Stop" : busy ? TITLES[status] : "Try it here";

  return (
    <div className="h-full min-h-0 overflow-y-auto animate-fade-in">
      <div className="max-w-xl mx-auto px-6 py-8 flex flex-col items-center gap-6">
        <SetupChecklist />

        <section className="w-full text-center flex flex-col items-center">
          <h1 aria-live="polite" className="text-xl font-semibold tracking-tight text-text">
            {TITLES[status]}
          </h1>

          {status === "Idle" && shortcuts && (
            <p className="flex flex-wrap items-center justify-center gap-x-1.5 gap-y-1 text-[13px] text-secondary mt-3">
              Hold <Keycap>{shortcuts.hold}</Keycap> to talk
              <span aria-hidden="true" className="text-tertiary px-0.5">·</span>
              <Keycap>{shortcuts.toggle}</Keycap> to start/stop
              <span aria-hidden="true" className="text-tertiary px-0.5">·</span>
              <Keycap>Esc</Keycap> to cancel
            </p>
          )}
          {status === "Idle" && shortcuts?.edit && (
            <p className="flex flex-wrap items-center justify-center gap-x-1.5 gap-y-1 text-[13px] text-secondary mt-1.5">
              Select text anywhere and press <Keycap>{shortcuts.edit}</Keycap> to rewrite it by voice
            </p>
          )}
          {recording && (
            <p className="flex flex-wrap items-center justify-center gap-1.5 text-[13px] text-secondary mt-3">
              {shortcuts ? (
                <>
                  Release the keys, or press <Keycap>{shortcuts.toggle}</Keycap> again to finish ·
                  <Keycap>Esc</Keycap> to cancel
                </>
              ) : (
                "Press Esc to cancel"
              )}
            </p>
          )}

          <button
            type="button"
            onClick={toggle}
            disabled={busy}
            aria-label={recording ? "Stop dictation" : "Try it here — start dictating"}
            className={`mt-6 size-12 rounded-full flex items-center justify-center shadow-soft-md transition-transform active:scale-95 disabled:cursor-not-allowed ${
              recording
                ? "bg-recording text-white"
                : busy
                  ? "bg-processing text-[#1c1917]"
                  : "bg-primary text-[var(--primary-foreground)] hover:scale-105"
            }`}
          >
            {recording ? (
              <span className="size-3.5 rounded-[3px] bg-white" />
            ) : busy ? (
              <span className="size-4 rounded-full border-2 border-current border-t-transparent animate-spin" />
            ) : (
              <svg className="w-5 h-5" fill="currentColor" viewBox="0 0 24 24" aria-hidden="true">
                <path d={MIC_PATH} />
              </svg>
            )}
          </button>
          <div className="mt-2 h-5 flex items-center gap-2 text-[13px] text-secondary">
            {recording && <LevelBars level={level} maxHeight={14} color="var(--recording)" />}
            <span className={recording ? "tabular-nums" : ""}>
              {recording ? `${micLabel} · ${formatClock(seconds)}` : status === "Idle" ? micLabel : ""}
            </span>
          </div>

          {(recording || busy) && partialText && (
            <p className="mt-4 w-full text-left text-sm text-secondary leading-relaxed whitespace-pre-wrap max-h-24 overflow-y-auto card p-3">
              {partialText}
            </p>
          )}
        </section>

        {error && (
          <div role="alert" className="w-full p-3 rounded-lg border border-danger/30 bg-danger/10 text-[13px] text-text">
            {error}
          </div>
        )}

        {result && !recording && !busy && (
          <section className="w-full card p-4 animate-fade-in" aria-label="Last dictation">
            <div className="flex items-center gap-2 mb-2">
              <h2 className="text-[13px] font-semibold text-text">
                {result.method === "none" ? "Here's what you said" : deliveryLabel(result)}
              </h2>
              <span className="text-xs text-tertiary tabular-nums">
                {resultWords} {resultWords === 1 ? "word" : "words"}
                {spokenSec ? ` · ${spokenSec.toFixed(1)} s` : ""}
              </span>
              <button
                type="button"
                className={`${ICON_BTN} ml-auto`}
                onClick={() => setResult(null)}
                aria-label="Dismiss"
              >
                <StrokeIcon d="M6 18L18 6M6 6l12 12" />
              </button>
            </div>
            {result.raw && result.raw !== result.polished && (
              <div className="mb-2">
                <VersionToggle value={resultVersion} onChange={setResultVersion} />
              </div>
            )}
            <p className="text-sm text-text whitespace-pre-wrap leading-relaxed select-text">{resultText}</p>
            <div className="flex justify-end gap-2 mt-3">
              <button type="button" className="btn-secondary !py-1.5 !px-3 !text-xs" onClick={() => copy(resultText)}>
                Copy
              </button>
              <button type="button" className="btn-primary !py-1.5 !px-3 !text-xs" onClick={() => paste(resultText)}>
                {pasteLabel(pasteApp)}
              </button>
            </div>
          </section>
        )}

        {stats.words > 0 && (
          <p className="text-[13px] text-secondary text-center">
            This week: <span className="text-text font-medium">{stats.words.toLocaleString()} words</span>
            {stats.minutesSaved > 0 && <> · ~{stats.minutesSaved} min saved</>}
            {stats.streakDays > 1 && <> · {stats.streakDays}-day streak</>}
          </p>
        )}

        <section className="w-full" aria-labelledby="recent-heading">
          <div className="flex items-center justify-between mb-2 px-0.5">
            <h2 id="recent-heading" className="text-[13px] font-semibold text-text">
              Recent
            </h2>
            {recents.length > 0 && (
              <button type="button" className="text-xs text-secondary hover:text-text" onClick={() => OpenHistoryWindow()}>
                View all history →
              </button>
            )}
          </div>
          {recents.length === 0 ? (
            <div className="card p-6 text-center text-[13px] text-secondary">
              Your dictations will show up here.
              {shortcuts && (
                <span className="block mt-1">
                  Try it: press <Keycap>{shortcuts.toggle}</Keycap>, say something, press it again.
                </span>
              )}
            </div>
          ) : (
            <ul className="card overflow-hidden divide-y divide-border/50">
              {recents.map((item) => {
                const text = item.polished_text || item.raw_text;
                const expanded = expandedId === item.id;
                return (
                  <li key={item.id} className="group px-4 py-2.5 hover:bg-surface-hover/50 focus-within:bg-surface-hover/50 transition-colors">
                    <div className="flex items-start gap-3">
                      <div className="flex-1 min-w-0">
                        <p className={`text-[13px] text-text leading-snug ${expanded ? "whitespace-pre-wrap select-text" : "truncate"}`}>
                          {text}
                        </p>
                        {expanded && item.raw_text && item.raw_text !== item.polished_text && (
                          <p className="mt-1.5 text-xs text-secondary whitespace-pre-wrap select-text">
                            <span className="font-medium">As you said it: </span>
                            {item.raw_text}
                          </p>
                        )}
                        <span className="text-xs text-tertiary block mt-0.5">
                          {[item.app_name, relativeTime(item.timestamp)].filter(Boolean).join(" · ")}
                        </span>
                      </div>
                      <div className="flex items-center gap-0.5 shrink-0 opacity-0 group-hover:opacity-100 group-focus-within:opacity-100 transition-opacity">
                        <button type="button" onClick={() => copy(text)} title="Copy" aria-label="Copy" className={ICON_BTN}>
                          <StrokeIcon d={COPY_PATH} />
                        </button>
                        <button
                          type="button"
                          onClick={() => paste(text)}
                          title={pasteLabel(pasteApp)}
                          aria-label={pasteLabel(pasteApp)}
                          className={ICON_BTN}
                        >
                          <StrokeIcon d={PASTE_PATH} />
                        </button>
                        <button
                          type="button"
                          onClick={() => setExpandedId(expanded ? null : item.id)}
                          aria-expanded={expanded}
                          title={expanded ? "Show less" : "Show all"}
                          aria-label={expanded ? "Show less" : "Show all"}
                          className={ICON_BTN}
                        >
                          <StrokeIcon d={expanded ? "M5 15l7-7 7 7" : "M19 9l-7 7-7-7"} />
                        </button>
                      </div>
                    </div>
                  </li>
                );
              })}
            </ul>
          )}
        </section>
      </div>
    </div>
  );
}
