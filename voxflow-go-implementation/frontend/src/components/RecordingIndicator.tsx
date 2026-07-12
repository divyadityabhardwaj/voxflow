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
  SetMiniModeExpanded,
} from "../../wailsjs/go/main/App";
import { useTheme } from "../contexts/ThemeContext";
import { Events } from "../constants/events";
import { useRecordingState, Status } from "../hooks/useRecordingState";

const LEAVE_DELAY_MS = 280;

const Waveform = ({
  active,
  compact,
  isDark,
}: {
  active: boolean;
  compact?: boolean;
  isDark: boolean;
}) => (
  <div
    className={`flex items-center justify-center ${compact ? "gap-px h-2.5" : "gap-[2px] h-4"}`}
  >
    {[1, 2, 3, 4, 5].map((i) => (
      <div
        key={i}
        className="w-0.5 rounded-full transition-all duration-300"
        style={{
          background: active ? "#ffffff" : isDark ? "#e5e7eb" : "#111827",
          opacity: active ? 1 : 0.45,
          height: active ? undefined : compact ? "1.5px" : "3px",
          animation: active
            ? `${compact ? "waveCompact" : "wave"} ${
                compact ? "0.75s" : "1s"
              } ease-in-out infinite`
            : "none",
          animationDelay: `${i * 0.08}s`,
          minHeight: compact ? "1.5px" : "3px",
        }}
      />
    ))}
    <style>{`
      @keyframes wave { 0%, 100% { height: 3px; } 50% { height: 14px; } }
      @keyframes waveCompact { 0%, 100% { height: 1.5px; } 50% { height: 7px; } }
    `}</style>
  </div>
);

export default function RecordingIndicator() {
  const status = useRecordingState();
  const [hovered, setHovered] = useState(false);
  const [activeToast, setActiveToast] = useState<{
    id: number;
    message: string;
    type: "error" | "warning" | "success" | "info";
  } | null>(null);
  const leaveTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null);
  const { theme } = useTheme();

  const uiExpanded = hovered || activeToast !== null;
  const showStatusLabel = status !== "Idle" && uiExpanded;

  useEffect(() => {
    const unsubState = EventsOn(Events.StateChanged, (newStatus: string) => {
      // Automatically clear toast if recording starts or processing stops
      if (newStatus === "Recording" || newStatus === "Processing") {
        setActiveToast(null);
      }
    });

    const unsubToast = EventsOn(
      Events.Toast,
      (data: {
        message: string;
        type: "error" | "warning" | "success" | "info";
      }) => {
        const id = Date.now();
        setActiveToast({
          id,
          message: data.message,
          type: data.type || "error",
        });

        // Automatically clear after 4 seconds
        setTimeout(() => {
          setActiveToast((current) => {
            if (current && current.id === id) {
              return null;
            }
            return current;
          });
        }, 4000);
      }
    );

    return () => {
      unsubState();
      unsubToast();
    };
  }, []);


  const hasToast = activeToast !== null;
  const targetHeight = hasToast ? 84 : 32;

  useEffect(() => {
    SetMiniModeExpanded(uiExpanded, targetHeight);
  }, [uiExpanded, targetHeight]);

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

  const handleToastClick = (e: MouseEvent) => {
    e.preventDefault();
    e.stopPropagation();
    // Click action: Restore normal mode to let them view details
    HideMiniMode();
  };

  const isDark = theme === "dark";

  // Dynamic Styles tailored to notification type / app state
  let statusBg =
    status === "Recording"
      ? "var(--recording-bg)"
      : status === "Processing"
        ? "var(--processing-bg)"
        : isDark
          ? "rgba(28, 28, 31, 0.62)"
          : "rgba(255, 255, 255, 0.78)";

  let statusBorder =
    status === "Recording"
      ? "2px solid var(--recording)"
      : status === "Processing"
        ? "2px solid var(--processing)"
        : isDark
          ? "2px solid rgba(255, 255, 255, 0.08)"
          : "2px solid rgba(0, 0, 0, 0.06)";

  let statusGlow =
    status === "Recording"
      ? isDark
        ? "0 0 0 1px rgba(248, 113, 113, 0.25), 0 3px 12px rgba(248, 113, 113, 0.18)"
        : "0 0 0 1px rgba(239, 68, 68, 0.18), 0 3px 12px rgba(239, 68, 68, 0.14)"
      : status === "Processing"
        ? isDark
          ? "0 0 0 1px rgba(167, 139, 250, 0.2), 0 3px 12px rgba(167, 139, 250, 0.12)"
          : "0 0 0 1px rgba(139, 92, 246, 0.16), 0 3px 12px rgba(139, 92, 246, 0.10)"
        : isDark
          ? "0 3px 12px rgba(0,0,0,0.25)"
          : "0 3px 12px rgba(0,0,0,0.10)";

  let foregroundColor =
    status === "Recording"
      ? "#ffffff"
      : isDark
        ? "rgba(255, 255, 255, 0.85)"
        : "rgba(17, 24, 39, 0.75)";

  let toastBg = "";
  let toastBorder = "";
  let toastGlow = "";
  let toastForegroundColor = "";

  if (activeToast) {
    if (activeToast.type === "error") {
      toastBg = isDark
        ? "linear-gradient(135deg, rgba(220, 38, 38, 0.28) 0%, rgba(153, 27, 27, 0.18) 100%)"
        : "linear-gradient(135deg, rgba(254, 226, 226, 0.96) 0%, rgba(254, 202, 202, 0.93) 100%)";
      toastBorder = "1.5px solid rgba(239, 68, 68, 0.6)";
      toastGlow = isDark
        ? "0 4px 12px rgba(239, 68, 68, 0.25)"
        : "0 4px 12px rgba(239, 68, 68, 0.15)";
      toastForegroundColor = isDark ? "#fca5a5" : "#dc2626";
    } else if (activeToast.type === "warning") {
      toastBg = isDark
        ? "linear-gradient(135deg, rgba(217, 119, 6, 0.28) 0%, rgba(146, 64, 14, 0.18) 100%)"
        : "linear-gradient(135deg, rgba(254, 243, 199, 0.96) 0%, rgba(253, 230, 138, 0.93) 100%)";
      toastBorder = "1.5px solid rgba(245, 158, 11, 0.6)";
      toastGlow = isDark
        ? "0 4px 12px rgba(245, 158, 11, 0.25)"
        : "0 4px 12px rgba(245, 158, 11, 0.15)";
      toastForegroundColor = isDark ? "#fcd34d" : "#d97706";
    } else if (activeToast.type === "success") {
      toastBg = isDark
        ? "linear-gradient(135deg, rgba(5, 150, 105, 0.28) 0%, rgba(6, 95, 70, 0.18) 100%)"
        : "linear-gradient(135deg, rgba(209, 250, 229, 0.96) 0%, rgba(167, 243, 208, 0.93) 100%)";
      toastBorder = "1.5px solid rgba(16, 185, 129, 0.6)";
      toastGlow = isDark
        ? "0 4px 12px rgba(16, 185, 129, 0.25)"
        : "0 4px 12px rgba(16, 185, 129, 0.15)";
      toastForegroundColor = isDark ? "#6ee7b7" : "#059669";
    } else {
      toastBg = isDark
        ? "linear-gradient(135deg, rgba(37, 99, 235, 0.28) 0%, rgba(30, 58, 138, 0.18) 100%)"
        : "linear-gradient(135deg, rgba(219, 234, 254, 0.96) 0%, rgba(191, 219, 254, 0.93) 100%)";
      toastBorder = "1.5px solid rgba(59, 130, 246, 0.6)";
      toastGlow = isDark
        ? "0 4px 12px rgba(59, 130, 246, 0.25)"
        : "0 4px 12px rgba(59, 130, 246, 0.15)";
      toastForegroundColor = isDark ? "#93c5fd" : "#2563eb";
    }
  }

  // Extract a brief, beautiful, tracked-out label for notifications
  const getShortErrorMessage = (message: string): string => {
    const msg = message.toLowerCase();
    if (msg.includes("no audio was captured")) return "No Audio Captured";
    if (msg.includes("no speech detected")) return "No Speech Detected";
    if (msg.includes("accessibility permission") || msg.includes("text injection failed")) return "Accessibility Error";
    if (msg.includes("failed to stop recording")) {
      if (msg.includes("no audio")) return "No Audio Input";
      return "Stop Failed";
    }
    if (msg.includes("transcription failed")) return "Transcription Failed";
    if (msg.includes("llm refining failed")) return "LLM Failed";
    if (msg.includes("gemini error") || msg.includes("openai error") || msg.includes("groq error") || msg.includes("llm error")) {
      return "LLM API Error";
    }
    if (msg.includes("model download") || msg.includes("model load")) return "Model Error";

    // Default fallback
    if (message.length > 20) {
      return message.substring(0, 18) + "...";
    }
    return message;
  };

  const hoverBgExpand = isDark ? "hover:bg-white/10" : "hover:bg-black/5";
  const hoverBgQuit = "hover:bg-red-500/20";

  return (
    <div
      className="w-full h-full flex flex-col justify-end pt-0.5 pb-1 px-1 select-none pointer-events-none"
      onMouseEnter={handlePointerEnter}
      onMouseLeave={handlePointerLeave}
    >
      {activeToast && (
        /* Floating Toast Card Above Pill */
        <div
          className="w-full h-[26px] flex flex-row items-center justify-between px-1.5 rounded-lg transition-all duration-300 pointer-events-auto cursor-pointer animate-slide-up-fade mb-1"
          onClick={handleToastClick}
          title={`${activeToast.message}\n\nClick to open VoxFlow`}
          style={
            {
              background: toastBg,
              backdropFilter: "blur(8px)",
              WebkitBackdropFilter: "blur(8px)",
              border: toastBorder,
              boxShadow: toastGlow,
            } as unknown as CSSProperties
          }
        >
          {/* Left: Alert Icon */}
          <div className="flex-none flex items-center justify-center size-4 rounded-full bg-white/20 animate-pulse-soft">
            {activeToast.type === "error" && (
              <svg
                className="w-2.5 h-2.5 text-white"
                fill="none"
                viewBox="0 0 24 24"
                stroke="currentColor"
                strokeWidth={3}
              >
                <path
                  strokeLinecap="round"
                  strokeLinejoin="round"
                  d="M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-3L13.732 4c-.77-1.333-2.694-1.333-3.464 0L3.34 16c-.77 1.333.192 3 1.732 3z"
                />
              </svg>
            )}
            {activeToast.type === "warning" && (
              <svg
                className="w-2.5 h-2.5 text-white"
                fill="none"
                viewBox="0 0 24 24"
                stroke="currentColor"
                strokeWidth={3}
              >
                <path
                  strokeLinecap="round"
                  strokeLinejoin="round"
                  d="M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-3L13.732 4c-.77-1.333-2.694-1.333-3.464 0L3.34 16c-.77 1.333.192 3 1.732 3z"
                />
              </svg>
            )}
            {activeToast.type === "success" && (
              <svg
                className="w-2.5 h-2.5 text-white"
                fill="none"
                viewBox="0 0 24 24"
                stroke="currentColor"
                strokeWidth={3}
              >
                <path
                  strokeLinecap="round"
                  strokeLinejoin="round"
                  d="M5 13l4 4L19 7"
                />
              </svg>
            )}
            {activeToast.type === "info" && (
              <svg
                className="w-2.5 h-2.5 text-white"
                fill="none"
                viewBox="0 0 24 24"
                stroke="currentColor"
                strokeWidth={3}
              >
                <path
                  strokeLinecap="round"
                  strokeLinejoin="round"
                  d="M13 16h-1v-4h-1m1-4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z"
                />
              </svg>
            )}
          </div>

          {/* Middle: Shortened message summary */}
          <div className="flex-1 px-1 min-w-0 flex items-center justify-center">
            <span
              className="text-[11px] font-semibold uppercase tracking-[0.08em] truncate whitespace-nowrap text-center animate-fade-in"
              style={{ color: toastForegroundColor }}
            >
              {getShortErrorMessage(activeToast.message)}
            </span>
          </div>

          {/* Right: Dismiss button */}
          <button
            className="flex-none size-4 flex items-center justify-center rounded-full hover:bg-white/20 transition-colors no-drag"
            style={{ WebkitAppRegion: "no-drag" } as unknown as CSSProperties}
            onClick={(e) => {
              e.preventDefault();
              e.stopPropagation();
              setActiveToast(null);
            }}
            title="Dismiss"
          >
            <svg
              className="w-2.5 h-2.5"
              style={{ color: toastForegroundColor }}
              fill="none"
              viewBox="0 0 24 24"
              stroke="currentColor"
              strokeWidth={3}
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

      {/* Main Control Pill */}
      <div
        className="w-full h-[24px] flex flex-row items-center justify-between px-1 rounded-full transition-[box-shadow] duration-150 ease-out pointer-events-auto shadow-sm"
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
        <div className="flex items-center gap-0.5 min-w-0">
          <div
            className="flex-none size-5 flex items-center justify-center cursor-pointer no-drag"
            style={{ WebkitAppRegion: "no-drag" } as unknown as CSSProperties}
            onClick={handleRecordClick}
            title={status === "Idle" ? "Start Recording" : "Stop Recording"}
          >
            <div className="rounded-full size-5 flex items-center justify-center transition-all duration-200">
              {status === "Processing" ? (
                <svg
                  className="w-3 h-3 animate-spin"
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
                  isDark={isDark}
                />
              )}
            </div>
          </div>

          <div
            className={`transition-all duration-200 ease-out overflow-hidden ${
              showStatusLabel ? "opacity-100" : "opacity-0"
            }`}
            aria-hidden={!showStatusLabel}
            style={{ maxWidth: uiExpanded ? 72 : 0 }}
          >
            {status === "Recording" && (
              <div
                className="text-[11px] font-semibold uppercase tracking-[0.14em] whitespace-nowrap overflow-hidden text-ellipsis"
                style={{ color: "var(--recording)" }}
              >
                Recording
              </div>
            )}
            {status === "Processing" && (
              <div
                className="text-[11px] font-semibold uppercase tracking-[0.14em] whitespace-nowrap overflow-hidden text-ellipsis"
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
              className="w-2.5 h-2.5 transform rotate-90"
              style={{ color: foregroundColor }}
              fill="currentColor"
              viewBox="0 0 24 24"
            >
              <path d="M11 18c0 1.1-.9 2-2 2s-2-.9-2-2 .9-2 2-2 2 .9 2 2zm-2-8c-1.1 0-2 .9-2 2s.9 2 2 2 2-.9 2-2-.9-2-2-2zm0-6c-1.1 0-2 .9-2 2s.9 2 2 2 2-.9 2-2-.9-2-2-2zm6 4c1.1 0 2-.9 2-2s-.9-2-2-2-2 .9-2 2 .9 2 2 2zm0 2c-1.1 0-2 .9-2 2s.9 2 2 2 2-.9 2-2-.9-2-2-2zm0 6c-1.1 0-2 .9-2 2s.9 2 2 2-.9-2-2-2-2 .9-2 2z" />
            </svg>
          </div>
        </div>

        {/* Right: Expand + Quit */}
        <div className="flex-none flex items-center gap-0.5">
          <div
            className={`transition-all duration-200 ease-out overflow-hidden ${
              uiExpanded ? "opacity-100 w-4" : "opacity-0 w-0"
                }`}
              >
                <div
                  className={`size-4 flex items-center justify-center cursor-pointer no-drag rounded-full ${hoverBgExpand} transition-colors`}
                  style={{ WebkitAppRegion: "no-drag" } as unknown as CSSProperties}
                  onClick={handleExpandClick}
                  title="Expand"
                >
                  <svg
                    className="w-2 h-2 opacity-70"
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
                className={`size-4 flex items-center justify-center cursor-pointer no-drag rounded-full ${hoverBgQuit} transition-colors`}
                style={{ WebkitAppRegion: "no-drag" } as unknown as CSSProperties}
                onClick={handleQuitClick}
                title="Quit"
              >
                <svg
                  className="w-2 h-2 opacity-75 hover:opacity-100"
                  style={{ color: foregroundColor }}
                  fill="none"
                  viewBox="0 0 24 24"
                  stroke="currentColor"
                  strokeWidth={2.5}
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
