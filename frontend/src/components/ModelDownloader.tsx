import { useEffect } from "react";
import { IsModelReady } from "../../wailsjs/go/main/App";
import { useModelDownload } from "../hooks/useModelDownload";

interface Props {
  onDownloadComplete: () => void;
}

export default function ModelDownloader({ onDownloadComplete }: Props) {
  const { downloading, error, start, percent } =
    useModelDownload(onDownloadComplete);

  useEffect(() => {
    IsModelReady().then((ready) => {
      if (ready) onDownloadComplete();
    });
  }, []);

  return (
    <div className="min-h-screen app-shell flex items-center justify-center p-8">
      <div className="max-w-md w-full text-center">
        <div className="w-20 h-20 mx-auto mb-8 rounded-2xl bg-primary flex items-center justify-center shadow-soft-md">
          <svg
            className="w-10 h-10 text-[var(--primary-foreground)]"
            fill="currentColor"
            viewBox="0 0 24 24"
          >
            <path d="M12 14c1.66 0 3-1.34 3-3V5c0-1.66-1.34-3-3-3S9 3.34 9 5v6c0 1.66 1.34 3 3 3z" />
            <path d="M17 11c0 2.76-2.24 5-5 5s-5-2.24-5-5H5c0 3.53 2.61 6.43 6 6.92V21h2v-3.08c3.39-.49 6-3.39 6-6.92h-2z" />
          </svg>
        </div>

        <h1 className="text-2xl font-bold text-text mb-2">
          Welcome to VoxFlow
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
                VoxFlow needs to download a speech recognition model (~490 MB
                for the English model). The model runs completely offline on your
                device for maximum privacy.
              </p>
            </div>

            <button onClick={start} className="btn-primary w-full">
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

              <div className="progress-bar">
                <div style={{ width: `${percent}%` }} />
              </div>

              <p className="text-sm text-tertiary font-bold mt-2">{percent}%</p>
            </div>

            <p className="text-xs text-tertiary font-medium">
              This may take a few minutes depending on your connection.
            </p>
          </div>
        )}

        {error && (
          <div className="space-y-4">
            <div
              role="alert"
              className="p-4 bg-red-500/10 border border-red-500/30 rounded-xl"
            >
              <p className="text-sm font-medium text-[var(--danger)]">{error}</p>
            </div>

            <button onClick={start} className="btn-secondary w-full">
              Retry Download
            </button>
          </div>
        )}
      </div>
    </div>
  );
}
