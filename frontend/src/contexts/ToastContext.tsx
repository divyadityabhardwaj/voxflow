import React, { createContext, useContext, useState, useCallback, useMemo, useRef } from "react";

export interface Toast {
  id: number;
  message: string;
  type: "error" | "warning" | "success" | "info";
}

interface ToastContextType {
  showToast: (message: string, type?: Toast["type"]) => void;
}

interface ToastListContextType {
  toasts: Toast[];
  dismissToast: (id: number) => void;
  clearToasts: () => void;
}

// Errors stay until dismissed so they can still be read after opening the window.
const TOAST_TTL_MS: Record<Toast["type"], number | null> = {
  error: null,
  warning: 6000,
  success: 3000,
  info: 3000,
};

const ToastContext = createContext<ToastContextType | null>(null);
// Separate so showToast callers don't re-render on every toast.
const ToastListContext = createContext<ToastListContextType | null>(null);

import { Events } from "../constants/events";

export function ToastProvider({ children }: { children: React.ReactNode }) {
  const [toasts, setToasts] = useState<Toast[]>([]);
  const [isMiniMode, setIsMiniMode] = useState(false);

  React.useEffect(() => {
    import("../../wailsjs/go/main/App").then(({ IsMiniMode }) => {
      IsMiniMode().then(setIsMiniMode);
    });

    let unsub: (() => void) | undefined;
    import("../../wailsjs/runtime/runtime").then(({ EventsOn }) => {
      unsub = EventsOn(Events.MiniMode, (isMini: boolean) => {
        setIsMiniMode(isMini);
      });
    });

    return () => {
      if (unsub) unsub();
    };
  }, []);

  const nextId = useRef(0);

  const dismissToast = useCallback((id: number) => {
    setToasts((prev) => prev.filter((t) => t.id !== id));
  }, []);

  const clearToasts = useCallback(() => setToasts([]), []);

  const showToast = useCallback(
    (message: string, type: Toast["type"] = "error") => {
      const id = nextId.current++;
      setToasts((prev) =>
        prev.some((t) => t.message === message)
          ? prev
          : [...prev, { id, message, type }],
      );
      const ttl = TOAST_TTL_MS[type];
      if (ttl) setTimeout(() => dismissToast(id), ttl);
    },
    [dismissToast],
  );

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
              className={`
                pointer-events-auto max-w-sm p-4 rounded-xl shadow-soft-md animate-scale-in border
                ${toast.type === "error" ? "bg-red-500/10 text-text border-red-500/30" : ""}
                ${toast.type === "warning" ? "bg-amber-500/10 text-text border-amber-500/30" : ""}
                ${
                  toast.type === "success" ? "bg-emerald-500/10 text-text border-emerald-500/30" : ""
                }
                ${toast.type === "info" ? "bg-blue-500/10 text-text border-blue-500/30" : ""}
              `}
            >
              <div className="flex items-start gap-3">
                <div className="flex-shrink-0">
                  {toast.type === "error" && (
                    <svg
                      className="w-5 h-5"
                      fill="none"
                      stroke="currentColor"
                      viewBox="0 0 24 24"
                      strokeWidth={2}
                    >
                      <path
                        strokeLinecap="round"
                        strokeLinejoin="round"
                        d="M12 8v4m0 4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z"
                      />
                    </svg>
                  )}
                  {toast.type === "warning" && (
                    <svg
                      className="w-5 h-5"
                      fill="none"
                      stroke="currentColor"
                      viewBox="0 0 24 24"
                      strokeWidth={2}
                    >
                      <path
                        strokeLinecap="round"
                        strokeLinejoin="round"
                        d="M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-3L13.732 4c-.77-1.333-2.694-1.333-3.464 0L3.34 16c-.77 1.333.192 3 1.732 3z"
                      />
                    </svg>
                  )}
                  {toast.type === "success" && (
                    <svg
                      className="w-5 h-5"
                      fill="none"
                      stroke="currentColor"
                      viewBox="0 0 24 24"
                      strokeWidth={2}
                    >
                      <path
                        strokeLinecap="round"
                        strokeLinejoin="round"
                        d="M9 12l2 2 4-4m6 2a9 9 0 11-18 0 9 9 0 0118 0z"
                      />
                    </svg>
                  )}
                  {toast.type === "info" && (
                    <svg
                      className="w-5 h-5"
                      fill="none"
                      stroke="currentColor"
                      viewBox="0 0 24 24"
                      strokeWidth={2}
                    >
                      <path
                        strokeLinecap="round"
                        strokeLinejoin="round"
                        d="M13 16h-1v-4h-1m1-4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z"
                      />
                    </svg>
                  )}
                </div>

                <p className="text-[13px] font-medium flex-1">{toast.message}</p>

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
                  >
                    <path
                      strokeLinecap="round"
                      strokeLinejoin="round"
                      d="M6 18L18 6M6 6l12 12"
                    />
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
