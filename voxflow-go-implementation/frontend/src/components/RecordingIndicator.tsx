import { useState, useEffect } from "react";
import { EventsOn, Quit } from "../../wailsjs/runtime/runtime";
import {
  HideMiniMode,
  ToggleRecording,
  GetStatus,
} from "../../wailsjs/go/main/App";
import { useTheme } from "../contexts/ThemeContext";

type Status = "Idle" | "Recording" | "Processing";

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

  // Waveform Component
  const Waveform = ({ active }: { active: boolean }) => (
    <div className="flex items-center gap-[3px] h-5">
      {[1, 2, 3, 4, 5].map((i) => (
        <div
          key={i}
          className="w-1 rounded-full transition-all duration-300"
          style={{
            background: active ? "#ffffff" : isDark ? "#ffffff" : "#000000",
            opacity: active ? 1 : 0.4,
            height: active ? undefined : "4px",
            animation: active ? `wave 1s ease-in-out infinite` : "none",
            animationDelay: `${i * 0.1}s`,
            minHeight: "4px",
          }}
        />
      ))}
      <style>{`
        @keyframes wave {
          0%, 100% { height: 4px; }
          50% { height: 20px; }
        }
      `}</style>
    </div>
  );

  return (
    <div
      className="w-full h-full flex flex-row items-center justify-between px-3 overflow-hidden select-none transition-colors duration-300"
      style={
        {
          ...glassStyle,
          background:
            status === "Recording"
              ? "rgba(239, 68, 68, 0.95)"
              : glassStyle.background,
          border:
            status === "Recording"
              ? "1px solid rgba(239, 68, 68, 1)"
              : glassStyle.border,
          borderRadius: "30px",
          // @ts-ignore - Wails drag property
          "--wails-draggable": "drag",
        } as React.CSSProperties
      }
    >
      {/* === LEFT: WAVEFORM / RECORD BUTTON === */}
      <div
        className="flex-none flex items-center justify-center cursor-pointer no-drag mr-3"
        style={{ WebkitAppRegion: "no-drag" } as React.CSSProperties}
        onClick={handleRecordClick}
        title={status === "Idle" ? "Start Recording" : "Stop Recording"}
      >
        <div
          className={`relative rounded-full flex items-center justify-center transition-all duration-300 w-10 h-10 hover:scale-105`}
        >
          {/* Inner Icon / State */}
          <div className="relative z-10 flex items-center justify-center">
            {status === "Processing" ? (
              <svg
                className="w-6 h-6 text-purple-500 animate-spin"
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
            ) : (
              <Waveform active={status === "Recording"} />
            )}
          </div>
        </div>
      </div>

      {/* === CENTER: DRAG HANDLE === */}
      <div className="flex-1 h-full flex items-center justify-center pointer-events-auto px-2 group">
        <div
          className="p-2 cursor-move opacity-40 group-hover:opacity-100 transition-opacity"
          style={{ "--wails-draggable": "drag" } as React.CSSProperties}
          title="Drag to move"
        >
          {/* Grip Icon - White when recording, otherwise current color */}
          <svg
            className={`w-5 h-5 transform rotate-90 transition-colors ${
              status === "Recording" ? "text-white" : "text-current"
            }`}
            fill="currentColor"
            viewBox="0 0 24 24"
          >
            <path d="M11 18c0 1.1-.9 2-2 2s-2-.9-2-2 .9-2 2-2 2 .9 2 2zm-2-8c-1.1 0-2 .9-2 2s.9 2 2 2 2-.9 2-2-.9-2-2-2zm0-6c-1.1 0-2 .9-2 2s.9 2 2 2 2-.9 2-2-.9-2-2-2zm6 4c1.1 0 2-.9 2-2s-.9-2-2-2-2 .9-2 2 .9 2 2 2zm0 2c-1.1 0-2 .9-2 2s.9 2 2 2 2-.9 2-2-.9-2-2-2zm0 6c-1.1 0-2 .9-2 2s.9 2 2 2 2-.9 2-2-.9-2-2-2z" />
          </svg>
        </div>
      </div>

      {/* === RIGHT: ACTIONS (Expand & Quit) === */}
      <div className="flex-none flex items-center gap-2">
        {/* Expand Button */}
        <div
          className={`w-8 h-8 flex items-center justify-center cursor-pointer transition-colors no-drag rounded-full ${
            status === "Recording" ? "text-white hover:bg-white/20" : ""
          }`}
          style={
            status !== "Recording"
              ? {
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
                }
              : ({ WebkitAppRegion: "no-drag" } as any)
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

        {/* Quit Button */}
        <div
          className={`w-8 h-8 flex items-center justify-center cursor-pointer transition-colors no-drag rounded-full ${
            status === "Recording" ? "text-white hover:bg-white/20" : ""
          }`}
          style={
            status !== "Recording"
              ? {
                  WebkitAppRegion: "no-drag",
                  background:
                    hoveredButton === "quit" ? "#ef4444" : "transparent",
                  color: hoveredButton === "quit" ? "#fff" : "inherit",
                }
              : ({ WebkitAppRegion: "no-drag" } as any)
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
