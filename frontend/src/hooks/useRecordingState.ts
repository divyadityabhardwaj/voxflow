import { useState, useEffect } from "react";
import { EventsOn } from "../../wailsjs/runtime/runtime";
import { GetStatus } from "../../wailsjs/go/main/App";
import { Events } from "../constants/events";

export type Status = "Idle" | "Recording" | "Processing" | "Refining";

export const isBusy = (status: Status) =>
  status === "Processing" || status === "Refining";

export function useRecordingState() {
  const [status, setStatus] = useState<Status>("Idle");

  useEffect(() => {
    // An event that lands before GetStatus resolves is newer; don't clobber it.
    let gotEvent = false;
    GetStatus().then((s) => {
      if (!gotEvent) setStatus(s as Status);
    });

    const unsub = EventsOn(Events.StateChanged, (newStatus: string) => {
      gotEvent = true;
      setStatus(newStatus as Status);
    });

    return unsub;
  }, []);

  return status;
}
