import { useEffect, useState } from "react";
import { GetAllModels, GetConfig, IsModelReady } from "../../wailsjs/go/main/App";
import { useModelDownload } from "../hooks/useModelDownload";

interface Props {
  onDownloadComplete: () => void;
}

const MB = 1024 * 1024;

// Shown after onboarding when the speech engine still isn't on this Mac.
export default function ModelDownloader({ onDownloadComplete }: Props) {
  const { downloading, error, start, percent, downloadedMB, totalMB } =
    useModelDownload(onDownloadComplete);
  const [sizeMB, setSizeMB] = useState<number | null>(null);

  useEffect(() => {
    IsModelReady().then((ready) => {
      if (ready) onDownloadComplete();
    });
    Promise.all([GetConfig(), GetAllModels()])
      .then(([cfg, models]) => {
        const m = models.find((x) => x.name === cfg.whisper_model);
        if (m?.size) setSizeMB(Math.round(m.size / MB));
      })
      .catch(() => {});
  }, []);

  return (
    <div className="h-full app-shell flex items-center justify-center p-8">
      <div className="max-w-md w-full card p-8">
        <h1 className="text-xl font-semibold text-text mb-2">
          {downloading ? "Downloading the speech engine" : "Download the speech engine"}
        </h1>
        <p className="text-sm text-secondary mb-6">
          It runs entirely on your Mac, so your voice stays private.
          {sizeMB ? ` It's a one-time download of about ${sizeMB} MB.` : " It's a one-time download."}
        </p>

        {downloading ? (
          <div role="status" aria-live="polite">
            <div className="progress-bar">
              <div style={{ width: `${percent}%` }} />
            </div>
            <p className="text-xs text-secondary mt-2 tabular-nums">
              {percent}%
              {totalMB > 0 && ` (${Math.round(downloadedMB)} of ${Math.round(totalMB)} MB)`}
            </p>
          </div>
        ) : (
          <>
            {error && (
              <p role="alert" className="text-[13px] text-text mb-4 p-3 rounded-lg border border-danger/30 bg-danger/10 break-words">
                Download paused — check your connection. <span className="text-secondary">({error})</span>
              </p>
            )}
            <button type="button" onClick={start} className="btn-primary w-full">
              {error ? "Retry" : "Download"}
            </button>
          </>
        )}
      </div>
    </div>
  );
}
