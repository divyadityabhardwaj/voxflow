import { useEffect, useState } from "react";
import { EventsOn } from "../../wailsjs/runtime/runtime";
import { GetConfig, GetPushToTalkKey } from "../../wailsjs/go/main/App";
import { Events } from "../constants/events";
import { formatShortcut } from "../lib/shortcut";
import type { Status } from "./useRecordingState";

export interface ProcessingResult {
  polished: string;
  raw: string;
  used_raw?: boolean;
  elapsed: number;
  words_per_second?: number;
  target_app?: string;
  method?: "paste" | "type" | "clipboard" | "none";
  details?: { audio?: number };
}

export const countWords = (text: string) =>
  text.trim() ? text.trim().split(/\s+/).length : 0;

// Where the text ended up, for the pill and the result card.
export function deliveryLabel(r: Pick<ProcessingResult, "method" | "target_app">) {
  const app = r.target_app || "your app";
  switch (r.method) {
    case "paste":
      return `Pasted into ${app}`;
    case "type":
      return `Typed into ${app}`;
    case "clipboard":
      return "Copied — press ⌘V";
    default:
      return "Done";
  }
}

// Level (0..1) of the microphone while recording; 0 otherwise.
export function useAudioLevel(active: boolean) {
  const [level, setLevel] = useState(0);
  useEffect(() => {
    if (!active) {
      setLevel(0);
      return;
    }
    return EventsOn(Events.AudioLevel, (l: number) =>
      setLevel(Math.max(0, Math.min(1, Number(l) || 0))),
    );
  }, [active]);
  return level;
}

// Seconds since the current recording started.
export function useRecordingSeconds(status: Status) {
  const [startedAt, setStartedAt] = useState<number | null>(null);
  const [seconds, setSeconds] = useState(0);
  useEffect(
    () =>
      EventsOn(Events.RecordingStarted, (d: { started_at?: number }) =>
        setStartedAt(d?.started_at || Date.now()),
      ),
    [],
  );
  useEffect(() => {
    if (status !== "Recording") {
      setSeconds(0);
      return;
    }
    // Mounted mid-recording (e.g. the pill after collapsing): count from now.
    const start = startedAt ?? Date.now();
    const tick = () => setSeconds(Math.max(0, Math.floor((Date.now() - start) / 1000)));
    tick();
    const t = setInterval(tick, 250);
    return () => clearInterval(t);
  }, [status, startedAt]);
  return seconds;
}

export const formatClock = (s: number) =>
  `${Math.floor(s / 60)}:${String(s % 60).padStart(2, "0")}`;

const HOLD_KEY_LABELS: Record<string, string> = {
  right_option: "Right ⌥",
  right_command: "Right ⌘",
  fn: "fn",
};

export interface Shortcuts {
  hold: string;
  toggle: string;
}

// The keys that actually work right now. Without Accessibility a single
// hold key can't be watched, so the backend falls back to the chord.
export async function loadShortcuts(): Promise<Shortcuts> {
  const [cfg, ptt] = await Promise.all([GetConfig(), GetPushToTalkKey()]);
  const chord = formatShortcut(cfg.push_to_talk_hotkey || "");
  return {
    hold: ptt.active ? HOLD_KEY_LABELS[ptt.key] ?? chord : chord,
    toggle: formatShortcut(cfg.hands_free_hotkey || cfg.hotkey || ""),
  };
}

export function useShortcuts() {
  const [shortcuts, setShortcuts] = useState<Shortcuts | null>(null);
  useEffect(() => {
    const load = () => loadShortcuts().then(setShortcuts).catch(() => {});
    load();
    window.addEventListener("focus", load);
    return () => window.removeEventListener("focus", load);
  }, []);
  return shortcuts;
}
