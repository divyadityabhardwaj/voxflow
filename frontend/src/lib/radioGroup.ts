import type { KeyboardEvent } from "react";

const STEPS: Record<string, number> = {
  ArrowRight: 1,
  ArrowDown: 1,
  ArrowLeft: -1,
  ArrowUp: -1,
};

// Arrow keys for a role="radiogroup" (or "tablist") of role="radio" (or
// "tab") children: move the selection and focus together, wrapping at the ends.
export function onRadioGroupKeyDown<T>(
  e: KeyboardEvent<HTMLElement>,
  values: readonly T[],
  current: T,
  select: (value: T) => void,
) {
  const step = STEPS[e.key];
  if (!step) return;
  e.preventDefault();
  const next = (Math.max(0, values.indexOf(current)) + step + values.length) % values.length;
  select(values[next]);
  e.currentTarget.querySelectorAll<HTMLElement>('[role="radio"], [role="tab"]')[next]?.focus();
}
