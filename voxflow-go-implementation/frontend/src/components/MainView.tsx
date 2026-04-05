import { useState, useEffect } from "react";
import { EventsOn } from "../../wailsjs/runtime/runtime";
import { ToggleRecording, GetStatus } from "../../wailsjs/go/main/App";
import { Events } from "../constants/events";

type Status = "Idle" | "Recording" | "Processing";

export default function MainView() {
  const [status, setStatus] = useState<Status>("Idle");
  const [lastTranscription, setLastTranscription] = useState<string | null>(
    null,
  );
  const [usedRawNoPolish, setUsedRawNoPolish] = useState(false);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    GetStatus().then((s) => setStatus(s as Status));

    EventsOn(Events.StateChanged, (newStatus: string) => {
      setStatus(newStatus as Status);
      if (newStatus === "Recording") {
        setError(null);
        setLastTranscription(null);
        setUsedRawNoPolish(false);
      }
    });

    EventsOn(
      Events.ProcessingComplete,
      (result: {
        polished: string;
        elapsed: number;
        used_raw?: boolean;
      }) => {
        setLastTranscription(result.polished);
        setUsedRawNoPolish(Boolean(result.used_raw));
      },
    );

    EventsOn(Events.Error, (err: string) => {
      setError(err);
    });
  }, []);

  const handleToggle = async () => {
    try {
      await ToggleRecording();
    } catch (err) {
      setError(String(err));
    }
  };

  return (
    <div className="flex flex-col items-center justify-center min-h-screen p-8 animate-fade-in">
      {/* Heading */}
      <div className="text-center mb-10">
        <h1 className="font-serif text-3xl font-black text-text mb-3 uppercase tracking-tighter">
          {status === "Idle" && "For quick thoughts you want to capture"}
          {status === "Recording" && "Listening..."}
          {status === "Processing" && "Processing your thoughts"}
        </h1>
        <p className="text-secondary text-sm font-medium">
          {status === "Idle" &&
            "Press the button or use your hotkey to start recording"}
          {status === "Recording" &&
            "Speak naturally, then press again to stop"}
          {status === "Processing" &&
            "Transcribing and refining your recording"}
        </p>
      </div>

      {/* Input Card */}
      <div className="w-full max-w-xl mb-10">
        <div
          className={`
            card-elevated p-5 flex items-center gap-4 transition-all duration-300
            ${status === "Recording" ? "border-recording" : ""}
            ${status === "Processing" ? "border-processing" : ""}
          `}
        >
          {/* Text area */}
          <div className="flex-1">
            <p className="text-tertiary text-sm font-medium">
              {status === "Idle" && "Take a quick note with your voice..."}
              {status === "Recording" && "Recording in progress..."}
              {status === "Processing" && "Transcribing..."}
            </p>
          </div>

          {/* Mic button */}
          <button
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
              flex items-center justify-center
              ${
                status === "Idle"
                  ? "bg-primary text-white hover:shadow-cartoon-sm hover:-translate-y-1"
                  : status === "Recording"
                    ? "bg-recording text-white"
                    : "bg-processing text-white"
              }
              disabled:cursor-not-allowed disabled:opacity-50
              active:translate-y-0 active:shadow-none
            `}
          >
            {/* Recording ring animation */}
            {status === "Recording" && (
              <span className="absolute inset-0 rounded-2xl bg-recording/40 animate-recording-ring" />
            )}

            {/* Processing spinner */}
            {status === "Processing" && (
              <span className="absolute inset-0 rounded-2xl border-2 border-white/20 border-t-white animate-spin-slow" />
            )}

            {/* Icon */}
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

      {/* Error display */}
      {error && (
        <div className="w-full max-w-xl mb-6 p-4 bg-red-500/10 border-2 border-red-500 rounded-[1.5rem]">
          <p className="text-sm font-bold text-red-500">{error}</p>
        </div>
      )}

      {/* Last transcription - Only polished result */}
      {lastTranscription && (
        <div className="w-full max-w-xl animate-fade-in">
          <div className="card p-6">
            <div className="flex items-center gap-2 mb-3">
              <h3 className="text-xs font-black text-text uppercase tracking-tighter">
                Result
              </h3>
              <span className="brutal-badge">Done</span>
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

      {/* Recent recordings section (placeholder for future) */}
      {!lastTranscription && status === "Idle" && (
        <div className="w-full max-w-xl">
          <div className="flex items-center justify-between mb-4">
            <h2 className="text-xs font-black text-text uppercase tracking-tighter">
              Recent
            </h2>
          </div>
          <div className="card p-8 text-center">
            <p className="text-sm text-tertiary font-medium">
              Your recent recordings will appear here
            </p>
          </div>
        </div>
      )}
    </div>
  );
}
