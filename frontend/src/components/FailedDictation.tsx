import { useEffect, useState } from "react";
import { DiscardFailedRecording, RetryFailedRecording } from "../../wailsjs/go/main/App";
import { useFailedRecording } from "../hooks/useFailedRecording";
import { formatClock } from "../hooks/useDictation";

export default function FailedDictation({ busy }: { busy: boolean }) {
  const failed = useFailedRecording();
  const [working, setWorking] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [now, setNow] = useState(Date.now());
  useEffect(() => {
    setError(null);
    setNow(Date.now());
    if (!failed) return;
    const timer = setInterval(() => setNow(Date.now()), 1000);
    return () => clearInterval(timer);
  }, [failed]);
  if (!failed) return null;
  const seconds = Math.max(0, Math.ceil((failed.expires_at - now) / 1000));
  const act = async (action: () => Promise<void>) => {
    setWorking(true);
    setError(null);
    try { await action(); } catch (err) { setError(String(err)); }
    finally { setWorking(false); }
  };
  return (
    <section className="w-full card p-4 border-danger/30" aria-label="Recover dictation">
      <h2 className="text-sm font-semibold text-text">Dictation failed</h2>
      <p className="text-[13px] text-secondary mt-1" role="alert">{error || failed.message}</p>
      <p className="text-xs text-secondary mt-2">
        Audio kept for {formatClock(seconds)}. A new recording or quitting discards it. Retried text appears here.
      </p>
      <div className="flex justify-end gap-2 mt-3">
        <button type="button" className="btn-secondary" disabled={working} onClick={() => act(DiscardFailedRecording)}>Discard</button>
        <button type="button" className="btn-primary" disabled={busy || working || seconds === 0} onClick={() => act(RetryFailedRecording)}>
          {working ? "Retrying…" : "Retry"}
        </button>
      </div>
    </section>
  );
}
