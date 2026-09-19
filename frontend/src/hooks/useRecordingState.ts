import { useState, useEffect } from "react";
import { EventsOn } from "../../wailsjs/runtime/runtime";
import { GetStatus } from "../../wailsjs/go/main/App";
import { Events } from "../constants/events";

export type Status = "Idle" | "Recording" | "Processing";

export function useRecordingState() {
  const [status, setStatus] = useState<Status>("Idle");

  useEffect(() => {
    GetStatus().then((s) => setStatus(s as Status));

    const unsub = EventsOn(Events.StateChanged, (newStatus: string) => {
      setStatus(newStatus as Status);
    });

    return unsub;
  }, []);

  return status;
}
