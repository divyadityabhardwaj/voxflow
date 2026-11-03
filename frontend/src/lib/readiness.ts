export interface ReadinessIssue {
  kind: "microphone" | "speech" | "accessibility";
  title: string;
  description: string;
}

export interface SetupSnapshot {
  microphone: string;
  accessibility: boolean;
  transcription: string;
  groqKeySet: boolean;
  engineReady: boolean;
  modelDownloaded: boolean;
  modelReady: boolean;
  refinementMode: string;
  cleanupProvider?: { name: string; needsKey: boolean; keySet: boolean };
}

export function evaluateReadiness(setup: SetupSnapshot) {
  const cloud = setup.transcription === "groq";
  const cleanup = setup.refinementMode === "raw" || setup.refinementMode === "copy-only"
    ? "Off"
    : !setup.cleanupProvider || (setup.cleanupProvider.needsKey && !setup.cleanupProvider.keySet)
      ? "Off (no key)"
      : setup.cleanupProvider.name;
  let issue: ReadinessIssue | null = null;
  if (setup.microphone !== "authorized") {
    issue = {
      kind: "microphone",
      title: "Allow microphone access",
      description: "VoxFlow needs microphone permission to hear you.",
    };
  } else if (cloud && !setup.groqKeySet) {
    issue = {
      kind: "speech",
      title: "Add your Groq key",
      description: "Add a key in Speech settings, or choose transcription on this Mac.",
    };
  } else if (!cloud && !setup.engineReady) {
    issue = {
      kind: "speech",
      title: "The speech engine is missing",
      description: "Reinstall VoxFlow to restore it, or choose Groq in Speech settings.",
    };
  } else if (!cloud && !setup.modelDownloaded) {
    issue = {
      kind: "speech",
      title: "Download a speech model",
      description: "Choose and download a model in Speech settings to transcribe on this Mac.",
    };
  } else if (!cloud && !setup.modelReady) {
    issue = {
      kind: "speech",
      title: "The speech model isn't ready",
      description: "Check your selected model in Speech settings, or choose Groq.",
    };
  }
  const canRecord = issue === null;
  if (!issue && !setup.accessibility && setup.refinementMode !== "copy-only") {
    issue = {
      kind: "accessibility",
      title: "Allow VoxFlow to paste for you",
      description: "You can still dictate. Text will be copied until Accessibility is allowed.",
    };
  }
  return {
    canRecord,
    speech: cloud ? "Groq (cloud)" : "On this Mac",
    cleanup,
    issue,
    microphone: setup.microphone,
    needsPermission: setup.microphone !== "authorized" || (!setup.accessibility && setup.refinementMode !== "copy-only"),
  };
}
