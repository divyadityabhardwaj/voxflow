import { useState, useEffect, useRef, type CSSProperties, type MouseEvent } from "react";
import { EventsOn } from "../../wailsjs/runtime/runtime";
import {
  CancelRecording,
  HideMiniMode,
  OpenPrivacySettings,
  SetMiniModeExpanded,
  ToggleRecording,
} from "../../wailsjs/go/main/App";
import { useTheme } from "../contexts/ThemeContext";
import { useToastList, type Toast } from "../contexts/ToastContext";
import { Events } from "../constants/events";
import { useRecordingState, isBusy } from "../hooks/useRecordingState";
import {
  deliveryLabel,
  formatClock,
  useAudioLevel,
  useRecordingSeconds,
  type ProcessingResult,
} from "../hooks/useDictation";
import LevelBars from "./dictation/LevelBars";

const LEAVE_DELAY_MS = 280;
const DONE_MS = 1500;
// Heights the Go side accepts: internal/window/sizes.go.
const PILL_H = 32;
const PILL_WITH_CHIP_H = 84;

const WARNING_ICON =
  "M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-3L13.732 4c-.77-1.333-2.694-1.333-3.464 0L3.34 16c-.77 1.333.192 3 1.732 3z";
const CHECK_ICON = "M5 13l4 4L19 7";
const CLOSE_ICON = "M6 18L18 6M6 6l12 12";

const TOAST_ICONS: Record<Toast["type"], string> = {
  error: WARNING_ICON,
  warning: WARNING_ICON,
  success: CHECK_ICON,
  info: "M13 16h-1v-4h-1m1-4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z",
};

// ponytail: matches backend message text; switch to structured toast codes once Go emits them.
const PILL_LABELS: [string[], string][] = [
  [["microphone access is off"], "Mic blocked — click to allow"],
  [["allow microphone access"], "Allow the mic, then try again"],
  [["allow voxflow in accessibility"], "Can't paste — text copied. Click to fix"],
  [["text copied"], "Text copied — press ⌘V"],
  [["no speech", "no audio"], "Didn't hear anything — check mic"],
  [["pasted raw transcription", "llm refining failed"], "Pasted without clean-up"],
  [["isn't connected"], "Using the default mic"],
  [["transcription failed", "couldn't be transcribed"], "Transcription failed — try again"],
  [["failed to stop recording", "audio stream", "initialize audio"], "Recording failed — try again"],
  [["model not ready"], "Speech engine isn't ready yet"],
  [["model download", "model load"], "Speech engine problem"],
];

const pillLabel = (message: string) => {
  const msg = message.toLowerCase();
  return PILL_LABELS.find(([keys]) => keys.some((k) => msg.includes(k)))?.[1] ?? message;
};

const Icon = ({ d, className = "w-2.5 h-2.5", strokeWidth = 3 }: { d: string; className?: string; strokeWidth?: number }) => (
  <svg className={className} fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={strokeWidth} aria-hidden="true">
    <path strokeLinecap="round" strokeLinejoin="round" d={d} />
  </svg>
);

const NO_DRAG = { WebkitAppRegion: "no-drag" } as unknown as CSSProperties;

const stop = (e: MouseEvent) => {
  e.preventDefault();
  e.stopPropagation();
};

export default function RecordingIndicator() {
  const status = useRecordingState();
  const busy = isBusy(status);
  const recording = status === "Recording";
  const level = useAudioLevel(recording);
  const seconds = useRecordingSeconds(status);
  const [hovered, setHovered] = useState(false);
  const [done, setDone] = useState<string | null>(null);
  const [editing, setEditing] = useState(false);
  const { toasts, dismissToast, clearToasts } = useToastList();
  const activeToast = toasts.length > 0 ? toasts[toasts.length - 1] : null;
  const leaveTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null);
  const { theme } = useTheme();
  const isDark = theme !== "light";

  useEffect(() => {
    let doneTimer: ReturnType<typeof setTimeout> | undefined;
    const unsubs = [
      EventsOn(Events.StateChanged, (s: string) => {
        if (s === "Recording") {
          clearToasts();
          setDone(null);
        }
      }),
      EventsOn(Events.RecordingStarted, (d: { edit?: boolean }) => setEditing(!!d?.edit)),
      EventsOn(Events.ProcessingComplete, (r: ProcessingResult) => {
        if (r.method === "none") return;
        setDone(deliveryLabel(r));
        clearTimeout(doneTimer);
        doneTimer = setTimeout(() => setDone(null), DONE_MS);
      }),
    ];
    return () => {
      clearTimeout(doneTimer);
      unsubs.forEach((u) => u());
    };
  }, [clearToasts]);

  const uiExpanded = hovered || status !== "Idle" || done !== null || activeToast !== null;
  const targetHeight = activeToast ? PILL_WITH_CHIP_H : PILL_H;

  useEffect(() => {
    const timer = setTimeout(() => SetMiniModeExpanded(uiExpanded, targetHeight), 50);
    return () => clearTimeout(timer);
  }, [uiExpanded, targetHeight]);

  useEffect(() => () => {
    if (leaveTimerRef.current) clearTimeout(leaveTimerRef.current);
  }, []);

  const handlePointerEnter = () => {
    if (leaveTimerRef.current) clearTimeout(leaveTimerRef.current);
    setHovered(true);
  };

  const handlePointerLeave = () => {
    if (leaveTimerRef.current) clearTimeout(leaveTimerRef.current);
    leaveTimerRef.current = setTimeout(() => setHovered(false), LEAVE_DELAY_MS);
  };

  const handleToastClick = (e: MouseEvent) => {
    stop(e);
    if (!activeToast) return;
    if (activeToast.settings) {
      OpenPrivacySettings(activeToast.settings).catch(() => HideMiniMode());
      dismissToast(activeToast.id);
    } else {
      HideMiniMode();
    }
  };

  const fg = isDark ? "rgba(255,255,255,0.88)" : "rgba(17,24,39,0.8)";
  const recordingFg = isDark ? "#fca5a5" : "#b91c1c";
  const busyFg = isDark ? "#fcd34d" : "#b45309";
  const doneFg = isDark ? "#6ee7b7" : "#047857";

  const pillStyle = {
    // Near-opaque so the pill reads over any desktop content.
    background: isDark ? "rgba(30,30,32,0.94)" : "rgba(255,255,255,0.96)",
    border: recording
      ? "1.5px solid var(--recording)"
      : busy
        ? "1.5px solid var(--processing)"
        : `1.5px solid ${isDark ? "rgba(255,255,255,0.12)" : "rgba(0,0,0,0.1)"}`,
    boxShadow: isDark ? "0 3px 12px rgba(0,0,0,0.35)" : "0 3px 12px rgba(0,0,0,0.12)",
    "--wails-draggable": "drag",
  } as unknown as CSSProperties;

  const toastTone: Record<Toast["type"], [string, string, string]> = isDark
    ? {
        error: ["rgba(69,10,10,0.95)", "rgba(239,68,68,0.6)", "#fca5a5"],
        warning: ["rgba(69,26,3,0.95)", "rgba(245,158,11,0.6)", "#fcd34d"],
        success: ["rgba(2,44,34,0.95)", "rgba(16,185,129,0.6)", "#6ee7b7"],
        info: ["rgba(23,37,84,0.95)", "rgba(59,130,246,0.6)", "#93c5fd"],
      }
    : {
        error: ["rgba(254,226,226,0.97)", "rgba(239,68,68,0.6)", "#991b1b"],
        warning: ["rgba(254,243,199,0.97)", "rgba(245,158,11,0.6)", "#92400e"],
        success: ["rgba(209,250,229,0.97)", "rgba(16,185,129,0.6)", "#065f46"],
        info: ["rgba(219,234,254,0.97)", "rgba(59,130,246,0.6)", "#1e40af"],
      };

  const hoverBg = isDark ? "hover:bg-white/10" : "hover:bg-black/5";
  const recordLabel = recording ? "Stop and paste" : "Start dictation";

  let body;
  if (done && status === "Idle") {
    body = (
      <div className="flex-1 min-w-0 flex items-center justify-center gap-1 px-1" role="status" style={{ color: doneFg }}>
        <Icon d={CHECK_ICON} className="w-3 h-3 flex-none" />
        <span className="text-[11px] font-semibold truncate" title={done}>{done}</span>
      </div>
    );
  } else if (busy) {
    const label =
      status === "Refining" ? (editing ? "Rewriting…" : "Cleaning up…") : "Transcribing…";
    body = (
      <div className="flex-1 min-w-0 flex items-center justify-center gap-1.5 px-1" role="status" style={{ color: busyFg }}>
        <span className="flex-none size-3 rounded-full border-2 border-current border-t-transparent animate-spin" aria-hidden="true" />
        <span className="text-[11px] font-semibold truncate">{label}</span>
      </div>
    );
  } else if (recording) {
    body = (
      <>
        <button
          type="button"
          className="flex-none h-5 px-1 flex items-center justify-center rounded-full no-drag"
          style={NO_DRAG}
          onClick={(e) => {
            stop(e);
            ToggleRecording();
          }}
          title={recordLabel}
          aria-label={recordLabel}
        >
          <LevelBars level={level} maxHeight={14} color={recordingFg} />
        </button>
        <span className="flex-none text-[11px] font-semibold tabular-nums" style={{ color: recordingFg }} aria-label={`Recording, ${seconds} seconds`}>
          {formatClock(seconds)}
        </span>
        <span className="flex-1 min-w-0 text-[10px] text-center truncate opacity-70" style={{ color: fg }}>
          {editing ? "Say the change" : "Esc to cancel"}
        </span>
        <button
          type="button"
          className={`flex-none size-5 flex items-center justify-center rounded-full no-drag transition-opacity ${hoverBg} ${
            hovered ? "opacity-100" : "opacity-0 focus-visible:opacity-100"
          }`}
          style={{ ...NO_DRAG, color: fg }}
          onClick={(e) => {
            stop(e);
            CancelRecording();
          }}
          title="Cancel — nothing is pasted"
          aria-label="Cancel dictation"
        >
          <Icon d={CLOSE_ICON} className="w-2.5 h-2.5" />
        </button>
      </>
    );
  } else {
    body = (
      <>
        <button
          type="button"
          className={`flex-none h-5 px-1 flex items-center justify-center rounded-full no-drag ${hoverBg}`}
          style={NO_DRAG}
          onClick={(e) => {
            stop(e);
            ToggleRecording();
          }}
          title={recordLabel}
          aria-label={recordLabel}
        >
          <LevelBars level={0} maxHeight={uiExpanded ? 12 : 8} barWidth={uiExpanded ? 2 : 1.5} color={fg} />
        </button>
        {uiExpanded && (
          <>
            <span className="flex-1 min-w-0 flex items-center justify-center cursor-move opacity-50" title="Drag to move" aria-hidden="true" style={{ color: fg }}>
              <svg className="w-2.5 h-2.5 rotate-90" fill="currentColor" viewBox="0 0 24 24">
                <path d="M11 18c0 1.1-.9 2-2 2s-2-.9-2-2 .9-2 2-2 2 .9 2 2zm-2-8c-1.1 0-2 .9-2 2s.9 2 2 2 2-.9 2-2-.9-2-2-2zm0-6c-1.1 0-2 .9-2 2s.9 2 2 2 2-.9 2-2-.9-2-2-2zm6 4c1.1 0 2-.9 2-2s-.9-2-2-2-2 .9-2 2 .9 2 2 2zm0 2c-1.1 0-2 .9-2 2s.9 2 2 2 2-.9 2-2-.9-2-2-2zm0 6c-1.1 0-2 .9-2 2s.9 2 2 2 2-.9 2-2-.9-2-2-2z" />
              </svg>
            </span>
            <button
              type="button"
              className={`flex-none size-5 flex items-center justify-center rounded-full no-drag ${hoverBg}`}
              style={{ ...NO_DRAG, color: fg }}
              onClick={(e) => {
                stop(e);
                HideMiniMode();
              }}
              title="Open VoxFlow"
              aria-label="Open VoxFlow"
            >
              <Icon
                d="M4 8V4m0 0h4M4 4l5 5m11-1V4m0 0h-4m4 0l-5 5M4 16v4m0 0h4m-4 0l5-5m11 5l-5-5m5 5v-4m0 4h-4"
                className="w-2.5 h-2.5"
                strokeWidth={2}
              />
            </button>
          </>
        )}
      </>
    );
  }

  const [toastBg, toastBorder, toastFg] = activeToast ? toastTone[activeToast.type] : ["", "", ""];

  return (
    <div
      className="w-full h-full flex flex-col justify-end pt-0.5 pb-1 px-1 select-none pointer-events-none"
      onMouseEnter={handlePointerEnter}
      onMouseLeave={handlePointerLeave}
    >
      {activeToast && (
        <div
          role={activeToast.type === "error" ? "alert" : "status"}
          className="w-full min-h-[26px] py-0.5 flex items-center px-1.5 mb-1 rounded-lg pointer-events-auto animate-scale-in"
          style={{ background: toastBg, border: `1px solid ${toastBorder}`, color: toastFg }}
        >
          <button
            type="button"
            className="flex-1 min-w-0 flex items-center gap-1 text-left no-drag"
            style={NO_DRAG}
            onClick={handleToastClick}
            title={`${activeToast.message}\n\n${activeToast.settings ? "Click to open System Settings" : "Click to open VoxFlow"}`}
          >
            <Icon d={TOAST_ICONS[activeToast.type]} className="flex-none w-3 h-3" />
            <span className="flex-1 min-w-0 text-[11px] font-semibold leading-tight line-clamp-2">
              {pillLabel(activeToast.message)}
            </span>
          </button>
          <button
            type="button"
            className="flex-none size-5 flex items-center justify-center rounded-full hover:bg-black/10 no-drag"
            style={NO_DRAG}
            onClick={(e) => {
              stop(e);
              dismissToast(activeToast.id);
            }}
            title="Dismiss"
            aria-label="Dismiss"
          >
            <Icon d={CLOSE_ICON} />
          </button>
        </div>
      )}

      <div
        className={`w-full h-[24px] flex items-center gap-1 px-1 rounded-full pointer-events-auto ${
          uiExpanded ? "justify-between" : "justify-center"
        }`}
        style={pillStyle}
      >
        {body}
      </div>
    </div>
  );
}
