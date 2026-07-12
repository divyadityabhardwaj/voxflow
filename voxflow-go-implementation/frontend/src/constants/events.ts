export const Events = {
  // App State Events
  StateChanged: "state-changed",
  RecordingStarted: "recording-started",
  RecordingStopped: "recording-stopped",
  ProcessingComplete: "processing-complete",
  PartialTranscript: "partial-transcript",

  // Window Events
  MiniMode: "mini-mode",

  // Whisper Model Events
  ModelStatus: "model-status",
  ModelDownloadProgress: "model-download-progress",
  ModelDownloadError: "model-download-error",
  ModelDownloadComplete: "model-download-complete",
  ModelDownloadCancelled: "model-download-cancelled",
  ModelLoadError: "model-load-error",

  // Local Model Events
  LocalModelStatus: "local-model-status",
  LocalModelDownloadProgress: "local-model-download-progress",
  LocalModelDownloadError: "local-model-download-error",
  LocalModelDownloadComplete: "local-model-download-complete",
  LocalModelDownloadCancelled: "local-model-download-cancelled",

  // UI & Navigation Events
  Toast: "toast",
  Error: "error",
  OpenHistory: "open-history",
  OpenSettings: "open-settings",
} as const;
