import { useEffect, useState } from "react";
import { CheckForUpdates, GetUpdateInfo, UpdateAndRestart } from "../../../wailsjs/go/main/App";
import { update } from "../../../wailsjs/go/models";
import { BrowserOpenURL, EventsOn } from "../../../wailsjs/runtime/runtime";
import { Events } from "../../constants/events";
import SettingsSection, { SettingRow } from "../ui/SettingsSection";

export default function AboutSettings() {
  const [info, setInfo] = useState<update.Info | null>(null);
  const [pending, setPending] = useState(false);
  const [error, setError] = useState("");

  useEffect(() => {
    let active = true;
    let receivedEvent = false;
    const unsubscribe = EventsOn(Events.UpdateChanged, (next: update.Info) => {
      receivedEvent = true;
      if (active) setInfo(next);
    });
    GetUpdateInfo().then((next) => {
      if (active && !receivedEvent) setInfo(next);
    }).catch((err) => active && setError(String(err)));
    return () => { active = false; unsubscribe(); };
  }, []);

  const dev = !info?.current || info.current === "dev";
  const busy = pending || (!!info?.state && info.state !== "idle");
  const progress = info?.total ? Math.min(100, Math.round(info.downloaded / info.total * 100)) : 0;
  const status = info?.state === "checking" ? "Checking for updates…"
    : info?.state === "downloading" ? `Downloading update… ${progress}%`
    : info?.state === "verifying" ? "Verifying update…"
    : info?.state === "restarting" ? "Restarting VoxFlow…"
    : "";

  const run = async (operation: () => Promise<unknown>) => {
    setPending(true);
    setError("");
    try { await operation(); }
    catch (err) { setError(String(err)); }
    finally { setPending(false); }
  };

  return (
    <SettingsSection title="About VoxFlow" description="Voice dictation for every app.">
      <SettingRow
        label="Version"
        description={dev ? "Development build — update checks are off."
          : info?.available ? `Version ${info.latest} is available.`
          : info?.latest ? "You're up to date."
          : "VoxFlow checks for updates at launch and once a day."}
      >
        <span className="text-[13px] text-secondary tabular-nums">
          {dev ? "Development" : info?.current}
        </span>
        {!dev && (
          <button type="button" className="btn-secondary" disabled={busy}
            onClick={() => void run(CheckForUpdates)}>
            {info?.state === "checking" ? "Checking…" : "Check for Updates"}
          </button>
        )}
      </SettingRow>

      {info?.available && (
        <div className="settings-pad space-y-3">
          <h4 className="text-[13px] font-medium text-text">What's new in v{info.latest}</h4>
          {info.summary?.length > 0 ? (
            <ul className="list-disc pl-4 space-y-1 text-[13px] text-secondary leading-relaxed">
              {info.summary.map((line, index) => <li key={index}>{line}</li>)}
            </ul>
          ) : <p className="text-[13px] text-secondary">Improvements and bug fixes. See the release notes for details.</p>}
          <div className="flex flex-wrap gap-2">
            {info.can_install && (
              <button type="button" className="btn btn-primary" disabled={busy}
                onClick={() => void run(UpdateAndRestart)}>
                {busy && info.state !== "checking" ? "Updating…" : "Update & Restart"}
              </button>
            )}
            <button type="button" className="btn-secondary" disabled={busy}
              onClick={() => BrowserOpenURL(info.url)}>
              {info.can_install ? "Full release notes" : "Download from GitHub"}
            </button>
          </div>
          <p className="text-xs text-secondary leading-relaxed">
            {info.can_install
              ? "VoxFlow will download the update and restart. Your settings, history, and models will stay in place."
              : info.reason}
          </p>
        </div>
      )}

      {status && (
        <div className="settings-pad space-y-2" role="status" aria-live="polite">
          <p className="text-[13px] text-secondary">{status}</p>
          {info?.state === "downloading" && (
            <progress className="w-full h-2 accent-accent" aria-label="Update download" max={100} value={progress} />
          )}
        </div>
      )}
      {(error || info?.error) && (
        <p role="alert" className="settings-pad text-[13px] text-danger">{error || info?.error}</p>
      )}
    </SettingsSection>
  );
}
