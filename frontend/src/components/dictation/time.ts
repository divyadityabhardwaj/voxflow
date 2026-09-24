const DAY_MS = 24 * 60 * 60 * 1000;

const relative = new Intl.RelativeTimeFormat(undefined, { numeric: "auto" });
const timeOfDay = new Intl.DateTimeFormat(undefined, { hour: "numeric", minute: "2-digit" });
const dayAndTime = new Intl.DateTimeFormat(undefined, {
  month: "short",
  day: "numeric",
  hour: "numeric",
  minute: "2-digit",
});

const startOfDay = (d: Date) => new Date(d.getFullYear(), d.getMonth(), d.getDate()).getTime();

// "2 min ago", "3 hr ago", "yesterday", then a date.
export function relativeTime(timestamp: string | Date, now = new Date()) {
  const d = new Date(timestamp);
  if (isNaN(d.getTime())) return "";
  const sec = Math.round((now.getTime() - d.getTime()) / 1000);
  if (sec < 45) return "just now";
  if (sec < 3600) return relative.format(-Math.round(sec / 60), "minute");
  if (startOfDay(d) === startOfDay(now)) return relative.format(-Math.round(sec / 3600), "hour");
  if (startOfDay(now) - startOfDay(d) <= DAY_MS) return relative.format(-1, "day");
  return dayAndTime.format(d);
}

export const clockTime = (timestamp: string | Date) => {
  const d = new Date(timestamp);
  return isNaN(d.getTime()) ? "" : timeOfDay.format(d);
};

// Group header for History: Today, Yesterday, This week, then month and year.
export function dateGroup(timestamp: string | Date, now = new Date()) {
  const d = new Date(timestamp);
  const days = Math.round((startOfDay(now) - startOfDay(d)) / DAY_MS);
  if (days <= 0) return "Today";
  if (days === 1) return "Yesterday";
  if (days < 7) return "This week";
  return d.toLocaleDateString(undefined, { month: "long", year: "numeric" });
}

export const dayKey = (d: Date) => startOfDay(d);
export { DAY_MS };
