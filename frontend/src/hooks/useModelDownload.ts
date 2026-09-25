import { useState, useEffect, useRef } from "react";
import { EventsOn } from "../../wailsjs/runtime/runtime";
import { DownloadModel, GetActiveDownload } from "../../wailsjs/go/main/App";
import { Events } from "../constants/events";

interface DownloadProgress {
  progress?: number;
  downloaded?: number;
  total?: number;
}

interface ModelStatus {
  downloaded: boolean;
  loaded?: boolean;
  error?: string;
}

const MB = 1024 * 1024;
const NO_PROGRESS = { percent: 0, downloadedMB: 0, totalMB: 0 };

// Error events arrive as a bare string or as {model, error}.
const errorText = (e: unknown) =>
  typeof e === "string"
    ? e
    : (e as { error?: string } | null)?.error || String(e);

export function useModelDownload(onReady: () => void) {
  const [downloading, setDownloading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [progress, setProgress] = useState(NO_PROGRESS);
  const onReadyRef = useRef(onReady);
  onReadyRef.current = onReady;

  const fail = (e: unknown) => {
    setError(errorText(e));
    setDownloading(false);
  };

  useEffect(() => {
    GetActiveDownload()
      .then((d) => {
        if (!d.model) return;
        setDownloading(true);
        setProgress({
          percent: Math.round(d.progress),
          downloadedMB: Number((d.downloaded / MB).toFixed(1)),
          totalMB: Number((d.total / MB).toFixed(1)),
        });
      })
      .catch(() => {});
    const unsubs = [
      EventsOn(Events.ModelDownloadProgress, (d: DownloadProgress) => {
        const pct = Math.round(
          d.progress ?? (d.total ? ((d.downloaded ?? 0) / d.total) * 100 : 0),
        );
        // A download started elsewhere (e.g. onboarding) is still ours to show.
        setDownloading(true);
        // Progress fires on every network read; only re-render per whole percent.
        setProgress((prev) =>
          prev.percent === pct
            ? prev
            : {
                percent: pct,
                downloadedMB: Number(((d.downloaded ?? 0) / MB).toFixed(1)),
                totalMB: Number(((d.total ?? 0) / MB).toFixed(1)),
              },
        );
      }),
      EventsOn(Events.ModelDownloadError, fail),
      EventsOn(Events.ModelLoadError, fail),
      EventsOn(Events.ModelStatus, (s: ModelStatus) => {
        if (s.downloaded && s.loaded) {
          setDownloading(false);
          onReadyRef.current();
        } else if (s.error) {
          fail(s.error);
        }
      }),
    ];
    return () => unsubs.forEach((u) => u());
  }, []);

  const start = async () => {
    setError(null);
    setProgress(NO_PROGRESS);
    setDownloading(true);
    try {
      await DownloadModel();
    } catch (err) {
      fail(err);
    }
  };

  return { downloading, error, start, ...progress };
}
