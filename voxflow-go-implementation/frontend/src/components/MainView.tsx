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
    // Get initial status
    GetStatus().then((s) => setStatus(s as Status));

    // Listen for state changes
    EventsOn("state-changed", (newStatus: string) => {
      setStatus(newStatus as Status);
      if (newStatus === "Recording") {
        setError(null);
        setLastTranscription(null);
      }
    });

    // Listen for transcription results
    EventsOn(
      "processing-complete",
      (result: { raw: string; polished: string; elapsed: number }) => {
        setLastTranscription({ raw: result.raw, polished: result.polished });
      }
    );

    // Listen for errors
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

  const getStatusColor = () => {
    switch (status) {
      case "Recording":
        return "bg-recording";
      case "Processing":
        return "bg-processing";
      default:
        return "bg-idle";
    }
  };

  const getStatusRingColor = () => {
    switch (status) {
      case "Recording":
        return "ring-recording/30";
      case "Processing":
        return "ring-processing/30";
      default:
        return "ring-idle/30";
    }
  };

  return (
    <div className="flex flex-col items-center justify-center min-h-[calc(100vh-3rem)] p-8">
      {/* Main recording button */}
      <div className="flex flex-col items-center gap-8">
        <button
          onClick={handleToggle}
          disabled={status === "Processing"}
          className={`
            relative w-32 h-32 rounded-full transition-all duration-300
            ${getStatusColor()}
            ${status === "Recording" ? "recording-pulse" : ""}
            ${status === "Processing" ? "animate-spin-slow opacity-70" : ""}
            ring-4 ${getStatusRingColor()}
            hover:scale-105 active:scale-95
            disabled:cursor-not-allowed
            shadow-lg shadow-dark-950
          `}
        >
          {status === "Idle" && (
            <svg
              className="w-12 h-12 mx-auto text-white"
              fill="currentColor"
              viewBox="0 0 24 24"
            >
              <path d="M12 14c1.66 0 3-1.34 3-3V5c0-1.66-1.34-3-3-3S9 3.34 9 5v6c0 1.66 1.34 3 3 3z" />
              <path d="M17 11c0 2.76-2.24 5-5 5s-5-2.24-5-5H5c0 3.53 2.61 6.43 6 6.92V21h2v-3.08c3.39-.49 6-3.39 6-6.92h-2z" />
            </svg>
          )}
          {status === "Recording" && (
            <svg
              className="w-12 h-12 mx-auto text-white"
              fill="currentColor"
              viewBox="0 0 24 24"
            >
              <rect x="6" y="6" width="12" height="12" rx="2" />
            </svg>
          )}
          {status === "Processing" && (
            <svg
              className="w-12 h-12 mx-auto text-white animate-pulse"
              fill="none"
              stroke="currentColor"
              viewBox="0 0 24 24"
            >
              <path
                strokeLinecap="round"
                strokeLinejoin="round"
                strokeWidth={2}
                d="M4 4v5h.582m15.356 2A8.001 8.001 0 004.582 9m0 0H9m11 11v-5h-.581m0 0a8.003 8.003 0 01-15.357-2m15.357 2H15"
              />
            </svg>
          )}
        </button>

        {/* Status text */}
        <div className="text-center">
          <p className="text-2xl font-semibold text-dark-200">{status}</p>
          <p className="text-sm text-dark-500 mt-1">
            {status === "Idle" && "Press the button or use hotkey to start"}
            {status === "Recording" && "Listening... Press again to stop"}
            {status === "Processing" && "Transcribing and refining..."}
          </p>
        </div>
      </div>

      {/* Error display */}
      {error && (
        <div className="mt-8 p-4 bg-red-900/20 border border-red-800 rounded-lg max-w-md">
          <p className="text-sm text-red-400">{error}</p>
        </div>
      )}

      {/* Last transcription */}
      {lastTranscription && (
        <div className="mt-8 w-full max-w-2xl space-y-4">
          <div className="p-4 bg-dark-900 rounded-lg border border-dark-800">
            <h3 className="text-xs font-medium text-dark-500 uppercase tracking-wider mb-2">
              Polished Result
            </h3>
            <p className="text-dark-200 whitespace-pre-wrap">
              {lastTranscription.polished}
            </p>
          </div>

          <details className="group">
            <summary className="text-xs text-dark-500 cursor-pointer hover:text-dark-400">
              Show raw transcription
            </summary>
            <div className="mt-2 p-4 bg-dark-900/50 rounded-lg border border-dark-800">
              <p className="text-sm text-dark-400 whitespace-pre-wrap">
                {lastTranscription.raw}
              </p>
            </div>
          </details>
        </div>
      )}
    </div>
  );
}
