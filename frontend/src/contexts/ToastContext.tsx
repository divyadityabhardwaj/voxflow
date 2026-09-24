import React, { createContext, useContext, useState, useCallback, useMemo, useRef } from "react";
import { Events } from "../constants/events";

export type PrivacyPane = "microphone" | "accessibility";

export interface ToastAction {
  label: string;
  onClick: () => void;
}

export interface Toast {
  id: number;
  message: string;
  type: "error" | "warning" | "success" | "info";
  // The Privacy & Security pane that fixes the problem.
  settings?: PrivacyPane;
  action?: ToastAction;
}

export interface ToastOptions {
  settings?: PrivacyPane;
  action?: ToastAction;
  ttl?: number | null;
}

interface ToastContextType {
  showToast: (message: string, type?: Toast["type"], options?: ToastOptions) => void;
}

interface ToastListContextType {
  toasts: Toast[];
  dismissToast: (id: number) => void;
  clearToasts: () => void;
}

// Errors, and anything with a fix to click, stay until dismissed so they can
// still be read after opening the window.
const TOAST_TTL_MS: Record<Toast["type"], number | null> = {
  error: null,
  warning: 6000,
  success: 3000,
  info: 3000,
};

const openPrivacySettings = (pane: PrivacyPane) =>
  import("../../wailsjs/go/main/App").then(({ OpenPrivacySettings }) =>
    OpenPrivacySettings(pane),
  );

export const settingsAction = (pane: PrivacyPane): ToastAction => ({
  label: "Open System Settings",
  onClick: () => void openPrivacySettings(pane),
});

const ToastContext = createContext<ToastContextType | null>(null);
// Separate so showToast callers don't re-render on every toast.
const ToastListContext = createContext<ToastListContextType | null>(null);

const ICONS: Record<Toast["type"], string> = {
  error: "M12 8v4m0 4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z",
  warning:
    "M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-3L13.732 4c-.77-1.333-2.694-1.333-3.464 0L3.34 16c-.77 1.333.192 3 1.732 3z",
  success: "M9 12l2 2 4-4m6 2a9 9 0 11-18 0 9 9 0 0118 0z",
  info: "M13 16h-1v-4h-1m1-4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z",
};

const TONE: Record<Toast["type"], string> = {
  error: "bg-red-500/10 border-red-500/30",
  warning: "bg-amber-500/10 border-amber-500/30",
  success: "bg-emerald-500/10 border-emerald-500/30",
  info: "bg-blue-500/10 border-blue-500/30",
};

export function ToastProvider({ children }: { children: React.ReactNode }) {
  const [toasts, setToasts] = useState<Toast[]>([]);
  const [isMiniMode, setIsMiniMode] = useState(false);

  const nextId = useRef(0);

  const dismissToast = useCallback((id: number) => {
    setToasts((prev) => prev.filter((t) => t.id !== id));
  }, []);

  const clearToasts = useCallback(() => setToasts([]), []);

  const showToast = useCallback(
    (message: string, type: Toast["type"] = "error", options: ToastOptions = {}) => {
      const id = nextId.current++;
      const { settings, action = settings && settingsAction(settings) } = options;
      // App.tsx also forwards backend toasts, without `settings`; merge the two.
      // Never merge an explicit action: each Undo belongs to its own row.
      setToasts((prev) =>
        !options.action && prev.some((t) => t.message === message)
          ? prev.map((t) =>
              t.message === message
                ? { ...t, settings: t.settings ?? settings, action: t.action ?? action }
                : t,
            )
          : [...prev, { id, message, type, settings, action }],
      );
      const ttl = options.ttl !== undefined ? options.ttl : settings ? null : TOAST_TTL_MS[type];
      if (ttl) {
        setTimeout(
          () => setToasts((prev) => prev.filter((t) => t.id !== id || t.settings)),
          ttl,
        );
      }
    },
    [],
  );

  React.useEffect(() => {
    let unsubs: (() => void)[] = [];
    let cancelled = false;
    import("../../wailsjs/go/main/App").then(({ IsMiniMode }) => {
      IsMiniMode().then(setIsMiniMode);
    });
    import("../../wailsjs/runtime/runtime").then(({ EventsOn }) => {
      if (cancelled) return;
      unsubs = [
        EventsOn(Events.MiniMode, (isMini: boolean) => setIsMiniMode(isMini)),
        EventsOn(
          Events.Toast,
          (d: { message: string; type?: Toast["type"]; settings?: PrivacyPane }) =>
            showToast(d.message, d.type ?? "error", { settings: d.settings }),
        ),
      ];
    });
    return () => {
      cancelled = true;
      unsubs.forEach((u) => u());
    };
  }, [showToast]);

  const value = useMemo(() => ({ showToast }), [showToast]);
  const listValue = useMemo(
    () => ({ toasts, dismissToast, clearToasts }),
    [toasts, dismissToast, clearToasts],
  );

  return (
    <ToastContext.Provider value={value}>
      <ToastListContext.Provider value={listValue}>
        {children}
      </ToastListContext.Provider>

      {!isMiniMode && (
        <div
          role="status"
          aria-live="polite"
          className="fixed top-4 right-4 z-[100] flex flex-col gap-2 pointer-events-none"
        >
          {toasts.map((toast) => (
            <div
              key={toast.id}
              role={toast.type === "error" ? "alert" : undefined}
              className={`pointer-events-auto max-w-sm p-4 rounded-xl shadow-soft-md animate-scale-in border text-text ${TONE[toast.type]}`}
            >
              <div className="flex items-start gap-3">
                <svg
                  className="w-5 h-5 flex-shrink-0"
                  fill="none"
                  stroke="currentColor"
                  viewBox="0 0 24 24"
                  strokeWidth={2}
                  aria-hidden="true"
                >
                  <path strokeLinecap="round" strokeLinejoin="round" d={ICONS[toast.type]} />
                </svg>

                <div className="flex-1 min-w-0">
                  <p className="text-[13px] font-medium">{toast.message}</p>
                  {toast.action && (
                    <button
                      type="button"
                      className="btn-secondary !py-1 !px-2.5 !text-xs mt-2"
                      onClick={() => {
                        toast.action!.onClick();
                        dismissToast(toast.id);
                      }}
                    >
                      {toast.action.label}
                    </button>
                  )}
                </div>

                <button
                  type="button"
                  onClick={() => dismissToast(toast.id)}
                  aria-label="Dismiss"
                  className="flex-shrink-0 opacity-70 hover:opacity-100 transition-opacity"
                >
                  <svg
                    className="w-4 h-4"
                    fill="none"
                    stroke="currentColor"
                    viewBox="0 0 24 24"
                    strokeWidth={2}
                    aria-hidden="true"
                  >
                    <path strokeLinecap="round" strokeLinejoin="round" d="M6 18L18 6M6 6l12 12" />
                  </svg>
                </button>
              </div>
            </div>
          ))}
        </div>
      )}
    </ToastContext.Provider>
  );
}

export function useToast() {
  const context = useContext(ToastContext);
  if (!context) {
    throw new Error("useToast must be used within a ToastProvider");
  }
  return context;
}

export function useToastList() {
  const context = useContext(ToastListContext);
  if (!context) {
    throw new Error("useToastList must be used within a ToastProvider");
  }
  return context;
}
