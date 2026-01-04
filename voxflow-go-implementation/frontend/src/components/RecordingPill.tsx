import { useState, useEffect } from "react";
import { EventsOn } from "../../wailsjs/runtime/runtime";
import { ToggleRecording, GetStatus } from "../../wailsjs/go/main/App";

type Status = "Idle" | "Recording" | "Processing";

export default function RecordingPill() {
  const [status, setStatus] = useState<Status>("Idle");

  useEffect(() => {
    GetStatus().then((s) => setStatus(s as Status));

    EventsOn("state-changed", (newStatus: string) => {
      setStatus(newStatus as Status);
    });
  }, []);

  const handleStop = async () => {
    if (status === "Recording") {
      await ToggleRecording();
    }
  };

  // Only show when recording
  if (status !== "Recording") {
    return null;
  }

  return (
    <div className="fixed top-4 left-1/2 -translate-x-1/2 z-50">
      <div className="flex items-center gap-3 px-4 py-2 bg-[var(--bg-secondary)] border border-[var(--border)] rounded-full shadow-lg">
        {/* Wavy animation bars */}
        <div className="flex items-center gap-0.5 h-5">
          {[1, 2, 3, 4, 5].map((i) => (
            <div
              key={i}
              className="w-0.5 bg-recording rounded-full animate-wave"
              style={{
                height: "100%",
                animationDelay: `${i * 0.1}s`,
              }}
            />
          ))}
        </div>

        {/* Recording text */}
        <span className="text-sm text-secondary font-medium">Recording</span>

        {/* Stop button (red dot) */}
        <button
          onClick={handleStop}
          className="w-5 h-5 bg-recording rounded-full hover:scale-110 transition-transform flex items-center justify-center group"
          title="Stop recording"
        >
          <div className="w-2 h-2 bg-white rounded-sm group-hover:scale-110 transition-transform" />
        </button>
      </div>
    </div>
  );
}
