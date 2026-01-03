import { useState, useEffect } from "react";
import { IsModelDownloaded, DownloadModel } from "../../wailsjs/go/main/App";
import { EventsOn } from "../../wailsjs/runtime/runtime";

interface Props {
  onDownloadStart: () => void;
  onDownloadComplete: () => void;
}

export default function ModelDownloader({
  onDownloadStart,
  onDownloadComplete,
}: Props) {
  const [downloading, setDownloading] = useState(false);
  const [progress, setProgress] = useState(0);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    // Check if already downloaded
    IsModelDownloaded().then((downloaded) => {
      if (downloaded) {
        onDownloadComplete();
      }
    });

    // Listen for download progress
    EventsOn("model-download-progress", (data: { progress: number }) => {
      setProgress(Math.round(data.progress));
    });

    EventsOn("model-download-error", (err: string) => {
      setError(err);
      setDownloading(false);
    });

    EventsOn(
      "model-status",
      (status: { downloaded: boolean; loaded: boolean }) => {
        if (status.downloaded && status.loaded) {
          onDownloadComplete();
        }
      }
    );
  }, []);

  const handleDownload = async () => {
    setDownloading(true);
    setError(null);
    onDownloadStart();

    try {
      await DownloadModel();
    } catch (err) {
      setError(String(err));
      setDownloading(false);
    }
  };

  return (
    <div className="min-h-screen bg-dark-950 flex items-center justify-center p-8">
      <div className="max-w-md w-full text-center">
        {/* Logo/Icon */}
        <div className="w-20 h-20 mx-auto mb-8 rounded-2xl bg-gradient-to-br from-accent-500 to-accent-700 flex items-center justify-center shadow-lg shadow-accent-900/50">
          <svg
            className="w-10 h-10 text-white"
            fill="currentColor"
            viewBox="0 0 24 24"
          >
            <path d="M12 14c1.66 0 3-1.34 3-3V5c0-1.66-1.34-3-3-3S9 3.34 9 5v6c0 1.66 1.34 3 3 3z" />
            <path d="M17 11c0 2.76-2.24 5-5 5s-5-2.24-5-5H5c0 3.53 2.61 6.43 6 6.92V21h2v-3.08c3.39-.49 6-3.39 6-6.92h-2z" />
          </svg>
        </div>

        <h1 className="text-2xl font-bold text-dark-100 mb-2">
          Welcome to voxflow
        </h1>
        <p className="text-dark-400 mb-8">
          AI-powered voice dictation that runs locally on your device.
        </p>

        {!downloading && !error && (
          <>
            <div className="p-4 bg-dark-900 rounded-xl border border-dark-800 mb-6 text-left">
              <h3 className="text-sm font-medium text-dark-200 mb-2">
                Before you start
              </h3>
              <p className="text-sm text-dark-500">
                voxflow needs to download a speech recognition model (~142 MB
                for Base model). The model runs completely offline on your
                device for maximum privacy.
              </p>
            </div>

            <button
              onClick={handleDownload}
              className="w-full py-3 px-6 bg-accent-600 hover:bg-accent-500 text-white 
                       font-medium rounded-xl transition-colors shadow-lg shadow-accent-900/30"
            >
              Download Model & Get Started
            </button>
          </>
        )}

        {downloading && (
          <div className="space-y-4">
            <div className="p-4 bg-dark-900 rounded-xl border border-dark-800">
              <p className="text-sm text-dark-400 mb-3">
                Downloading Whisper model...
              </p>

              {/* Progress bar */}
              <div className="h-2 bg-dark-800 rounded-full overflow-hidden">
                <div
                  className="h-full bg-gradient-to-r from-accent-600 to-accent-400 transition-all duration-300"
                  style={{ width: `${progress}%` }}
                />
              </div>

              <p className="text-sm text-dark-500 mt-2">{progress}%</p>
            </div>

            <p className="text-xs text-dark-600">
              This may take a few minutes depending on your connection.
            </p>
          </div>
        )}

        {error && (
          <div className="space-y-4">
            <div className="p-4 bg-red-900/20 border border-red-800 rounded-xl">
              <p className="text-sm text-red-400">{error}</p>
            </div>

            <button
              onClick={handleDownload}
              className="w-full py-3 px-6 bg-dark-800 hover:bg-dark-700 text-dark-200 
                       font-medium rounded-xl transition-colors"
            >
              Retry Download
            </button>
          </div>
        )}
      </div>
    </div>
  );
}
