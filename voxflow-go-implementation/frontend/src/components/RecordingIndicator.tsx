import {
  useState,
  useEffect,
  useRef,
  type CSSProperties,
  type MouseEvent,
} from "react";
import { EventsOn, Quit } from "../../wailsjs/runtime/runtime";
import {
  HideMiniMode,
  ToggleRecording,
  GetStatus,
  SetMiniModeExpanded,
} from "../../wailsjs/go/main/App";
import { useTheme } from "../contexts/ThemeContext";
import { Events } from "../constants/events";

type Status = "Idle" | "Recording" | "Processing";

const LEAVE_DELAY_MS = 280;

export default function RecordingIndicator() {
  const [status, setStatus] = useState<Status>("Idle");
  const [hovered, setHovered] = useState(false);
  const leaveTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null);
  const { theme } = useTheme();

  const uiExpanded = hovered;
  const showStatusLabel = status !== "Idle" && uiExpanded;

  useEffect(() => {
    GetStatus().then((s) => setStatus(s as Status));

    EventsOn(Events.StateChanged, (newStatus: string) => {
      setStatus(newStatus as Status);
    });
  }, []);

  useEffect(() => {
    SetMiniModeExpanded(uiExpanded);
  }, [uiExpanded]);

  useEffect(() => {
    return () => {
      if (leaveTimerRef.current) clearTimeout(leaveTimerRef.current);
    };
  }, []);

  const clearLeaveTimer = () => {
    if (leaveTimerRef.current) {
      clearTimeout(leaveTimerRef.current);
      leaveTimerRef.current = null;
    }
  };

  const handlePointerEnter = () => {
    clearLeaveTimer();
    setHovered(true);
  };

  const handlePointerLeave = () => {
    clearLeaveTimer();
    leaveTimerRef.current = setTimeout(() => {
      setHovered(false);
      leaveTimerRef.current = null;
    }, LEAVE_DELAY_MS);
  };

  const handleRecordClick = async (e: MouseEvent) => {
    e.preventDefault();
    e.stopPropagation();
    if (status !== "Processing") await ToggleRecording();
  };

  const handleExpandClick = (e: MouseEvent) => {
    e.preventDefault();
    e.stopPropagation();
    HideMiniMode();
  };

  const handleQuitClick = (e: MouseEvent) => {
    e.preventDefault();
    e.stopPropagation();
    Quit();
  };

  const isDark = theme === "dark";

  const statusBg =
    status === "Recording"
      ? "var(--recording-bg)"
      : status === "Processing"
        ? "var(--processing-bg)"
        : isDark
          ? "rgba(28, 28, 31, 0.62)"
          : "rgba(255, 255, 255, 0.78)";

  const statusBorder =
    status === "Recording"
      ? "2px solid var(--recording)"
      : status === "Processing"
        ? "2px solid var(--processing)"
        : isDark
          ? "2px solid rgba(255, 255, 255, 0.08)"
          : "2px solid rgba(0, 0, 0, 0.06)";

  const statusGlow =
    status === "Recording"
      ? isDark
        ? "0 0 0 1px rgba(248, 113, 113, 0.25), 0 14px 36px rgba(248, 113, 113, 0.18)"
        : "0 0 0 1px rgba(239, 68, 68, 0.18), 0 14px 36px rgba(239, 68, 68, 0.14)"
      : status === "Processing"
        ? isDark
          ? "0 0 0 1px rgba(167, 139, 250, 0.2), 0 14px 36px rgba(167, 139, 250, 0.12)"
          : "0 0 0 1px rgba(139, 92, 246, 0.16), 0 14px 36px rgba(139, 92, 246, 0.10)"
        : isDark
          ? "0 4px 18px rgba(0,0,0,0.25)"
          : "0 4px 18px rgba(0,0,0,0.10)";

  const foregroundColor =
    status === "Recording"
      ? "#ffffff"
      : isDark
        ? "rgba(255, 255, 255, 0.85)"
        : "rgba(17, 24, 39, 0.75)";

  const hoverBgExpand = isDark ? "hover:bg-white/10" : "hover:bg-black/5";
  const hoverBgQuit = "hover:bg-red-500/20";

  const Waveform = ({
    active,
    compact,
  }: {
    active: boolean;
    compact?: boolean;
  }) => (
    <div
      className={`flex items-center justify-center ${compact ? "gap-[2px] h-3" : "gap-[3px] h-5"}`}
    >
      {[1, 2, 3, 4, 5].map((i) => (
        <div
          key={i}
          className="w-0.5 rounded-full transition-all duration-300"
          style={{
            background: active ? "#ffffff" : isDark ? "#e5e7eb" : "#111827",
            opacity: active ? 1 : 0.45,
            height: active ? undefined : compact ? "2px" : "4px",
            animation: active
              ? `${compact ? "waveCompact" : "wave"} ${
                  compact ? "0.8s" : "1s"
                } ease-in-out infinite`
              : "none",
            animationDelay: `${i * 0.08}s`,
            minHeight: compact ? "2px" : "4px",
          }}
        />
      ))}
      <style>{`
        @keyframes wave { 0%, 100% { height: 4px; } 50% { height: 18px; } }
        @keyframes waveCompact { 0%, 100% { height: 2px; } 50% { height: 9px; } }
      `}</style>
    </div>
  );

  return (
    <div
      className="w-full h-full flex items-center justify-center overflow-hidden select-none"
      onMouseEnter={handlePointerEnter}
      onMouseLeave={handlePointerLeave}
    >
      <div
        className="w-full h-full flex flex-row items-center justify-between px-1 rounded-full transition-[box-shadow] duration-150 ease-out"
        style={
          {
            background: statusBg,
            backdropFilter: "blur(10px)",
            WebkitBackdropFilter: "blur(10px)",
            border: statusBorder,
            boxShadow: statusGlow,
            "--wails-draggable": "drag",
          } as unknown as CSSProperties
        }
      >
        {/* Left: Record button + label */}
        <div className="flex items-center gap-1 min-w-0">
          <div
            className="flex-none w-7 h-7 flex items-center justify-center cursor-pointer no-drag"
            style={{ WebkitAppRegion: "no-drag" } as unknown as CSSProperties}
            onClick={handleRecordClick}
            title={status === "Idle" ? "Start Recording" : "Stop Recording"}
          >
            <div className="rounded-full w-7 h-7 flex items-center justify-center transition-all duration-200">
              {status === "Processing" ? (
                <svg
                  className="w-3.5 h-3.5 animate-spin"
                  style={{ color: foregroundColor }}
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
                <Waveform
                  active={status === "Recording"}
                  compact={!uiExpanded}
                />
              )}
            </div>
          </div>

          <div
            className={`transition-all duration-200 ease-out overflow-hidden ${
              showStatusLabel ? "opacity-100" : "opacity-0"
            }`}
            aria-hidden={!showStatusLabel}
            style={{ maxWidth: uiExpanded ? 100 : 0 }}
          >
            {status === "Recording" && (
              <div
                className="text-[9px] font-black uppercase tracking-[0.2em] whitespace-nowrap overflow-hidden text-ellipsis"
                style={{ color: "var(--recording)" }}
              >
                Recording
              </div>
            )}
            {status === "Processing" && (
              <div
                className="text-[9px] font-black uppercase tracking-[0.2em] whitespace-nowrap overflow-hidden text-ellipsis"
                style={{ color: "var(--processing)" }}
              >
                Processing
              </div>
            )}
          </div>
        </div>

        {/* Middle: Drag handle */}
        <div
          className={`flex-1 h-full flex items-center justify-center px-0.5 transition-all duration-200 ease-out overflow-hidden min-w-0 ${
            uiExpanded ? "opacity-100" : "opacity-0 pointer-events-none"
          }`}
        >
          <div
            className="p-1 cursor-move opacity-50 hover:opacity-100 transition-opacity duration-200"
            style={{ "--wails-draggable": "drag" } as unknown as CSSProperties}
            title="Drag to move"
          >
            <svg
              className="w-3.5 h-3.5 transform rotate-90"
              style={{ color: foregroundColor }}
              fill="currentColor"
              viewBox="0 0 24 24"
            >
              <path d="M11 18c0 1.1-.9 2-2 2s-2-.9-2-2 .9-2 2-2 2 .9 2 2zm-2-8c-1.1 0-2 .9-2 2s.9 2 2 2 2-.9 2-2-.9-2-2-2zm0-6c-1.1 0-2 .9-2 2s.9 2 2 2 2-.9 2-2-.9-2-2-2zm6 4c1.1 0 2-.9 2-2s-.9-2-2-2-2 .9-2 2 .9 2 2 2zm0 2c-1.1 0-2 .9-2 2s.9 2 2 2 2-.9 2-2-.9-2-2-2zm0 6c-1.1 0-2 .9-2 2s.9 2 2 2-.9-2-2-2-2 .9-2 2z" />
            </svg>
          </div>
        </div>

        {/* Right: Expand + Quit */}
        <div className="flex-none flex items-center gap-1">
          <div
            className={`transition-all duration-200 ease-out overflow-hidden ${
              uiExpanded ? "opacity-100 w-[22px]" : "opacity-0 w-0"
            }`}
          >
            <div
              className={`w-[22px] h-[22px] flex items-center justify-center cursor-pointer no-drag rounded-full ${hoverBgExpand} transition-colors`}
              style={{ WebkitAppRegion: "no-drag" } as unknown as CSSProperties}
              onClick={handleExpandClick}
              title="Expand"
            >
              <svg
                className="w-2.5 h-2.5 opacity-70"
                style={{ color: foregroundColor }}
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
          </div>

          <div
            className={`w-[22px] h-[22px] flex items-center justify-center cursor-pointer no-drag rounded-full ${hoverBgQuit} transition-colors`}
            style={{ WebkitAppRegion: "no-drag" } as unknown as CSSProperties}
            onClick={handleQuitClick}
            title="Quit"
          >
            <svg
              className="w-2.5 h-2.5 opacity-75 hover:opacity-100"
              style={{ color: foregroundColor }}
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
    </div>
  );
}
