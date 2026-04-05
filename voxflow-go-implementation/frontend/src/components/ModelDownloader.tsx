import { useState, useEffect } from "react";
import { IsModelDownloaded, DownloadModel } from "../../wailsjs/go/main/App";
import { EventsOn } from "../../wailsjs/runtime/runtime";
import { Events } from "../constants/events";

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
    IsModelDownloaded().then((downloaded) => {
      if (downloaded) {
        onDownloadComplete();
      }
    });

    EventsOn(Events.ModelDownloadProgress, (data: { progress: number }) => {
      setProgress(Math.round(data.progress));
    });

    EventsOn(Events.ModelDownloadError, (err: string) => {
      setError(err);
      setDownloading(false);
    });

    EventsOn(
      Events.ModelStatus,
      (status: { downloaded: boolean; loaded: boolean }) => {
        if (status.downloaded && status.loaded) {
          onDownloadComplete();
        }
      },
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
    <div className="min-h-screen bg-background flex items-center justify-center p-8">
      <div className="max-w-md w-full text-center">
        {/* Logo/Icon */}
        <div className="w-20 h-20 mx-auto mb-8 rounded-2xl bg-primary flex items-center justify-center shadow-soft-md">
          <svg
            className="w-10 h-10 text-white"
            fill="currentColor"
            viewBox="0 0 24 24"
          >
            <path d="M12 14c1.66 0 3-1.34 3-3V5c0-1.66-1.34-3-3-3S9 3.34 9 5v6c0 1.66 1.34 3 3 3z" />
            <path d="M17 11c0 2.76-2.24 5-5 5s-5-2.24-5-5H5c0 3.53 2.61 6.43 6 6.92V21h2v-3.08c3.39-.49 6-3.39 6-6.92h-2z" />
          </svg>
        </div>

        <h1 className="text-2xl font-bold text-text mb-2">
          Welcome to voxflow
        </h1>
        <p className="text-secondary font-medium mb-8">
          AI-powered voice dictation that runs locally on your device.
        </p>

        {!downloading && !error && (
          <>
            <div className="p-4 bg-secondary rounded-xl border border-border mb-6 text-left">
              <h3 className="text-sm font-bold text-text mb-2">
                Before you start
              </h3>
              <p className="text-sm text-secondary font-medium">
                voxflow needs to download a speech recognition model (~142 MB
                for Base model). The model runs completely offline on your
                device for maximum privacy.
              </p>
            </div>

            <button
              onClick={handleDownload}
              className="btn-primary w-full"
            >
              Download Model & Get Started
            </button>
          </>
        )}

        {downloading && (
          <div className="space-y-4">
            <div className="p-4 bg-secondary rounded-xl border border-border">
              <p className="text-sm text-secondary font-medium mb-3">
                Downloading Whisper model...
              </p>

              {/* Progress bar */}
              <div className="progress-bar">
                <div
                  style={{ width: `${progress}%` }}
                />
              </div>

              <p className="text-sm text-tertiary font-bold mt-2">{progress}%</p>
            </div>

            <p className="text-xs text-tertiary font-medium">
              This may take a few minutes depending on your connection.
            </p>
          </div>
        )}

        {error && (
          <div className="space-y-4">
            <div className="p-4 bg-red-500/10 border border-red-500 rounded-xl">
              <p className="text-sm font-bold text-red-500">{error}</p>
            </div>

            <button
              onClick={handleDownload}
              className="btn-secondary w-full"
            >
              Retry Download
            </button>
          </div>
        )}
      </div>
    </div>
  );
}
