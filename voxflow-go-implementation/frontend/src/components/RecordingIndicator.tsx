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
  const [isHovered, setIsHovered] = useState(false);
  const { theme } = useTheme();

  useEffect(() => {
    GetStatus().then((s) => setStatus(s as Status));

    EventsOn("state-changed", (newStatus: string) => {
      setStatus(newStatus as Status);
    });
  }, []);

  const handleClick = async () => {
    if (status === "Idle" || status === "Recording") {
      await ToggleRecording();
    }
  };

  const handleMaximize = (e: React.MouseEvent) => {
    e.stopPropagation();
    HideMiniMode();
  };

  const handleClose = (e: React.MouseEvent) => {
    e.stopPropagation();
    Quit();
  };

  // Theme-aware colors
  const strokeColor =
    theme === "dark" ? "rgba(255, 255, 255, 0.6)" : "rgba(0, 0, 0, 0.6)";
  const strokeColorLight =
    theme === "dark" ? "rgba(255, 255, 255, 0.3)" : "rgba(0, 0, 0, 0.3)";
  const bgColor = theme === "dark" ? "#0a0a0b" : "#ffffff";

  return (
    <div
      className="w-full h-full flex items-center justify-center overflow-hidden"
      style={
        {
          // @ts-ignore - Wails drag property
          "--wails-draggable": "drag",
          background: bgColor,
        } as React.CSSProperties
      }
      onMouseEnter={() => setIsHovered(true)}
      onMouseLeave={() => setIsHovered(false)}
    >
      {/* Main clickable area */}
      <div
        className="relative w-16 h-16 cursor-pointer flex items-center justify-center"
        onClick={handleClick}
      >
        {/* Animated waveform circles */}
        <svg className="absolute inset-0 w-full h-full" viewBox="0 0 100 100">
          {/* Multiple animated circular paths */}
          {[0, 1, 2, 3, 4].map((i) => (
            <ellipse
              key={i}
              cx="50"
              cy="50"
              rx={35 + i * 2}
              ry={32 + i * 3}
              fill="none"
              stroke={i % 2 === 0 ? strokeColor : strokeColorLight}
              strokeWidth="1"
              style={{
                transformOrigin: "50% 50%",
                animation: `orbit${i % 3} ${3 + i * 0.5}s ease-in-out infinite`,
                animationDelay: `${i * 0.2}s`,
              }}
            />
          ))}
        </svg>

        {/* Center content */}
        <div className="relative z-10 flex items-center justify-center">
          {status === "Recording" ? (
            // Red stop dot when recording
            <div className="w-4 h-4 rounded-full bg-red-500 animate-pulse" />
          ) : status === "Processing" ? (
            // Spinning indicator when processing
            <div
              className="w-5 h-5 border-2 border-t-current border-r-transparent border-b-transparent border-l-transparent rounded-full animate-spin"
              style={{ borderTopColor: strokeColor }}
            />
          ) : (
            // Small dot when idle
            <div
              className="w-2 h-2 rounded-full transition-all duration-200"
              style={{ background: strokeColorLight }}
            />
          )}
        </div>

        {/* Hover text */}
        {isHovered && status === "Idle" && (
          <div className="absolute -bottom-5 left-1/2 -translate-x-1/2 whitespace-nowrap">
            <span className="text-[8px] text-secondary opacity-70">Start</span>
          </div>
        )}

        {isHovered && status === "Recording" && (
          <div className="absolute -bottom-5 left-1/2 -translate-x-1/2 whitespace-nowrap">
            <span className="text-[8px] text-red-400 opacity-70">Stop</span>
          </div>
        )}
      </div>

      {/* Hover controls - maximize and close */}
      {isHovered && (
        <div className="absolute top-1 right-1 flex gap-1 animate-fade-in">
          <button
            onClick={handleMaximize}
            className="w-3 h-3 rounded-full bg-green-500 hover:bg-green-400 transition-all hover:scale-125"
            title="Open full app"
          />
          <button
            onClick={handleClose}
            className="w-3 h-3 rounded-full bg-red-500 hover:bg-red-400 transition-all hover:scale-125"
            title="Quit"
          />
        </div>
      )}

      {/* Orbit animations */}
      <style>{`
        @keyframes orbit0 {
          0%, 100% { transform: rotate(0deg) scaleX(1); }
          25% { transform: rotate(5deg) scaleX(0.95); }
          50% { transform: rotate(0deg) scaleX(1.05); }
          75% { transform: rotate(-5deg) scaleX(0.98); }
        }
        @keyframes orbit1 {
          0%, 100% { transform: rotate(0deg) scaleY(1); }
          33% { transform: rotate(-8deg) scaleY(0.92); }
          66% { transform: rotate(8deg) scaleY(1.08); }
        }
        @keyframes orbit2 {
          0%, 100% { transform: rotate(0deg); }
          50% { transform: rotate(10deg); }
        }
      `}</style>
    </div>
  );
}
