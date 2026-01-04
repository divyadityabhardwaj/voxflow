import { useState, useEffect } from "react";
import { EventsOn, Quit } from "../../wailsjs/runtime/runtime";
import {
  HideMiniMode,
  ToggleRecording,
  GetStatus,
} from "../../wailsjs/go/main/App";
import { useTheme } from "../contexts/ThemeContext";

type Status = "Idle" | "Recording" | "Processing";

/**
 * RecordingIndicator - Horizontal Glass Soundbar
 *
 * Layout (200x60 pixels):
 * ┌──────────────────────────────────────────────┐
 * │  [● Record]     |      [⤢]    [✕]        │
 * └──────────────────────────────────────────────┘
 */
export default function RecordingIndicator() {
  const [status, setStatus] = useState<Status>("Idle");
  const [hoveredButton, setHoveredButton] = useState<string | null>(null);
  const { theme } = useTheme();

  useEffect(() => {
    GetStatus().then((s) => setStatus(s as Status));

    EventsOn("state-changed", (newStatus: string) => {
      setStatus(newStatus as Status);
    });
  }, []);

  const handleRecordClick = async (e: React.MouseEvent) => {
    e.preventDefault();
    e.stopPropagation();
    if (status !== "Processing") {
      await ToggleRecording();
    }
  };

  const handleExpandClick = (e: React.MouseEvent) => {
    e.preventDefault();
    e.stopPropagation();
    HideMiniMode();
  };

  const handleQuitClick = (e: React.MouseEvent) => {
    e.preventDefault();
    e.stopPropagation();
    Quit();
  };

  const isDark = theme === "dark";

  // Premium Glass Settings
  const glassStyle = {
    background: isDark ? "rgba(20, 20, 25, 0.7)" : "rgba(255, 255, 255, 0.7)",
    backdropFilter: "blur(20px)",
    WebkitBackdropFilter: "blur(20px)",
    border: isDark
      ? "1px solid rgba(255, 255, 255, 0.1)"
      : "1px solid rgba(255, 255, 255, 0.4)",
    boxShadow: isDark
      ? "0 8px 32px rgba(0, 0, 0, 0.4), inset 0 1px 0 rgba(255, 255, 255, 0.1)"
      : "0 8px 32px rgba(0, 0, 0, 0.15), inset 0 1px 0 rgba(255, 255, 255, 0.5)",
  };

  return (
    <div
      className="w-full h-full flex flex-row items-center justify-between px-3 overflow-hidden select-none"
      style={
        {
          ...glassStyle,
          borderRadius: "30px", // Fully rounded pill ends
          // @ts-ignore - Wails drag property
          "--wails-draggable": "drag",
        } as React.CSSProperties
      }
    >
      {/* === LEFT: RECORD BUTTON (Big Target) === */}
      <div
        className="flex-none flex items-center justify-center cursor-pointer no-drag mr-3"
        style={{ WebkitAppRegion: "no-drag" } as React.CSSProperties}
        onClick={handleRecordClick}
        title={status === "Idle" ? "Start Recording" : "Stop Recording"}
      >
        <div
          className={`relative rounded-full flex items-center justify-center transition-all duration-300 ${
            status === "Recording" ? "w-10 h-10" : "w-10 h-10 hover:scale-105"
          }`}
        >
          {/* Button Background */}
          <div
            className="absolute inset-0 rounded-full transition-all duration-300 shadow-md"
            style={{
              background:
                status === "Recording"
                  ? "linear-gradient(135deg, #ef4444 0%, #dc2626 100%)"
                  : isDark
                  ? "rgba(255,255,255,0.15)"
                  : "rgba(0,0,0,0.08)",
            }}
          />

          {/* Inner Icon / State */}
          <div className="relative z-10 flex items-center justify-center">
            {status === "Idle" && (
              <div className="w-3.5 h-3.5 rounded-full bg-red-500 shadow-inner" />
            )}

            {status === "Recording" && (
              <div className="w-4 h-4 rounded-sm bg-white animate-pulse" />
            )}

            {status === "Processing" && (
              <svg
                className="w-5 h-5 text-purple-500 animate-spin"
                fill="none"
                viewBox="0 0 24 24"
              >
                <circle
                  className="opacity-25"
                  cx="12"
                  cy="12"
                  r="10"
                  stroke="currentColor"
                  strokeWidth="4"
                ></circle>
                <path
                  className="opacity-75"
                  fill="currentColor"
                  d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"
                ></path>
              </svg>
            )}

            {/* Ping Ring */}
            {status === "Recording" && (
              <div className="absolute inset-0 -m-6 rounded-full border border-red-500/30 animate-[ping_2s_ease-in-out_infinite]" />
            )}
          </div>
        </div>
      </div>

      {/* === CENTER: DRAG AREA / SPACER  === */}
      <div className="flex-1 h-full flex items-center justify-center opacity-20 pointer-events-none px-2">
        {/* Soundbar visual dots */}
        <div className="flex gap-1.5">
          <div className="w-1 h-1 rounded-full bg-current opacity-70" />
          <div className="w-1 h-1 rounded-full bg-current opacity-50" />
          <div className="w-1 h-1 rounded-full bg-current opacity-30" />
        </div>
      </div>

      {/* === RIGHT: ACTIONS (Expand & Quit) === */}
      <div className="flex-none flex items-center gap-2">
        {/* Expand */}
        <div
          className="w-8 h-8 flex items-center justify-center cursor-pointer transition-colors no-drag rounded-full"
          style={
            {
              WebkitAppRegion: "no-drag",
              background:
                hoveredButton === "expand"
                  ? isDark
                    ? "rgba(255,255,255,0.15)"
                    : "rgba(0,0,0,0.08)"
                  : "transparent",
              color:
                hoveredButton === "expand"
                  ? isDark
                    ? "#fff"
                    : "#000"
                  : "inherit",
            } as React.CSSProperties
          }
          onClick={handleExpandClick}
          onMouseEnter={() => setHoveredButton("expand")}
          onMouseLeave={() => setHoveredButton(null)}
          title="Expand"
        >
          <svg
            className="w-4 h-4 opacity-70"
            fill="none"
            viewBox="0 0 24 24"
            stroke="currentColor"
            strokeWidth={2}
          >
            <path
              strokeLinecap="round"
              strokeLinejoin="round"
              d="M4 8V4m0 0h4M4 4l5 5m11-1V4m0 0h-4m4 0l-5 5M4 16v4m0 0h4m-4 0l5-5m11 5l-5-5m5 5v-4m0 4h-4"
            />
          </svg>
        </div>

        {/* Quit */}
        <div
          className="w-8 h-8 flex items-center justify-center cursor-pointer transition-colors no-drag rounded-full"
          style={
            {
              WebkitAppRegion: "no-drag",
              background: hoveredButton === "quit" ? "#ef4444" : "transparent",
              color: hoveredButton === "quit" ? "#fff" : "inherit",
            } as React.CSSProperties
          }
          onClick={handleQuitClick}
          onMouseEnter={() => setHoveredButton("quit")}
          onMouseLeave={() => setHoveredButton(null)}
          title="Quit"
        >
          <svg
            className="w-4 h-4 opacity-70 hover:opacity-100"
            fill="none"
            viewBox="0 0 24 24"
            stroke="currentColor"
            strokeWidth={2}
          >
            <path
              strokeLinecap="round"
              strokeLinejoin="round"
              d="M6 18L18 6M6 6l12 12"
            />
          </svg>
        </div>
      </div>
    </div>
  );
}
