import { useState, useEffect, useRef, type CSSProperties } from "react";
import "./style.css";
import MainView from "./components/MainView";
import HistoryView from "./components/HistoryView";
import SettingsView from "./components/SettingsView";
import ModelDownloader from "./components/ModelDownloader";
import RecordingIndicator from "./components/RecordingIndicator";
import RecordingPill from "./components/RecordingPill";
import { ThemeProvider } from "./contexts/ThemeContext";
import { ToastProvider, useToast } from "./contexts/ToastContext";
import { EventsOn } from "../wailsjs/runtime/runtime";
import {
  GetOnboardingCompleted,
  IsMiniMode,
  IsModelReady,
  ShowMiniMode,
} from "../wailsjs/go/main/App";
import OnboardingWizard from "./components/OnboardingWizard";
import { Events } from "./constants/events";

type View = "main" | "history" | "settings";

const DRAG = { "--wails-draggable": "drag" } as CSSProperties;

const MicIcon = () => (
  <svg fill="currentColor" viewBox="0 0 24 24">
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

function AppContent() {
  const [currentView, setCurrentView] = useState<View>("main");
  const [settingsTab, setSettingsTab] = useState<string | undefined>();
  const [modelReady, setModelReady] = useState<boolean>(false);
  const [isMiniMode, setIsMiniMode] = useState<boolean>(true);
  const [onboardingDone, setOnboardingDone] = useState<boolean | null>(null);

  const { showToast } = useToast();
  const showToastRef = useRef(showToast);
  showToastRef.current = showToast;

  // Wails mini mode needs inline transparent bg; full mode clears so theme CSS applies.
  const setMiniModeTransparency = (enabled: boolean) => {
    if (enabled) {
      document.documentElement.style.backgroundColor = "transparent";
      document.body.style.backgroundColor = "transparent";
      const root = document.getElementById("root");
      if (root) root.style.backgroundColor = "transparent";
    } else {
      document.documentElement.style.backgroundColor = "";
      document.body.style.backgroundColor = "";
      const root = document.getElementById("root");
      if (root) root.style.backgroundColor = "";
    }
  };

  useEffect(() => {
    IsMiniMode().then((isMini) => {
      setIsMiniMode(isMini);
    });
    GetOnboardingCompleted().then(setOnboardingDone);
    // The startup ModelStatus event can fire before this listener exists.
    IsModelReady().then((ready) => ready && setModelReady(true));

    const unsub1 = EventsOn(Events.OpenHistory, () => setCurrentView("history"));
    const unsub2 = EventsOn(Events.OpenSettings, (tab?: string) => {
      setSettingsTab(typeof tab === "string" ? tab : undefined);
      setCurrentView("settings");
    });
    const unsubHome = EventsOn(Events.OpenHome, () => setCurrentView("main"));
    const unsub3 = EventsOn(Events.MiniMode, (isMini: boolean) => {
      setIsMiniMode(isMini);
    });

    const unsub4 = EventsOn(
      Events.Toast,
      (data: {
        message: string;
        type: "error" | "warning" | "success" | "info";
      }) => {
        showToastRef.current(data.message, data.type);
      },
    );

    const unsubError = EventsOn(Events.Error, (message: string) => {
      showToastRef.current(String(message), "error");
    });

    const unsub5 = EventsOn(
      Events.ModelStatus,
      (status: { downloaded: boolean; loaded?: boolean; error?: string }) => {
        if (status.downloaded && status.loaded) setModelReady(true);
        else if (!status.downloaded) setModelReady(false);
        if (status.error) showToastRef.current(status.error, "error");
      },
    );

    return () => {
      unsub1();
      unsub2();
      unsub3();
      unsub4();
      unsub5();
      unsubError();
      unsubHome();
    };
  }, []);

  // Re-apply when isMiniMode changes; no polling loop.
  useEffect(() => {
    setMiniModeTransparency(isMiniMode);
    return () => setMiniModeTransparency(false);
  }, [isMiniMode]);

  if (onboardingDone === false && !isMiniMode) {
    return (
      <OnboardingWizard onComplete={() => setOnboardingDone(true)} />
    );
  }

  if (isMiniMode) {
    return <RecordingIndicator />;
  }

  if (!modelReady) {
    return <ModelDownloader onDownloadComplete={() => setModelReady(true)} />;
  }

  const navItem = (view: View, label: string, icon: JSX.Element) => (
    <button
      type="button"
      onClick={() => setCurrentView(view)}
      aria-current={currentView === view ? "page" : undefined}
      className="sidebar-item no-drag"
    >
      {icon}
      {label}
    </button>
  );

  return (
    <div className="h-full min-h-0 flex app-shell">
      <RecordingPill />

      <aside className="sidebar">
        <div className="sidebar-titlebar" style={DRAG} />
        <nav aria-label="VoxFlow" className="flex flex-col gap-0.5">
          {navItem("main", "Home", <MicIcon />)}
          {navItem("history", "History", <HistoryIcon />)}
          {navItem("settings", "Settings", <SettingsIcon />)}
        </nav>
        <div className="flex-1" style={DRAG} />
        <button
          type="button"
          onClick={() => ShowMiniMode()}
          className="sidebar-item no-drag text-secondary"
        >
          <MinimizeIcon />
          Collapse to pill
        </button>
      </aside>

      <main className="flex-1 min-w-0 overflow-hidden relative">
        <div className="absolute inset-x-0 top-0 h-3 z-10" style={DRAG} />
        <div className="view-enter h-full">
          {currentView === "main" && <MainView />}
          {currentView === "history" && <HistoryView />}
          {currentView === "settings" && <SettingsView initialTab={settingsTab} />}
        </div>
      </main>
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
