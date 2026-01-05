import React, { createContext, useContext, useState, useCallback } from "react";

interface Toast {
  id: number;
  message: string;
  type: "error" | "warning" | "success" | "info";
}

interface ToastContextType {
  showToast: (message: string, type?: Toast["type"]) => void;
}

const ToastContext = createContext<ToastContextType | null>(null);

export function ToastProvider({ children }: { children: React.ReactNode }) {
  const [toasts, setToasts] = useState<Toast[]>([]);
  const [isMiniMode, setIsMiniMode] = useState(false);

  // Use a ref to track if we've initialized listeners to prevent duplicate registration
  // in strict mode dev environments, though typically useEffect dependency array handles this.

  React.useEffect(() => {
    // Check initial state
    import("../../wailsjs/go/main/App").then(({ IsMiniMode }) => {
      IsMiniMode().then(setIsMiniMode);
    });

    // Listen for mode changes
    import("../../wailsjs/runtime/runtime").then(({ EventsOn }) => {
      EventsOn("mini-mode", (isMini: boolean) => {
        setIsMiniMode(isMini);
      });
    });
  }, []);

  let nextId = 0;

  const showToast = useCallback(
    (message: string, type: Toast["type"] = "error") => {
      // Deduplicate: Don't show if same message already exists
      setToasts((prev) => {
        if (prev.some((t) => t.message === message)) {
          return prev;
        }

        // Mini-mode text shortening
        let finalMessage = message;
        if (isMiniMode && message.includes("No speech detected")) {
          finalMessage = "No Speech Detected";
        }

        const id = nextId++;
        const newToasts = [...prev, { id, message: finalMessage, type }];

        // Auto-dismiss
        setTimeout(() => {
          setToasts((current) => current.filter((t) => t.id !== id));
        }, 3000);

        return newToasts;
      });
    },
    [isMiniMode]
  );

  const dismissToast = useCallback((id: number) => {
    setToasts((prev) => prev.filter((t) => t.id !== id));
  }, []);

  return (
    <ToastContext.Provider value={{ showToast }}>
      {children}

      {/* MINI MODE TOAST CONTAINER */}
      {isMiniMode ? (
        <div className="fixed inset-0 z-[200] flex items-center justify-center pointer-events-none p-1">
          {toasts.length > 0 && (
            <div
              key={toasts[toasts.length - 1].id} // Only show latest
              className={`
                 pointer-events-none rounded-full px-3 py-1 shadow-lg animate-fade-in
                 flex items-center gap-2 max-w-full
                 ${
                   toasts[toasts.length - 1].type === "error"
                     ? "bg-red-500 text-white"
                     : ""
                 }
                 ${
                   toasts[toasts.length - 1].type === "warning"
                     ? "bg-amber-500 text-white"
                     : ""
                 }
                 ${
                   toasts[toasts.length - 1].type === "success"
                     ? "bg-emerald-500 text-white"
                     : ""
                 }
                 ${
                   toasts[toasts.length - 1].type === "info"
                     ? "bg-blue-500 text-white"
                     : ""
                 }
               `}
              onClick={() => dismissToast(toasts[toasts.length - 1].id)}
            >
              {/* Tiny Icon */}
              <div className="flex-shrink-0">
                {toasts[toasts.length - 1].type === "warning" && (
                  <svg
                    className="w-3 h-3"
                    fill="none"
                    viewBox="0 0 24 24"
                    stroke="currentColor"
                    strokeWidth={3}
                  >
                    <path
                      strokeLinecap="round"
                      strokeLinejoin="round"
                      d="M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-3L13.732 4c-.77-1.333-2.694-1.333-3.464 0L3.34 16c-.77 1.333.192 3 1.732 3z"
                    />
                  </svg>
                )}
                {toasts[toasts.length - 1].type === "error" && (
                  <svg
                    className="w-3 h-3"
                    fill="none"
                    viewBox="0 0 24 24"
                    stroke="currentColor"
                    strokeWidth={3}
                  >
                    <path
                      strokeLinecap="round"
                      strokeLinejoin="round"
                      d="M12 8v4m0 4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z"
                    />
                  </svg>
                )}
              </div>

              <p className="text-[10px] font-bold truncate max-w-[120px]">
                {toasts[toasts.length - 1].message}
              </p>

              <button className="opacity-70 hover:opacity-100">
                <svg
                  className="w-3 h-3"
                  fill="none"
                  viewBox="0 0 24 24"
                  stroke="currentColor"
                  strokeWidth={3}
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
        </div>
      ) : (
        /* STANDARD TOAST CONTAINER */
        <div className="fixed top-4 right-4 z-[100] flex flex-col gap-2 pointer-events-none">
          {toasts.map((toast) => (
            <div
              key={toast.id}
              className={`
                pointer-events-auto max-w-sm p-4 rounded-xl shadow-lg
                animate-fade-in backdrop-blur-md
                ${toast.type === "error" ? "bg-red-500/90 text-white" : ""}
                ${toast.type === "warning" ? "bg-amber-500/90 text-white" : ""}
                ${
                  toast.type === "success" ? "bg-emerald-500/90 text-white" : ""
                }
                ${toast.type === "info" ? "bg-blue-500/90 text-white" : ""}
              `}
            >
              <div className="flex items-start gap-3">
                {/* Icon */}
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

                {/* Message */}
                <p className="text-sm font-medium flex-1">{toast.message}</p>

                {/* Dismiss button */}
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
