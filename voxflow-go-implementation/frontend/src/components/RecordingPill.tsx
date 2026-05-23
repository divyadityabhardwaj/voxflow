import { ToggleRecording } from "../../wailsjs/go/main/App";
import { useRecordingState } from "../hooks/useRecordingState";

export default function RecordingPill() {
  const status = useRecordingState();

  const handleStop = async () => {

    if (status === "Recording") {
      await ToggleRecording();
    }
  };

  if (status !== "Recording") {
    return null;
  }

  return (
    <div className="fixed top-4 left-1/2 -translate-x-1/2 z-50">
      <div className="flex items-center gap-3 px-4 py-2 bg-background border border-border rounded-2xl shadow-soft-md">
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
        <span className="text-sm text-text font-bold">Recording</span>

        {/* Stop button */}
        <button
          onClick={handleStop}
          className="w-5 h-5 bg-recording rounded-md hover:scale-110 transition-transform flex items-center justify-center group"
          title="Stop recording"
        >
          <div className="w-2 h-1.5 bg-white rounded-sm group-hover:scale-110 transition-transform" />
        </button>
      </div>
    </div>
  );
}
