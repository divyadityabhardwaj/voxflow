import React, { createContext, useContext, useState, useCallback, useMemo } from "react";

interface Toast {
  id: number;
  message: string;
  type: "error" | "warning" | "success" | "info";
}

interface ToastContextType {
  showToast: (message: string, type?: Toast["type"]) => void;
}

const ToastContext = createContext<ToastContextType | null>(null);

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

  let nextId = 0;

  const showToast = useCallback(
    (message: string, type: Toast["type"] = "error") => {
      setToasts((prev) => {
        if (prev.some((t) => t.message === message)) {
          return prev;
        }

        let finalMessage = message;
        if (isMiniMode && message.includes("No speech detected")) {
          finalMessage = "No Speech Detected";
        }

        const id = nextId++;
        const newToasts = [...prev, { id, message: finalMessage, type }];

        setTimeout(() => {
          setToasts((current) => current.filter((t) => t.id !== id));
        }, 3000);

        return newToasts;
      });
    },
    [isMiniMode],
  );

  const dismissToast = useCallback((id: number) => {
    setToasts((prev) => prev.filter((t) => t.id !== id));
  }, []);

  const value = useMemo(() => ({ showToast }), [showToast]);

  return (
    <ToastContext.Provider value={value}>
      {children}

      {!isMiniMode && (
        /* STANDARD TOAST CONTAINER */
        <div className="fixed top-4 right-4 z-[100] flex flex-col gap-2 pointer-events-none">
          {toasts.map((toast) => (
            <div
              key={toast.id}
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

                <p className="text-sm font-bold flex-1">{toast.message}</p>

                <button
                  onClick={() => dismissToast(toast.id)}
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
