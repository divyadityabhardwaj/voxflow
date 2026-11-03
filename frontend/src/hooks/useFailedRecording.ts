import { useEffect, useState } from "react";
import { GetFailedRecording } from "../../wailsjs/go/main/App";
import { EventsOn } from "../../wailsjs/runtime/runtime";
import { Events } from "../constants/events";

export interface FailedRecording {
  available: boolean;
  message: string;
  expires_at: number;
}

export function useFailedRecording() {
  const [failed, setFailed] = useState<FailedRecording | null>(null);
  useEffect(() => {
    let live = true;
    let gotEvent = false;
    const off = EventsOn(Events.FailedRecordingChanged, (value: FailedRecording) => {
      gotEvent = true;
      setFailed(value.available ? value : null);
    });
    GetFailedRecording().then((value) => {
      if (live && !gotEvent) setFailed(value.available ? value : null);
    }).catch(() => {});
    return () => { live = false; off(); };
  }, []);
  return failed;
}
