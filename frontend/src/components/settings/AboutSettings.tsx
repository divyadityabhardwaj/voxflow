import { useEffect, useState } from "react";
import { GetUpdateInfo } from "../../../wailsjs/go/main/App";
import { update } from "../../../wailsjs/go/models";
import { BrowserOpenURL, EventsOn } from "../../../wailsjs/runtime/runtime";
import { Events } from "../../constants/events";
import SettingsSection, { SettingRow } from "../ui/SettingsSection";

export default function AboutSettings() {
  const [info, setInfo] = useState<update.Info | null>(null);

  useEffect(() => {
    GetUpdateInfo().then(setInfo).catch(() => {});
    return EventsOn(Events.UpdateAvailable, (i: update.Info) => setInfo(i));
  }, []);

  const dev = !info?.current || info.current === "dev";

  return (
    <SettingsSection title="About VoxFlow" description="Voice dictation for every app.">
      <SettingRow
        label="Version"
        description={
          info?.available
            ? `Version ${info.latest} is available.`
            : dev
              ? "Development build — update checks are off."
              : info?.latest
                ? "You're up to date."
                : "VoxFlow checks for updates once a day."
        }
      >
        <span className="text-[13px] text-secondary tabular-nums">
          {dev ? "Development" : info?.current}
        </span>
        {info?.available && info.url && (
          <button
            type="button"
            className="btn btn-primary"
            onClick={() => BrowserOpenURL(info.url)}
          >
            Download
          </button>
        )}
      </SettingRow>
    </SettingsSection>
  );
}
