import { useState, useEffect } from "react";
import { EventsOn, Quit } from "../../wailsjs/runtime/runtime";
import { HideMiniMode } from "../../wailsjs/go/main/App";

type Status = "Idle" | "Recording" | "Processing";

export default function RecordingIndicator() {
  const [status, setStatus] = useState<Status>("Idle");
  const [showButtons, setShowButtons] = useState(false);

  useEffect(() => {
    // Listen for state changes
    EventsOn("state-changed", (newStatus: string) => {
      setStatus(newStatus as Status);
    });
  }, []);

  const handleMaximize = (e: React.MouseEvent) => {
    e.stopPropagation();
    HideMiniMode();
  };

  const handleClose = (e: React.MouseEvent) => {
    e.stopPropagation();
    Quit();
  };

  const getBackgroundColor = () => {
    switch (status) {
      case "Recording":
        return "rgba(220, 38, 38, 0.95)";
      case "Processing":
        return "rgba(79, 70, 229, 0.95)";
      default:
        return "rgba(30, 30, 40, 0.95)";
    }
  };

  return (
    <div
      className="w-full h-full relative"
      onMouseEnter={() => setShowButtons(true)}
      onMouseLeave={() => setShowButtons(false)}
    >
      {/* Main indicator circle */}
      <div
        className="w-full h-full flex items-center justify-center rounded-full cursor-move transition-all"
        style={
          {
            "--wails-draggable": "drag",
            background: getBackgroundColor(),
          } as React.CSSProperties
        }
      >
        {status === "Idle" && (
          <svg
            className="w-7 h-7 text-gray-400"
            fill="currentColor"
            viewBox="0 0 24 24"
          >
            <path d="M12 14c1.66 0 3-1.34 3-3V5c0-1.66-1.34-3-3-3S9 3.34 9 5v6c0 1.66 1.34 3 3 3z" />
            <path d="M17 11c0 2.76-2.24 5-5 5s-5-2.24-5-5H5c0 3.53 2.61 6.43 6 6.92V21h2v-3.08c3.39-.49 6-3.39 6-6.92h-2z" />
          </svg>
        )}

        {status === "Recording" && (
          <div className="relative">
            <div className="absolute inset-0 rounded-full bg-white/30 animate-ping" />
            <div className="relative w-10 h-10 rounded-full bg-white/20 flex items-center justify-center">
              <svg
                className="w-6 h-6 text-white"
                fill="currentColor"
                viewBox="0 0 24 24"
              >
                <path d="M12 14c1.66 0 3-1.34 3-3V5c0-1.66-1.34-3-3-3S9 3.34 9 5v6c0 1.66 1.34 3 3 3z" />
                <path d="M17 11c0 2.76-2.24 5-5 5s-5-2.24-5-5H5c0 3.53 2.61 6.43 6 6.92V21h2v-3.08c3.39-.49 6-3.39 6-6.92h-2z" />
              </svg>
            </div>
          </div>
        )}

        {status === "Processing" && (
          <div className="relative w-10 h-10">
            <div className="absolute inset-0 rounded-full border-3 border-t-white border-r-white/30 border-b-white/30 border-l-white/30 animate-spin" />
            <div className="absolute inset-1 rounded-full bg-indigo-600 flex items-center justify-center">
              <svg
                className="w-5 h-5 text-white"
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
            </div>
          </div>
        )}
      </div>

      {/* Floating action buttons - appear on hover */}
      {showButtons && status === "Idle" && (
        <div className="absolute -top-2 -right-2 flex gap-1">
          {/* Maximize button */}
          <button
            onClick={handleMaximize}
            className="w-5 h-5 rounded-full bg-green-500 hover:bg-green-400 flex items-center justify-center transition-colors shadow-lg"
            title="Open full app"
          >
            <svg
              className="w-3 h-3 text-white"
              fill="none"
              stroke="currentColor"
              viewBox="0 0 24 24"
            >
              <path
                strokeLinecap="round"
                strokeLinejoin="round"
                strokeWidth={2}
                d="M4 8V4m0 0h4M4 4l5 5m11-1V4m0 0h-4m4 0l-5 5M4 16v4m0 0h4m-4 0l5-5m11 5l-5-5m5 5v-4m0 4h-4"
              />
            </svg>
          </button>
          {/* Close button */}
          <button
            onClick={handleClose}
            className="w-5 h-5 rounded-full bg-red-500 hover:bg-red-400 flex items-center justify-center transition-colors shadow-lg"
            title="Quit voxflow"
          >
            <svg
              className="w-3 h-3 text-white"
              fill="none"
              stroke="currentColor"
              viewBox="0 0 24 24"
            >
              <path
                strokeLinecap="round"
                strokeLinejoin="round"
                strokeWidth={2}
                d="M6 18L18 6M6 6l12 12"
              />
            </svg>
          </button>
        </div>
      )}
    </div>
  );
}
