export {};

// Define the Wails runtime type for the App
declare global {
  interface Window {
    go: any;
  }
}

// Fallback logger if Wails runtime isn't ready
const safeLog = (
  message: string,
  level: "INFO" | "ERROR" | "WARN" | "DEBUG" = "DEBUG"
) => {
  // Always log to browser console
  const timestamp = new Date().toISOString().split("T")[1].slice(0, -1);
  const formattedMsg = `[${timestamp}] [${level}] ${message}`;

  if (level === "ERROR") console.error(formattedMsg);
  else if (level === "WARN") console.warn(formattedMsg);
  else console.log(formattedMsg);

  // Try to log to backend
  try {
    if (
      window.go &&
      window.go.main &&
      window.go.main.App &&
      window.go.main.App.Log
    ) {
      window.go.main.App.Log(level, message);
    }
  } catch (e) {
    // Ignore errors sending to backend
  }
};

export const Logger = {
  info: (...args: any[]) =>
    safeLog(args.map((a) => String(a)).join(" "), "INFO"),
  error: (...args: any[]) =>
    safeLog(args.map((a) => String(a)).join(" "), "ERROR"),
  warn: (...args: any[]) =>
    safeLog(args.map((a) => String(a)).join(" "), "WARN"),
  debug: (...args: any[]) =>
    safeLog(args.map((a) => String(a)).join(" "), "DEBUG"),
};
