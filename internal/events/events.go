package events

const (
	StateChanged       = "state-changed"
	RecordingStarted   = "recording-started"
	RecordingStopped   = "recording-stopped"
	ProcessingComplete = "processing-complete"
	ResetToIdle        = "reset-to-idle"
	PartialTranscript  = "partial-transcript"

	MiniMode = "mini-mode"

	ModelStatus            = "model-status"
	ModelDownloadProgress  = "model-download-progress"
	ModelDownloadError     = "model-download-error"
	ModelDownloadComplete  = "model-download-complete"
	ModelDownloadCancelled = "model-download-cancelled"
	ModelLoadError         = "model-load-error"

	LocalModelStatus            = "local-model-status"
	LocalModelDownloadProgress  = "local-model-download-progress"
	LocalModelDownloadError     = "local-model-download-error"
	LocalModelDownloadComplete  = "local-model-download-complete"
	LocalModelDownloadCancelled = "local-model-download-cancelled"

	Toast        = "toast"
	Error        = "error"
	OpenHistory  = "open-history"
	OpenSettings = "open-settings"
)
