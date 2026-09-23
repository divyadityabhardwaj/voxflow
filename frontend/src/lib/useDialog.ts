import { useEffect, useRef } from "react";

const FOCUSABLE =
  'button:not(:disabled), [href], input:not(:disabled), select:not(:disabled), textarea:not(:disabled), [tabindex]:not([tabindex="-1"])';

// Modal keyboard/focus plumbing: focuses [data-autofocus] (or the panel) on
// open, keeps Tab inside the panel, forwards keydown to the caller, and
// returns focus to whatever was focused before on close.
export function useDialog<T extends HTMLElement>(
  open: boolean,
  onKeyDown: (e: KeyboardEvent) => void,
) {
  const panelRef = useRef<T>(null);
  const keyHandler = useRef(onKeyDown);
  keyHandler.current = onKeyDown;

  useEffect(() => {
    if (!open) return;
    const previous = document.activeElement as HTMLElement | null;
    const panel = panelRef.current;
    (panel?.querySelector<HTMLElement>("[data-autofocus]") ?? panel)?.focus();

    const handle = (e: KeyboardEvent) => {
      if (e.key === "Tab" && panel) {
        const items = Array.from(panel.querySelectorAll<HTMLElement>(FOCUSABLE));
        const first = items[0];
        const last = items[items.length - 1];
        const active = document.activeElement;
        if (!panel.contains(active) || (e.shiftKey ? active === first : active === last)) {
          e.preventDefault();
          (e.shiftKey ? last : first)?.focus();
        }
      }
      keyHandler.current(e);
    };
    window.addEventListener("keydown", handle);
    return () => {
      window.removeEventListener("keydown", handle);
      previous?.focus();
    };
  }, [open]);

  return panelRef;
}
