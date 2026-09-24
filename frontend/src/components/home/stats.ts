import { DAY_MS, dayKey } from "../dictation/time";

export interface StatsInput {
  timestamp: string;
  raw_text: string;
  polished_text: string;
  words_per_second?: number;
}

export interface Stats {
  words: number;
  minutesSaved: number;
  streakDays: number;
}

const TYPING_WPM = 40;

export const countWords = (text: string) =>
  text.trim() ? text.trim().split(/\s+/).length : 0;

// This week's words, the typing time they saved at 40 WPM (minus the time
// spent dictating), and how many days in a row ending today (or yesterday)
// had a dictation. `items` must be newest first.
export function computeStats(items: StatsInput[], now = new Date()): Stats {
  const weekStart = now.getTime() - 7 * DAY_MS;
  let words = 0;
  let spentSec = 0;
  const days = new Set<number>();
  for (const t of items) {
    const d = new Date(t.timestamp);
    if (isNaN(d.getTime())) continue;
    days.add(dayKey(d));
    if (d.getTime() < weekStart) continue;
    const n = countWords(t.polished_text || t.raw_text);
    words += n;
    if (t.words_per_second && t.words_per_second > 0) spentSec += n / t.words_per_second;
  }

  let day = dayKey(now);
  if (!days.has(day)) day = dayKey(new Date(day - DAY_MS / 2));
  let streakDays = 0;
  while (days.has(day)) {
    streakDays++;
    // Half a day back lands inside the previous day even across DST changes.
    day = dayKey(new Date(day - DAY_MS / 2));
  }

  return {
    words,
    minutesSaved: Math.max(0, Math.round(words / TYPING_WPM - spentSec / 60)),
    streakDays,
  };
}
