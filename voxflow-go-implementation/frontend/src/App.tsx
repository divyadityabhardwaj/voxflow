import { useState, useEffect } from "react";
import "./style.css";
import MainView from "./components/MainView";
import HistoryView from "./components/HistoryView";
import SettingsView from "./components/SettingsView";
import ModelDownloader from "./components/ModelDownloader";
import RecordingIndicator from "./components/RecordingIndicator";
import { EventsOn, Quit } from "../wailsjs/runtime/runtime";
import { IsMiniMode, ShowMiniMode } from "../wailsjs/go/main/App";

type View = "main" | "history" | "settings";

function App() {
  const [currentView, setCurrentView] = useState<View>("main");
  const [modelReady, setModelReady] = useState<boolean>(false);
  const [modelDownloading, setModelDownloading] = useState<boolean>(false);
  const [isMiniMode, setIsMiniMode] = useState<boolean>(true); // Default to mini mode

  useEffect(() => {
    // Check initial mini-mode state
    IsMiniMode().then(setIsMiniMode);

    // Listen for navigation events from Go backend
    EventsOn("open-history", () => setCurrentView("history"));
    EventsOn("open-settings", () => setCurrentView("settings"));

    // Listen for mini-mode changes
    EventsOn("mini-mode", (isMini: boolean) => {
      setIsMiniMode(isMini);
    });

    // Listen for model status
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
  }, []);

  const handleDownloadStart = () => {
    setModelDownloading(true);
  };

  const handleDownloadComplete = () => {
    setModelReady(true);
    setModelDownloading(false);
  };

  // If in mini mode, show only the recording indicator
  if (isMiniMode) {
    return <RecordingIndicator />;
  }

  // Show model downloader if model is not ready
  if (!modelReady && !modelDownloading) {
    return (
      <ModelDownloader
        onDownloadStart={handleDownloadStart}
        onDownloadComplete={handleDownloadComplete}
      />
    );
  }

  return (
    <div className="min-h-screen bg-dark-950 text-dark-100">
      {/* Navigation */}
      <nav className="titlebar h-12 bg-dark-900/80 backdrop-blur-md border-b border-dark-800 flex items-center justify-between px-4 sticky top-0 z-50">
        <div className="flex items-center gap-2">
          <div className="w-20" /> {/* Space for macOS traffic lights */}
          <h1 className="text-sm font-semibold text-dark-200">voxflow</h1>
        </div>

        <div className="flex items-center gap-1">
          <button
            onClick={() => setCurrentView("main")}
            className={`px-3 py-1.5 text-xs rounded-md transition-all ${
              currentView === "main"
                ? "bg-accent-600 text-white"
                : "text-dark-400 hover:text-dark-200 hover:bg-dark-800"
            }`}
          >
            Dictation
          </button>
          <button
            onClick={() => setCurrentView("history")}
            className={`px-3 py-1.5 text-xs rounded-md transition-all ${
              currentView === "history"
                ? "bg-accent-600 text-white"
                : "text-dark-400 hover:text-dark-200 hover:bg-dark-800"
            }`}
          >
            History
          </button>
          <button
            onClick={() => setCurrentView("settings")}
            className={`px-3 py-1.5 text-xs rounded-md transition-all ${
              currentView === "settings"
                ? "bg-accent-600 text-white"
                : "text-dark-400 hover:text-dark-200 hover:bg-dark-800"
            }`}
          >
            Settings
          </button>

          {/* Separator */}
          <div className="w-px h-4 bg-dark-700 mx-1" />

          {/* Minimize to indicator */}
          <button
            onClick={() => ShowMiniMode()}
            className="p-1.5 text-dark-400 hover:text-dark-200 hover:bg-dark-800 rounded-md transition-all"
            title="Minimize to floating indicator"
          >
            <svg
              className="w-4 h-4"
              fill="none"
              stroke="currentColor"
              viewBox="0 0 24 24"
            >
              <path
                strokeLinecap="round"
                strokeLinejoin="round"
                strokeWidth={2}
                d="M20 12H4"
              />
            </svg>
          </button>

          {/* Close */}
          <button
            onClick={() => Quit()}
            className="p-1.5 text-dark-400 hover:text-red-400 hover:bg-red-900/20 rounded-md transition-all"
            title="Quit voxflow"
          >
            <svg
              className="w-4 h-4"
              fill="none"
              stroke="currentColor"
              viewBox="0 0 24 24"
            >
              <path
                strokeLinecap="round"
                strokeLinejoin="round"
                strokeWidth={2}
                d="M6 18L18 6M6 6l12 12"
              />
            </svg>
          </button>
        </div>
      </nav>

      {/* Content */}
      <main className="flex-1">
        {currentView === "main" && <MainView />}
        {currentView === "history" && <HistoryView />}
        {currentView === "settings" && <SettingsView />}
      </main>
    </div>
  );
}

export default App;
