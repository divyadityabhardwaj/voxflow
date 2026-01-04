import { useState, useEffect } from "react";
import { EventsOn, Quit } from "../../wailsjs/runtime/runtime";
import { HideMiniMode } from "../../wailsjs/go/main/App";

type Status = "Idle" | "Recording" | "Processing";

export default function RecordingIndicator() {
  const [status, setStatus] = useState<Status>("Idle");
  const [showButtons, setShowButtons] = useState(false);

  useEffect(() => {
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
        return "linear-gradient(135deg, #DC2626 0%, #B91C1C 100%)";
      case "Processing":
        return "linear-gradient(135deg, #7C3AED 0%, #6D28D9 100%)";
      default:
        return "linear-gradient(135deg, #1C1C1F 0%, #141416 100%)";
    }
  };

  const getBorderColor = () => {
    switch (status) {
      case "Recording":
        return "rgba(220, 38, 38, 0.3)";
      case "Processing":
        return "rgba(124, 58, 237, 0.3)";
      default:
        return "rgba(255, 255, 255, 0.1)";
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
        className="w-full h-full flex items-center justify-center rounded-full cursor-move transition-all duration-300"
        style={
          {
            // @ts-ignore - Wails drag property
            "--wails-draggable": "drag",
            background: getBackgroundColor(),
            border: `1px solid ${getBorderColor()}`,
            boxShadow:
              status === "Recording"
                ? "0 0 20px rgba(220, 38, 38, 0.4)"
                : status === "Processing"
                ? "0 0 20px rgba(124, 58, 237, 0.4)"
                : "0 4px 20px rgba(0, 0, 0, 0.3)",
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
            <div className="absolute inset-0 rounded-full bg-white/20 animate-ping" />
            <div className="relative w-10 h-10 rounded-full bg-white/10 flex items-center justify-center backdrop-blur-sm">
              <svg
                className="w-5 h-5 text-white"
                fill="currentColor"
                viewBox="0 0 24 24"
              >
                <rect x="6" y="6" width="12" height="12" rx="2" />
              </svg>
            </div>
          </div>
        )}

        {status === "Processing" && (
          <div className="relative w-10 h-10">
            <div className="absolute inset-0 rounded-full border-2 border-t-white border-r-white/30 border-b-white/30 border-l-white/30 animate-spin" />
            <div className="absolute inset-1.5 rounded-full bg-violet-600/50 flex items-center justify-center backdrop-blur-sm">
              <svg
                className="w-4 h-4 text-white"
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
            </div>
          </div>
        )}
      </div>

      {/* Floating action buttons - appear on hover */}
      {showButtons && status === "Idle" && (
        <div className="absolute -top-1 -right-1 flex gap-1 animate-fade-in">
          {/* Maximize button */}
          <button
            onClick={handleMaximize}
            className="w-5 h-5 rounded-full bg-emerald-500 hover:bg-emerald-400 flex items-center justify-center transition-all duration-200 shadow-lg hover:scale-110"
            title="Open full app"
          >
            <svg
              className="w-2.5 h-2.5 text-white"
              fill="none"
              stroke="currentColor"
              viewBox="0 0 24 24"
              strokeWidth={2}
            >
              <path
                strokeLinecap="round"
                strokeLinejoin="round"
                d="M4 8V4m0 0h4M4 4l5 5m11-1V4m0 0h-4m4 0l-5 5M4 16v4m0 0h4m-4 0l5-5m11 5l-5-5m5 5v-4m0 4h-4"
              />
            </svg>
          </button>
          {/* Close button */}
          <button
            onClick={handleClose}
            className="w-5 h-5 rounded-full bg-red-500 hover:bg-red-400 flex items-center justify-center transition-all duration-200 shadow-lg hover:scale-110"
            title="Quit voxflow"
          >
            <svg
              className="w-2.5 h-2.5 text-white"
              fill="none"
              stroke="currentColor"
              viewBox="0 0 24 24"
              strokeWidth={2}
            >
              <path
                strokeLinecap="round"
                strokeLinejoin="round"
                d="M6 18L18 6M6 6l12 12"
              />
            </svg>
          </button>
        </div>
      )}
    </div>
  );
}
