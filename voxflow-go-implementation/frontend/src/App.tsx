import { useState, useEffect } from "react";
import "./style.css";
import MainView from "./components/MainView";
import HistoryView from "./components/HistoryView";
import SettingsView from "./components/SettingsView";
import ModelDownloader from "./components/ModelDownloader";
import RecordingIndicator from "./components/RecordingIndicator";
import RecordingPill from "./components/RecordingPill";
import { ThemeProvider, useTheme } from "./contexts/ThemeContext";
import { ToastProvider, useToast } from "./contexts/ToastContext";
import { EventsOn, Quit } from "../wailsjs/runtime/runtime";
import { IsMiniMode, ShowMiniMode } from "../wailsjs/go/main/App";

type View = "main" | "history" | "settings";

// Icons as components for cleaner code
const MicIcon = () => (
  <svg className="w-5 h-5" fill="currentColor" viewBox="0 0 24 24">
    <path d="M12 14c1.66 0 3-1.34 3-3V5c0-1.66-1.34-3-3-3S9 3.34 9 5v6c0 1.66 1.34 3 3 3z" />
    <path d="M17 11c0 2.76-2.24 5-5 5s-5-2.24-5-5H5c0 3.53 2.61 6.43 6 6.92V21h2v-3.08c3.39-.49 6-3.39 6-6.92h-2z" />
  </svg>
);

const HistoryIcon = () => (
  <svg
    className="w-5 h-5"
    fill="none"
    stroke="currentColor"
    viewBox="0 0 24 24"
    strokeWidth={1.5}
  >
    <path
      strokeLinecap="round"
      strokeLinejoin="round"
      d="M12 6v6h4.5m4.5 0a9 9 0 11-18 0 9 9 0 0118 0z"
    />
  </svg>
);

const SettingsIcon = () => (
  <svg
    className="w-5 h-5"
    fill="none"
    stroke="currentColor"
    viewBox="0 0 24 24"
    strokeWidth={1.5}
  >
    <path
      strokeLinecap="round"
      strokeLinejoin="round"
      d="M9.594 3.94c.09-.542.56-.94 1.11-.94h2.593c.55 0 1.02.398 1.11.94l.213 1.281c.063.374.313.686.645.87.074.04.147.083.22.127.324.196.72.257 1.075.124l1.217-.456a1.125 1.125 0 011.37.49l1.296 2.247a1.125 1.125 0 01-.26 1.431l-1.003.827c-.293.24-.438.613-.431.992a6.759 6.759 0 010 .255c-.007.378.138.75.43.99l1.005.828c.424.35.534.954.26 1.43l-1.298 2.247a1.125 1.125 0 01-1.369.491l-1.217-.456c-.355-.133-.75-.072-1.076.124a6.57 6.57 0 01-.22.128c-.331.183-.581.495-.644.869l-.213 1.28c-.09.543-.56.941-1.11.941h-2.594c-.55 0-1.02-.398-1.11-.94l-.213-1.281c-.062-.374-.312-.686-.644-.87a6.52 6.52 0 01-.22-.127c-.325-.196-.72-.257-1.076-.124l-1.217.456a1.125 1.125 0 01-1.369-.49l-1.297-2.247a1.125 1.125 0 01.26-1.431l1.004-.827c.292-.24.437-.613.43-.992a6.932 6.932 0 010-.255c.007-.378-.138-.75-.43-.99l-1.004-.828a1.125 1.125 0 01-.26-1.43l1.297-2.247a1.125 1.125 0 011.37-.491l1.216.456c.356.133.751.072 1.076-.124.072-.044.146-.087.22-.128.332-.183.582-.495.644-.869l.214-1.281z"
    />
    <path
      strokeLinecap="round"
      strokeLinejoin="round"
      d="M15 12a3 3 0 11-6 0 3 3 0 016 0z"
    />
  </svg>
);

const SunIcon = () => (
  <svg
    className="w-5 h-5"
    fill="none"
    stroke="currentColor"
    viewBox="0 0 24 24"
    strokeWidth={1.5}
  >
    <path
      strokeLinecap="round"
      strokeLinejoin="round"
      d="M12 3v2.25m6.364.386l-1.591 1.591M21 12h-2.25m-.386 6.364l-1.591-1.591M12 18.75V21m-4.773-4.227l-1.591 1.591M5.25 12H3m4.227-4.773L5.636 5.636M15.75 12a3.75 3.75 0 11-7.5 0 3.75 3.75 0 017.5 0z"
    />
  </svg>
);

const MoonIcon = () => (
  <svg
    className="w-5 h-5"
    fill="none"
    stroke="currentColor"
    viewBox="0 0 24 24"
    strokeWidth={1.5}
  >
    <path
      strokeLinecap="round"
      strokeLinejoin="round"
      d="M21.752 15.002A9.718 9.718 0 0118 15.75c-5.385 0-9.75-4.365-9.75-9.75 0-1.33.266-2.597.748-3.752A9.753 9.753 0 003 11.25C3 16.635 7.365 21 12.75 21a9.753 9.753 0 009.002-5.998z"
    />
  </svg>
);

const MinimizeIcon = () => (
  <svg
    className="w-5 h-5"
    fill="none"
    stroke="currentColor"
    viewBox="0 0 24 24"
    strokeWidth={1.5}
  >
    <path
      strokeLinecap="round"
      strokeLinejoin="round"
      d="M9 9V4.5M9 9H4.5M9 9L3.75 3.75M9 15v4.5M9 15H4.5M9 15l-5.25 5.25M15 9h4.5M15 9V4.5M15 9l5.25-5.25M15 15h4.5M15 15v4.5m0-4.5l5.25 5.25"
    />
  </svg>
);

const CloseIcon = () => (
  <svg
    className="w-5 h-5"
    fill="none"
    stroke="currentColor"
    viewBox="0 0 24 24"
    strokeWidth={1.5}
  >
    <path
      strokeLinecap="round"
      strokeLinejoin="round"
      d="M6 18L18 6M6 6l12 12"
    />
  </svg>
);

function AppContent() {
  const [currentView, setCurrentView] = useState<View>("main");
  const [modelReady, setModelReady] = useState<boolean>(false);
  const [modelDownloading, setModelDownloading] = useState<boolean>(false);
  const [isMiniMode, setIsMiniMode] = useState<boolean>(true);
  const { theme, toggleTheme } = useTheme();
  const { showToast } = useToast();

  useEffect(() => {
    IsMiniMode().then(setIsMiniMode);

    EventsOn("open-history", () => setCurrentView("history"));
    EventsOn("open-settings", () => setCurrentView("settings"));

    EventsOn("mini-mode", (isMini: boolean) => {
      setIsMiniMode(isMini);
    });

    // Listen for toast events from backend
    EventsOn(
      "toast",
      (data: {
        message: string;
        type: "error" | "warning" | "success" | "info";
      }) => {
        showToast(data.message, data.type);
      }
    );

    EventsOn(
      "model-status",
      (status: { downloaded: boolean; loaded: boolean }) => {
        if (status.downloaded && status.loaded) {
          setModelReady(true);
          setModelDownloading(false);
        } else if (!status.downloaded) {
          setModelReady(false);
          setModelDownloading(false);
        }
      }
    );
  }, [showToast]);

  const handleDownloadStart = () => {
    setModelDownloading(true);
  };

  const handleDownloadComplete = () => {
    setModelReady(true);
    setModelDownloading(false);
  };

  // Mini mode - show only recording indicator
  if (isMiniMode) {
    return <RecordingIndicator />;
  }

  // Model download screen
  if (!modelReady && !modelDownloading) {
    return (
      <ModelDownloader
        onDownloadStart={handleDownloadStart}
        onDownloadComplete={handleDownloadComplete}
      />
    );
  }

  return (
    <div className="min-h-screen flex bg-primary">
      {/* Recording pill - shows at top when recording in full app mode */}
      <RecordingPill />

      {/* Sidebar */}
      <aside className="sidebar titlebar">
        {/* Top section - Traffic lights space + main nav */}
        <div className="flex flex-col items-center gap-2 pt-8">
          {/* Spacer for macOS traffic lights */}
          <div className="h-4" />

          {/* Main navigation */}
          <button
            onClick={() => setCurrentView("main")}
            className={`sidebar-btn no-drag ${
              currentView === "main" ? "active" : ""
            }`}
            title="Dictation"
          >
            <MicIcon />
          </button>

          <button
            onClick={() => setCurrentView("history")}
            className={`sidebar-btn no-drag ${
              currentView === "history" ? "active" : ""
            }`}
            title="History"
          >
            <HistoryIcon />
          </button>

          <button
            onClick={() => setCurrentView("settings")}
            className={`sidebar-btn no-drag ${
              currentView === "settings" ? "active" : ""
            }`}
            title="Settings"
          >
            <SettingsIcon />
          </button>
        </div>

        {/* Spacer */}
        <div className="flex-1" />

        {/* Bottom section - Theme, minimize, close */}
        <div className="flex flex-col items-center gap-2 pb-4">
          <button
            onClick={toggleTheme}
            className="sidebar-btn no-drag"
            title={`Switch to ${theme === "dark" ? "light" : "dark"} theme`}
          >
            {theme === "dark" ? <SunIcon /> : <MoonIcon />}
          </button>

          <button
            onClick={() => ShowMiniMode()}
            className="sidebar-btn no-drag"
            title="Minimize to floating indicator"
          >
            <MinimizeIcon />
          </button>

          <button
            onClick={() => Quit()}
            className="sidebar-btn no-drag hover:!bg-red-500/10 hover:!text-red-500"
            title="Quit voxflow"
          >
            <CloseIcon />
          </button>
        </div>
      </aside>

      {/* Main content */}
      <main className="flex-1 overflow-hidden relative">
        <div className="view-enter h-full">
          {currentView === "main" && <MainView />}
          {currentView === "history" && <HistoryView />}
          {currentView === "settings" && <SettingsView />}
        </div>
      </main>

      {/* Manual Resize Grip (Bottom Right) */}
      {!isMiniMode && (
        <div
          className="absolute bottom-0 right-0 w-6 h-6 z-50 cursor-se-resize flex items-end justify-end p-1 no-drag group"
          onMouseDown={(e) => {
            e.preventDefault();
            const startX = e.screenX;
            const startY = e.screenY;

            // Get current size
            import("../wailsjs/runtime/runtime").then(
              ({ WindowGetSize, WindowSetSize }) => {
                WindowGetSize().then((size) => {
                  const startW = size.w;
                  const startH = size.h;

                  const onMouseMove = (ev: MouseEvent) => {
                    const newW = Math.max(400, startW + (ev.screenX - startX)); // Min width 400
                    const newH = Math.max(300, startH + (ev.screenY - startY)); // Min height 300
                    WindowSetSize(newW, newH);
                  };

                  const onMouseUp = () => {
                    window.removeEventListener("mousemove", onMouseMove);
                    window.removeEventListener("mouseup", onMouseUp);
                  };

                  window.addEventListener("mousemove", onMouseMove);
                  window.addEventListener("mouseup", onMouseUp);
                });
              }
            );
          }}
        >
          {/* Visual Grip Lines */}
          <svg
            className="w-4 h-4 text-secondary opacity-50 group-hover:opacity-100 transition-opacity"
            viewBox="0 0 24 24"
            fill="currentColor"
          >
            <path d="M22 22H20V20H22V22ZM22 18H18V20H22V18ZM18 22H16V20H18V22ZM14 22H12V20H14V22ZM22 14H20V16H22V14Z" />
          </svg>
        </div>
      )}
    </div>
  );
}

function App() {
  return (
    <ThemeProvider>
      <ToastProvider>
        <AppContent />
      </ToastProvider>
    </ThemeProvider>
  );
}

export default App;
