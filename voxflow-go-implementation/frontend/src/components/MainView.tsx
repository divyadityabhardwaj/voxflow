import { useState, useEffect, useRef } from "react";
import { EventsOn } from "../../wailsjs/runtime/runtime";
import {
  ToggleRecording,
  GetStatus,
  GetConfig,
  GetHistory,
  CopyToClipboard,
  InjectText,
} from "../../wailsjs/go/main/App";
import { Events } from "../constants/events";
import { useToast } from "../contexts/ToastContext";

type Status = "Idle" | "Recording" | "Processing" | "Refining";

interface Transcript {
  id: number;
  timestamp: string;
  raw_text: string;
  polished_text: string;
}

const formatElapsed = (ms: number) => {
  const seconds = ms / 1000;
  return seconds < 1 ? `${ms}ms` : `${seconds.toFixed(1)}s`;
};

const formatRecentDate = (timestamp: string) => {
  try {
    const date = new Date(timestamp);
    const now = new Date();
    const isToday = date.toDateString() === now.toDateString();
    if (isToday) {
      return date.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' });
    }
    return date.toLocaleDateString([], { month: 'short', day: 'numeric' }) + ' ' + date.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' });
  } catch {
    return "";
  }
};

export default function MainView() {
  const [status, setStatus] = useState<Status>("Idle");
  const [handsFreeHotkey, setHandsFreeHotkey] = useState<string>("");
  const [pttHotkey, setPttHotkey] = useState<string>("");
  const [lastTranscription, setLastTranscription] = useState<string | null>(
    null,
  );
  const [usedRawNoPolish, setUsedRawNoPolish] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [partialText, setPartialText] = useState<string>("");
  const [elapsedMs, setElapsedMs] = useState<number | null>(null);
  const [wordsPerMinute, setWordsPerMinute] = useState<number | null>(null);
  const [recents, setRecents] = useState<Transcript[]>([]);
  const partialRef = useRef<HTMLDivElement>(null);
  const { showToast } = useToast();

  useEffect(() => {
    GetStatus().then((s) => setStatus(s as Status));
    GetConfig().then((cfg) => {
      if (cfg) {
        setHandsFreeHotkey(cfg.hands_free_hotkey || cfg.hotkey || "");
        setPttHotkey(cfg.push_to_talk_hotkey || "");
      }
    });

    const loadRecents = () => {
      GetHistory(4)
        .then((items) => {
          setRecents(items || []);
        })
        .catch((err) => {
          console.error("Failed to load recent recordings:", err);
        });
    };

    loadRecents();

    const unsubState = EventsOn(Events.StateChanged, (newStatus: string) => {
      setStatus(newStatus as Status);
      if (newStatus === "Recording") {
        setError(null);
        setLastTranscription(null);
        setUsedRawNoPolish(false);
        setPartialText("");
        setElapsedMs(null);
        setWordsPerMinute(null);
      } else if (newStatus === "Idle") {
        loadRecents();
      }
    });

    const unsubComplete = EventsOn(
      Events.ProcessingComplete,
      (result: {
        polished: string;
        elapsed: number;
        used_raw?: boolean;
        words_per_second?: number;
      }) => {
        setLastTranscription(result.polished);
        setUsedRawNoPolish(Boolean(result.used_raw));
        setPartialText("");
        setElapsedMs(result.elapsed);
        if (result.words_per_second && result.words_per_second > 0) {
          setWordsPerMinute(Math.round(result.words_per_second * 60));
        }
        loadRecents();
      },
    );

    const unsubError = EventsOn(Events.Error, (err: string) => {
      setError(err);
    });

    const unsubPartial = EventsOn(
      Events.PartialTranscript,
      (data: { text: string }) => {
        if (data?.text) {
          setPartialText(data.text);
        }
      },
    );

    return () => {
      unsubState();
      unsubComplete();
      unsubError();
      unsubPartial();
    };
  }, []);

  useEffect(() => {
    if (partialRef.current) {
      partialRef.current.scrollTop = partialRef.current.scrollHeight;
    }
  }, [partialText]);

  const handleToggle = async () => {
    try {
      await ToggleRecording();
    } catch (err) {
      setError(String(err));
    }
  };



  const handleCopy = async (text: string) => {
    try {
      await CopyToClipboard(text);
      showToast("Copied to clipboard", "success");
    } catch (err) {
      console.error("Failed to copy:", err);
      showToast("Failed to copy text", "error");
    }
  };

  const handleInject = async (text: string) => {
    try {
      await InjectText(text);
      showToast("Text injected", "success");
    } catch (err) {
      console.error("Failed to inject:", err);
      showToast("Failed to inject text", "error");
    }
  };

  return (
    <div className="flex flex-col items-center justify-center h-full min-h-0 p-8 overflow-y-auto animate-fade-in">
      <div className="text-center mb-8">
        <h1 className="text-2xl font-semibold text-text mb-2">
          {status === "Idle" && "Capture a quick thought"}
          {status === "Recording" && "Listening…"}
          {status === "Processing" && "Processing…"}
          {status === "Refining" && "Refining…"}
        </h1>
        <p className="text-secondary text-sm">
          {status === "Idle" &&
            (handsFreeHotkey
              ? `Press ${handsFreeHotkey} to start recording, or hold ${pttHotkey || "PTT key"} to speak`
              : "Press the button or use your hotkey to start recording")}
          {status === "Recording" &&
            "Speak naturally, then press again or release key to stop"}
          {status === "Processing" &&
            (partialText
              ? "Transcribing your recording…"
              : "Transcribing your recording")}
          {status === "Refining" && "Polishing transcription with AI…"}
        </p>
      </div>

      <div className="w-full max-w-xl mb-10">
        <div
          className={`
            card-elevated p-5 flex items-center gap-4 transition-all duration-300
            ${status === "Recording" ? "border-recording" : ""}
            ${status === "Processing" ? "border-processing" : ""}
          `}
        >
          <div className="flex-1 min-w-0">
            {status === "Processing" && partialText ? (
              <div
                ref={partialRef}
                className="max-h-24 overflow-y-auto"
              >
                <p className="text-text text-sm leading-relaxed opacity-70 whitespace-pre-wrap">
                  {partialText}
                  <span className="inline-block w-0.5 h-3.5 bg-primary ml-0.5 align-text-bottom animate-pulse-soft" />
                </p>
              </div>
            ) : (
              <p className="text-tertiary text-sm font-medium">
                {status === "Idle" && "Take a quick note with your voice..."}
                {status === "Recording" && "Recording in progress..."}
                {status === "Processing" && "Transcribing..."}
              </p>
            )}
          </div>

          <button
            type="button"
            onClick={handleToggle}
            disabled={status === "Processing"}
            title={
              status === "Idle"
                ? "Start recording (use your hotkey)"
                : status === "Recording"
                  ? "Stop recording"
                  : "Processing transcription..."
            }
            className={`
              relative w-12 h-12 rounded-2xl transition-all duration-300
              flex items-center justify-center flex-shrink-0
              ${
                status === "Idle"
                  ? "bg-primary text-[var(--primary-foreground)] hover:opacity-90"
                  : status === "Recording"
                    ? "bg-recording text-white"
                    : "bg-processing text-white"
              }
              disabled:cursor-not-allowed disabled:opacity-50
              active:translate-y-0 active:shadow-none
            `}
          >
            {status === "Recording" && (
              <span className="absolute inset-0 rounded-2xl bg-recording/40 animate-recording-ring" />
            )}

            {status === "Processing" && (
              <span className="absolute inset-0 rounded-2xl border-2 border-white/20 border-t-white animate-spin-slow" />
            )}

            {status === "Idle" && (
              <svg
                className="w-5 h-5 relative z-10"
                fill="currentColor"
                viewBox="0 0 24 24"
              >
                <path d="M12 14c1.66 0 3-1.34 3-3V5c0-1.66-1.34-3-3-3S9 3.34 9 5v6c0 1.66 1.34 3 3 3z" />
                <path d="M17 11c0 2.76-2.24 5-5 5s-5-2.24-5-5H5c0 3.53 2.61 6.43 6 6.92V21h2v-3.08c3.39-.49 6-3.39 6-6.92h-2z" />
              </svg>
            )}
            {status === "Recording" && (
              <svg
                className="w-5 h-5 relative z-10"
                fill="currentColor"
                viewBox="0 0 24 24"
              >
                <rect x="6" y="6" width="12" height="12" rx="2" />
              </svg>
            )}
            {status === "Processing" && (
              <svg
                className="w-5 h-5 relative z-10 animate-pulse"
                fill="none"
                stroke="currentColor"
                viewBox="0 0 24 24"
                strokeWidth={2}
              >
                <path
                  strokeLinecap="round"
                  strokeLinejoin="round"
                  d="M4 4v5h.582m15.356 2A8.001 8.001 0 004.582 9m0 0H9m11 11v-5h-.581m0 0a8.003 8.003 0 01-15.357-2m15.357 2H15"
                />
              </svg>
            )}
          </button>
        </div>
      </div>

      {error && (
        <div className="w-full max-w-xl mb-6 p-4 rounded-lg border border-[var(--danger)]/30 bg-[var(--danger)]/10">
          <p className="text-sm text-[var(--danger)]">{error}</p>
        </div>
      )}

      {lastTranscription && (
        <div className="w-full max-w-xl animate-fade-in">
          <div className="card p-6">
            <div className="flex items-center gap-2 mb-3">
              <h3 className="text-xs font-medium text-secondary uppercase tracking-wide">
                Result
              </h3>
              <span className="text-xs px-2 py-0.5 rounded-full bg-accent-soft text-primary font-medium">
                Done
              </span>
              {elapsedMs != null && (
                <span className="text-xs text-tertiary ml-auto font-medium tabular-nums">
                  {formatElapsed(elapsedMs)}
                  {wordsPerMinute != null && ` · ${wordsPerMinute} WPM`}
                </span>
              )}
            </div>
            {usedRawNoPolish && (
              <p className="text-xs text-tertiary mb-2 font-medium">
                Shown as transcribed — refinement skipped (already clear).
              </p>
            )}
            <p className="text-text whitespace-pre-wrap leading-relaxed font-medium">
              {lastTranscription}
            </p>
          </div>
        </div>
      )}

      {!lastTranscription && status === "Idle" && (
        <div className="w-full max-w-xl animate-fade-in">
          <div className="flex items-center justify-between mb-4">
            <h2 className="text-xs font-semibold text-secondary uppercase tracking-wide">
              Recent Recordings
            </h2>
          </div>
          {recents.length === 0 ? (
            <div className="card p-8 text-center">
              <p className="text-sm text-tertiary font-medium">
                Your recent recordings will appear here
              </p>
            </div>
          ) : (
            <div className="space-y-3">
              {recents.map((item) => (
                <div
                  key={item.id}
                  className="card p-4 flex items-center justify-between gap-4 hover:border-border-hover transition-colors group"
                >
                  <div className="flex-1 min-w-0">
                    <p className="text-sm text-text font-medium truncate">
                      {item.polished_text || item.raw_text}
                    </p>
                    <p className="text-xs text-tertiary mt-1 font-medium">
                      {formatRecentDate(item.timestamp)}
                    </p>
                  </div>
                  <div className="flex items-center gap-1.5 opacity-60 group-hover:opacity-100 transition-opacity">
                    <button
                      type="button"
                      onClick={() => handleCopy(item.polished_text || item.raw_text)}
                      title="Copy to clipboard"
                      className="p-2 text-text hover:text-primary hover:bg-secondary rounded-lg transition-all"
                    >
                      <svg
                        className="w-4 h-4"
                        fill="none"
                        stroke="currentColor"
                        viewBox="0 0 24 24"
                        strokeWidth={2}
                      >
                        <path
                          strokeLinecap="round"
                          strokeLinejoin="round"
                          d="M8 5H6a2 2 0 00-2 2v12a2 2 0 002 2h10a2 2 0 002-2v-1M8 5a2 2 0 002 2h2a2 2 0 002-2M8 5a2 2 0 012-2h2a2 2 0 012 2m0 0h2a2 2 0 012 2v3"
                        />
                      </svg>
                    </button>
                    <button
                      type="button"
                      onClick={() => handleInject(item.polished_text || item.raw_text)}
                      title="Inject text at cursor"
                      className="p-2 text-text hover:text-primary hover:bg-secondary rounded-lg transition-all"
                    >
                      <svg
                        className="w-4 h-4"
                        fill="none"
                        stroke="currentColor"
                        viewBox="0 0 24 24"
                        strokeWidth={2}
                      >
                        <path
                          strokeLinecap="round"
                          strokeLinejoin="round"
                          d="M8 4H6a2 2 0 00-2 2v12a2 2 0 002 2h12a2 2 0 002-2V6a2 2 0 00-2-2h-2m-4-1v8m0 0l3-3m-3 3L9 8m-5 5h2.586a1 1 0 01.707.293l2.414 2.414a1 1 0 00.707.293h3.172a1 1 0 00.707-.293l2.414-2.414a1 1 0 01.707-.293H20"
                        />
                      </svg>
                    </button>
                  </div>
                </div>
              ))}
            </div>
          )}
        </div>
      )}
    </div>
  );
}
