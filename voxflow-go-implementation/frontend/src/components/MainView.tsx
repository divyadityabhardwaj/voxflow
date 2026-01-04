import { useState, useEffect } from "react";
import { EventsOn } from "../../wailsjs/runtime/runtime";
import { ToggleRecording, GetStatus } from "../../wailsjs/go/main/App";

type Status = "Idle" | "Recording" | "Processing";

export default function MainView() {
  const [status, setStatus] = useState<Status>("Idle");
  const [lastTranscription, setLastTranscription] = useState<{
    raw: string;
    polished: string;
  } | null>(null);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    GetStatus().then((s) => setStatus(s as Status));

    EventsOn("state-changed", (newStatus: string) => {
      setStatus(newStatus as Status);
      if (newStatus === "Recording") {
        setError(null);
        setLastTranscription(null);
      }
    });

    EventsOn(
      "processing-complete",
      (result: { raw: string; polished: string; elapsed: number }) => {
        setLastTranscription({ raw: result.raw, polished: result.polished });
      }
    );

    EventsOn("error", (err: string) => {
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
        <h1 className="font-serif text-3xl font-medium text-primary mb-3">
          {status === "Idle" && "For quick thoughts you want to capture"}
          {status === "Recording" && "Listening..."}
          {status === "Processing" && "Processing your thoughts"}
        </h1>
        <p className="text-secondary text-sm">
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
            card-elevated p-6 flex items-center gap-4 transition-all duration-300
            ${status === "Recording" ? "ring-2 ring-recording/30" : ""}
            ${status === "Processing" ? "ring-2 ring-processing/30" : ""}
          `}
        >
          {/* Text area */}
          <div className="flex-1">
            <p className="text-tertiary text-sm">
              {status === "Idle" && "Take a quick note with your voice..."}
              {status === "Recording" && "Recording in progress..."}
              {status === "Processing" && "Transcribing..."}
            </p>
          </div>

          {/* Mic button */}
          <button
            onClick={handleToggle}
            disabled={status === "Processing"}
            className={`
              relative w-12 h-12 rounded-full transition-all duration-300
              flex items-center justify-center
              ${
                status === "Idle"
                  ? "bg-[var(--accent)] text-[var(--bg-primary)] hover:scale-105"
                  : status === "Recording"
                  ? "bg-recording text-white"
                  : "bg-processing text-white"
              }
              disabled:cursor-not-allowed
              active:scale-95
            `}
          >
            {/* Recording ring animation */}
            {status === "Recording" && (
              <span className="absolute inset-0 rounded-full bg-recording/50 animate-recording-ring" />
            )}

            {/* Processing spinner */}
            {status === "Processing" && (
              <span className="absolute inset-0 rounded-full border-2 border-white/20 border-t-white animate-spin-slow" />
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
        <div className="w-full max-w-xl mb-6 p-4 bg-red-500/10 border border-red-500/20 rounded-xl">
          <p className="text-sm text-red-500">{error}</p>
        </div>
      )}

      {/* Last transcription */}
      {lastTranscription && (
        <div className="w-full max-w-xl space-y-4 animate-fade-in">
          <div className="card p-6">
            <h3 className="text-xs font-medium text-tertiary uppercase tracking-wider mb-3">
              Result
            </h3>
            <p className="text-primary whitespace-pre-wrap leading-relaxed">
              {lastTranscription.polished}
            </p>
          </div>

          <details className="group">
            <summary className="text-xs text-tertiary cursor-pointer hover:text-secondary transition-colors flex items-center gap-1">
              <svg
                className="w-3 h-3 transition-transform group-open:rotate-90"
                fill="none"
                stroke="currentColor"
                viewBox="0 0 24 24"
                strokeWidth={2}
              >
                <path
                  strokeLinecap="round"
                  strokeLinejoin="round"
                  d="M9 5l7 7-7 7"
                />
              </svg>
              Show raw transcription
            </summary>
            <div className="mt-3 p-4 bg-tertiary rounded-xl">
              <p className="text-sm text-secondary whitespace-pre-wrap">
                {lastTranscription.raw}
              </p>
            </div>
          </details>
        </div>
      )}

      {/* Recent recordings section (placeholder for future) */}
      {!lastTranscription && status === "Idle" && (
        <div className="w-full max-w-xl">
          <div className="flex items-center justify-between mb-4">
            <h2 className="text-xs font-medium text-tertiary uppercase tracking-wider">
              Recent
            </h2>
          </div>
          <p className="text-sm text-tertiary text-center py-8">
            Your recent recordings will appear here
          </p>
        </div>
      )}
    </div>
  );
}
