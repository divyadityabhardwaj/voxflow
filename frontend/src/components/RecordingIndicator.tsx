import {
  useState,
  useEffect,
  useRef,
  type CSSProperties,
  type MouseEvent,
} from "react";
import { EventsOn } from "../../wailsjs/runtime/runtime";
import {
  HideMiniMode,
  ToggleRecording,
  SetMiniModeExpanded,
} from "../../wailsjs/go/main/App";
import { useTheme } from "../contexts/ThemeContext";
import { useToastList, type Toast } from "../contexts/ToastContext";
import { Events } from "../constants/events";
import { useRecordingState, isBusy } from "../hooks/useRecordingState";

const LEAVE_DELAY_MS = 280;

const WARNING_ICON =
  "M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-3L13.732 4c-.77-1.333-2.694-1.333-3.464 0L3.34 16c-.77 1.333.192 3 1.732 3z";

const TOAST_ICONS: Record<Toast["type"], string> = {
  error: WARNING_ICON,
  warning: WARNING_ICON,
  success: "M5 13l4 4L19 7",
  info: "M13 16h-1v-4h-1m1-4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z",
};

// ponytail: matches backend message text; switch to structured toast codes once Go emits them.
const PILL_LABELS: [string[], string][] = [
  [["accessibility", "text injection failed"], "Can't paste — click to fix"],
  [["no speech", "no audio"], "No sound — check mic"],
  [["pasted raw transcription", "llm refining failed"], "Pasted without clean-up"],
  [["transcription failed", "transcription fallback failed"], "Transcription failed"],
  [
    ["failed to stop recording", "audio stream", "initialize audio", "already recording"],
    "Recording failed",
  ],
  [["model not ready"], "Speech model not ready"],
  [["model download", "model load"], "Speech model error"],
];

const pillLabel = (message: string) => {
  const msg = message.toLowerCase();
  return PILL_LABELS.find(([keys]) => keys.some((k) => msg.includes(k)))?.[1] ?? message;
};

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
          background: active
            ? isDark
              ? "#ffffff"
              : "#dc2626"
            : isDark
              ? "#e5e7eb"
              : "#111827",
          opacity: active ? 1 : 0.45,
          height: active
            ? compact
              ? "7px"
              : "14px"
            : compact
              ? "1.5px"
              : "3px",
          animation: active
            ? `pillWave ${compact ? "0.75s" : "1s"} ease-in-out infinite`
            : "none",
          animationDelay: `${i * 0.08}s`,
        }}
      />
    ))}
    <style>{`
      @keyframes pillWave { 0%, 100% { transform: scaleY(0.21); } 50% { transform: scaleY(1); } }
    `}</style>
  </div>
);

export default function RecordingIndicator() {
  const status = useRecordingState();
  const busy = isBusy(status);
  const [hovered, setHovered] = useState(false);
  const { toasts, dismissToast, clearToasts } = useToastList();
  const activeToast = toasts.length > 0 ? toasts[toasts.length - 1] : null;
  const leaveTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null);
  const { theme } = useTheme();

  const uiExpanded = hovered || activeToast !== null;
  const showStatusLabel = status !== "Idle" && uiExpanded;

  useEffect(
    () =>
      EventsOn(Events.StateChanged, (newStatus: string) => {
        if (newStatus === "Recording" || newStatus === "Processing") {
          clearToasts();
        }
      }),
    [clearToasts],
  );


  const hasToast = activeToast !== null;
  const targetHeight = hasToast ? 84 : 32;

  useEffect(() => {
    const timer = setTimeout(() => {
      SetMiniModeExpanded(uiExpanded, targetHeight);
    }, 50);
    return () => clearTimeout(timer);
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
    if (!busy) await ToggleRecording();
  };

  const handleExpandClick = (e: MouseEvent) => {
    e.preventDefault();
    e.stopPropagation();
    HideMiniMode();
  };

  const handleToastClick = (e: MouseEvent) => {
    e.preventDefault();
    e.stopPropagation();
    HideMiniMode(); // Exit mini mode so user can read full toast/details
  };

  const isDark = theme !== "light";

  let statusBg =
    status === "Recording"
      ? "var(--recording-bg)"
      : busy
        ? "var(--processing-bg)"
        : isDark
          ? "rgba(28, 28, 31, 0.62)"
          : "rgba(255, 255, 255, 0.78)";

  let statusBorder =
    status === "Recording"
      ? "2px solid var(--recording)"
      : busy
        ? "2px solid var(--processing)"
        : isDark
          ? "2px solid rgba(255, 255, 255, 0.08)"
          : "2px solid rgba(0, 0, 0, 0.06)";

  let statusGlow =
    status === "Recording"
      ? isDark
        ? "0 0 0 1px rgba(248, 113, 113, 0.25), 0 3px 12px rgba(248, 113, 113, 0.18)"
        : "0 0 0 1px rgba(239, 68, 68, 0.18), 0 3px 12px rgba(239, 68, 68, 0.14)"
      : busy
        ? isDark
          ? "0 0 0 1px rgba(251, 191, 36, 0.2), 0 3px 12px rgba(251, 191, 36, 0.12)"
          : "0 0 0 1px rgba(245, 158, 11, 0.16), 0 3px 12px rgba(245, 158, 11, 0.10)"
        : isDark
          ? "0 3px 12px rgba(0,0,0,0.25)"
          : "0 3px 12px rgba(0,0,0,0.10)";

  const foregroundColor = isDark
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
      toastForegroundColor = isDark ? "#fca5a5" : "#b91c1c";
    } else if (activeToast.type === "warning") {
      toastBg = isDark
        ? "linear-gradient(135deg, rgba(217, 119, 6, 0.28) 0%, rgba(146, 64, 14, 0.18) 100%)"
        : "linear-gradient(135deg, rgba(254, 243, 199, 0.96) 0%, rgba(253, 230, 138, 0.93) 100%)";
      toastBorder = "1.5px solid rgba(245, 158, 11, 0.6)";
      toastGlow = isDark
        ? "0 4px 12px rgba(245, 158, 11, 0.25)"
        : "0 4px 12px rgba(245, 158, 11, 0.15)";
      toastForegroundColor = isDark ? "#fcd34d" : "#92400e";
    } else if (activeToast.type === "success") {
      toastBg = isDark
        ? "linear-gradient(135deg, rgba(5, 150, 105, 0.28) 0%, rgba(6, 95, 70, 0.18) 100%)"
        : "linear-gradient(135deg, rgba(209, 250, 229, 0.96) 0%, rgba(167, 243, 208, 0.93) 100%)";
      toastBorder = "1.5px solid rgba(16, 185, 129, 0.6)";
      toastGlow = isDark
        ? "0 4px 12px rgba(16, 185, 129, 0.25)"
        : "0 4px 12px rgba(16, 185, 129, 0.15)";
      toastForegroundColor = isDark ? "#6ee7b7" : "#047857";
    } else {
      toastBg = isDark
        ? "linear-gradient(135deg, rgba(37, 99, 235, 0.28) 0%, rgba(30, 58, 138, 0.18) 100%)"
        : "linear-gradient(135deg, rgba(219, 234, 254, 0.96) 0%, rgba(191, 219, 254, 0.93) 100%)";
      toastBorder = "1.5px solid rgba(59, 130, 246, 0.6)";
      toastGlow = isDark
        ? "0 4px 12px rgba(59, 130, 246, 0.25)"
        : "0 4px 12px rgba(59, 130, 246, 0.15)";
      toastForegroundColor = isDark ? "#93c5fd" : "#1d4ed8";
    }
  }

  const recordLabel = busy
    ? status === "Refining"
      ? "Cleaning up…"
      : "Transcribing…"
    : status === "Recording"
      ? "Stop dictation"
      : "Start dictation";

  const hoverBgExpand = isDark ? "hover:bg-white/10" : "hover:bg-black/5";

  return (
    <div
      className="w-full h-full flex flex-col justify-end pt-0.5 pb-1 px-1 select-none pointer-events-none"
      onMouseEnter={handlePointerEnter}
      onMouseLeave={handlePointerLeave}
    >
      {activeToast && (
        <div
          role={activeToast.type === "error" ? "alert" : "status"}
          className="w-full min-h-[26px] py-0.5 flex flex-row items-center justify-between px-1.5 rounded-lg transition-all duration-300 pointer-events-auto animate-slide-up-fade mb-1"
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
          <button
            type="button"
            className="flex-1 min-w-0 flex items-center"
            onClick={handleToastClick}
            title={`${activeToast.message}\n\nClick to open VoxFlow`}
            style={{ color: toastForegroundColor }}
          >
            <span className="flex-none flex items-center justify-center size-4 rounded-full bg-white/20 animate-pulse-soft">
              <svg
                className="w-2.5 h-2.5"
                fill="none"
                viewBox="0 0 24 24"
                stroke="currentColor"
                strokeWidth={3}
                aria-hidden="true"
              >
                <path
                  strokeLinecap="round"
                  strokeLinejoin="round"
                  d={TOAST_ICONS[activeToast.type] ?? TOAST_ICONS.error}
                />
              </svg>
            </span>
            <span className="flex-1 min-w-0 px-1 text-[11px] font-semibold leading-tight text-center line-clamp-2 animate-fade-in">
              {pillLabel(activeToast.message)}
            </span>
          </button>

          <button
            type="button"
            className="flex-none size-4 flex items-center justify-center rounded-full hover:bg-white/20 transition-colors no-drag"
            style={{ WebkitAppRegion: "no-drag" } as unknown as CSSProperties}
            onClick={(e) => {
              e.preventDefault();
              e.stopPropagation();
              dismissToast(activeToast.id);
            }}
            title="Dismiss"
            aria-label="Dismiss"
          >
            <svg
              className="w-2.5 h-2.5"
              style={{ color: toastForegroundColor }}
              fill="none"
              viewBox="0 0 24 24"
              stroke="currentColor"
              strokeWidth={3}
              aria-hidden="true"
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

      <div
        className={`w-full h-[24px] flex flex-row items-center px-1 rounded-full transition-[box-shadow] duration-150 ease-out pointer-events-auto shadow-sm ${
          uiExpanded ? "justify-between" : "justify-center"
        }`}
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
        <div className="flex items-center gap-0.5 min-w-0">
          <button
            type="button"
            className="flex-none size-5 flex items-center justify-center cursor-pointer no-drag disabled:cursor-default"
            style={{ WebkitAppRegion: "no-drag" } as unknown as CSSProperties}
            onClick={handleRecordClick}
            disabled={busy}
            title={recordLabel}
            aria-label={recordLabel}
          >
            <div className="rounded-full size-5 flex items-center justify-center transition-all duration-200">
              {busy ? (
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
          </button>

          <div
            className={`transition-all duration-200 ease-out overflow-hidden ${
              showStatusLabel ? "opacity-100" : "opacity-0"
            }`}
            aria-hidden={!showStatusLabel}
            style={{ maxWidth: uiExpanded ? 88 : 0 }}
          >
            {status === "Recording" && (
              <div
                className="text-[11px] font-semibold whitespace-nowrap overflow-hidden text-ellipsis"
                style={{ color: isDark ? "var(--recording)" : "#b91c1c" }}
              >
                Recording
              </div>
            )}
            {busy && (
              <div
                className="text-[11px] font-semibold whitespace-nowrap overflow-hidden text-ellipsis"
                style={{ color: isDark ? "var(--processing)" : "#b45309" }}
              >
                {status === "Refining" ? "Cleaning up…" : "Transcribing…"}
              </div>
            )}
          </div>
        </div>

        <div
          className={`h-full flex items-center justify-center transition-all duration-200 ease-out overflow-hidden min-w-0 ${
            uiExpanded ? "flex-1 px-0.5 opacity-100" : "w-0 opacity-0 pointer-events-none"
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

        <div
          className={`flex-none transition-all duration-200 ease-out overflow-hidden ${
            uiExpanded ? "opacity-100 w-4" : "opacity-0 w-0"
          }`}
        >
          <button
            type="button"
            className={`size-4 flex items-center justify-center cursor-pointer no-drag rounded-full ${hoverBgExpand} transition-colors`}
            style={{ WebkitAppRegion: "no-drag" } as unknown as CSSProperties}
            onClick={handleExpandClick}
            title="Open VoxFlow"
            aria-label="Open VoxFlow"
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
          </button>
        </div>
      </div>
    </div>
  );
}
