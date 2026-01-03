package main

import (
	"context"
	"fmt"
	"os"
	"sync"
	"time"
	"voxflow/internal/audio"
	"voxflow/internal/config"
	"voxflow/internal/gemini"
	"voxflow/internal/history"
	"voxflow/internal/hotkey"
	"voxflow/internal/injection"
	"voxflow/internal/whisper"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// App struct holds the application state
type App struct {
	ctx              context.Context
	config           *config.Config
	hotkeyManager    *hotkey.Manager
	state            hotkey.State
	audioRecorder    *audio.Recorder
	whisperService   *whisper.Service
	geminiClient     *gemini.Client
	historyService   *history.Service
	injectionService *injection.Service
	modelReady       bool
	downloadCancel   context.CancelFunc // Cancel function for active download
	downloadMu       sync.Mutex         // Mutex for download operations
}

// NewApp creates a new App application struct
func NewApp() *App {
	cfg := config.GetInstance()
	app := &App{
		config:         cfg,
		state:          hotkey.StateIdle,
		audioRecorder:  audio.NewRecorder(),
		whisperService: whisper.NewService(),
		geminiClient:   gemini.NewClient(cfg.GetGeminiAPIKey()),
	}
	return app
}

// startup is called when the app starts
func (a *App) startup(ctx context.Context) {
	a.ctx = ctx

	// Initialize audio
	if err := a.audioRecorder.Initialize(); err != nil {
		fmt.Printf("Warning: Failed to initialize audio: %v\n", err)
	}

	// Initialize history service
	histService, err := history.NewService()
	if err != nil {
		fmt.Printf("Warning: Failed to initialize history: %v\n", err)
	} else {
		a.historyService = histService
	}

	// Initialize injection service
	injService, err := injection.NewService(true)
	if err != nil {
		fmt.Printf("Warning: Failed to initialize injection: %v\n", err)
	} else {
		a.injectionService = injService
	}

	// Clean up any partial model downloads from previous interrupted sessions
	if err := whisper.CleanupPartialDownloads(); err != nil {
		fmt.Printf("Warning: Failed to cleanup partial downloads: %v\n", err)
	}

	// Check if model is downloaded
	go a.checkModelStatus()

	// Initialize hotkey manager with callback
	a.hotkeyManager = hotkey.NewManager(a.onHotkeyPressed)

	// Register and Start listening for hotkeys
	hfHotkey := a.config.GetHandsFreeHotkey()
	pttHotkey := a.config.GetPushToTalkHotkey()

	fmt.Printf("Starting hotkey manager with: HF=%s, PTT=%s\n", hfHotkey, pttHotkey)
	if err := a.hotkeyManager.Start(hfHotkey, pttHotkey); err != nil {
		fmt.Printf("Failed to start hotkey listener: %v\n", err)
	}
}

// shutdown is called when the app is closing
func (a *App) shutdown(ctx context.Context) {
	if a.hotkeyManager != nil {
		a.hotkeyManager.Stop()
	}
	if a.audioRecorder != nil {
		a.audioRecorder.Terminate()
	}
	if a.whisperService != nil {
		a.whisperService.Close()
	}
	if a.historyService != nil {
		a.historyService.Close()
	}
	a.config.Save()
}

// checkModelStatus checks if the Whisper model is downloaded and loads it
func (a *App) checkModelStatus() {
	modelSize := a.config.GetWhisperModel()
	downloaded, _ := a.whisperService.IsModelDownloaded(modelSize)

	if !downloaded {
		runtime.EventsEmit(a.ctx, "model-status", map[string]interface{}{
			"downloaded": false,
			"model":      modelSize,
		})
		return
	}

	// Try to load the model
	if err := a.whisperService.LoadModel(modelSize); err != nil {
		fmt.Printf("Failed to load model: %v\n", err)
		runtime.EventsEmit(a.ctx, "model-status", map[string]interface{}{
			"downloaded": true,
			"loaded":     false,
			"error":      err.Error(),
		})
		return
	}

	a.modelReady = true
	runtime.EventsEmit(a.ctx, "model-status", map[string]interface{}{
		"downloaded": true,
		"loaded":     true,
		"model":      modelSize,
	})
}

// IsModelReady returns whether the Whisper model is ready
func (a *App) IsModelReady() bool {
	return a.modelReady
}

// IsModelDownloaded checks if the model is downloaded
func (a *App) IsModelDownloaded() bool {
	modelSize := a.config.GetWhisperModel()
	downloaded, _ := a.whisperService.IsModelDownloaded(modelSize)
	return downloaded
}

// DownloadModel downloads the Whisper model
func (a *App) DownloadModel() error {
	modelSize := a.config.GetWhisperModel()

	err := a.whisperService.DownloadModel(modelSize, func(downloaded, total int64) {
		progress := float64(downloaded) / float64(total) * 100
		runtime.EventsEmit(a.ctx, "model-download-progress", map[string]interface{}{
			"downloaded": downloaded,
			"total":      total,
			"progress":   progress,
		})
	})

	if err != nil {
		runtime.EventsEmit(a.ctx, "model-download-error", err.Error())
		return err
	}

	// Load the model after download
	if err := a.whisperService.LoadModel(modelSize); err != nil {
		runtime.EventsEmit(a.ctx, "model-load-error", err.Error())
		return err
	}

	a.modelReady = true
	runtime.EventsEmit(a.ctx, "model-status", map[string]interface{}{
		"downloaded": true,
		"loaded":     true,
		"model":      modelSize,
	})

	return nil
}

// onHotkeyPressed is called when the global hotkey is pressed
func (a *App) onHotkeyPressed(state hotkey.State) {
	a.state = state
	runtime.EventsEmit(a.ctx, "state-changed", state.String())

	switch state {
	case hotkey.StateRecording:
		a.StartRecording()
	case hotkey.StateProcessing:
		a.StopRecording()
	}
}

// GetStatus returns the current application status
func (a *App) GetStatus() string {
	return a.state.String()
}

// StartRecording begins audio capture
func (a *App) StartRecording() error {
	if !a.modelReady {
		return fmt.Errorf("model not ready")
	}

	a.state = hotkey.StateRecording
	a.hotkeyManager.SetState(hotkey.StateRecording)

	if err := a.audioRecorder.Start(); err != nil {
		a.state = hotkey.StateIdle
		a.hotkeyManager.SetState(hotkey.StateIdle)
		runtime.EventsEmit(a.ctx, "error", err.Error())
		return err
	}

	runtime.EventsEmit(a.ctx, "state-changed", "Recording")
	runtime.EventsEmit(a.ctx, "recording-started", nil)
	fmt.Println("Recording started...")
	return nil
}

// StopRecording stops audio capture and begins processing
func (a *App) StopRecording() {
	a.state = hotkey.StateProcessing
	a.hotkeyManager.SetState(hotkey.StateProcessing)
	runtime.EventsEmit(a.ctx, "state-changed", "Processing")
	runtime.EventsEmit(a.ctx, "recording-stopped", nil)
	fmt.Println("Recording stopped, processing...")

	go a.processRecording()
}

// processRecording handles the transcription and refinement pipeline
func (a *App) processRecording() {
	startTime := time.Now()

	// Stop recording and get WAV file
	wavPath, err := a.audioRecorder.Stop()
	if err != nil {
		a.handleError("Failed to stop recording", err)
		return
	}
	defer os.Remove(wavPath) // Clean up temp file

	// Transcribe with Whisper
	rawText, err := a.whisperService.Transcribe(wavPath)
	if err != nil {
		a.handleError("Transcription failed", err)
		return
	}

	if rawText == "" {
		a.handleError("No speech detected", nil)
		return
	}

	runtime.EventsEmit(a.ctx, "transcription-complete", rawText)

	// Refine with Gemini
	mode := a.config.GetMode()
	polishedText, err := a.geminiClient.RefineText(rawText, mode)
	if err != nil {
		// If Gemini fails, use raw text
		fmt.Printf("Gemini refinement failed: %v, using raw text\n", err)
		polishedText = rawText
	}

	runtime.EventsEmit(a.ctx, "refinement-complete", polishedText)

	// Save to history
	if a.historyService != nil {
		_, err := a.historyService.Save("", rawText, polishedText, mode)
		if err != nil {
			fmt.Printf("Failed to save to history: %v\n", err)
		}
	}

	// Inject text
	if a.injectionService != nil {
		if err := a.injectionService.Inject(polishedText); err != nil {
			fmt.Printf("Failed to inject text: %v\n", err)
			// Fall back to just copying to clipboard
			a.injectionService.CopyToClipboard(polishedText)
		}
	}

	elapsed := time.Since(startTime)
	fmt.Printf("Processing complete in %v\n", elapsed)

	// Reset state
	a.state = hotkey.StateIdle
	a.hotkeyManager.SetState(hotkey.StateIdle)
	runtime.EventsEmit(a.ctx, "state-changed", "Idle")
	runtime.EventsEmit(a.ctx, "processing-complete", map[string]interface{}{
		"raw":      rawText,
		"polished": polishedText,
		"elapsed":  elapsed.Milliseconds(),
	})
}

// handleError handles errors during processing
func (a *App) handleError(message string, err error) {
	errMsg := message
	if err != nil {
		errMsg = fmt.Sprintf("%s: %v", message, err)
	}
	fmt.Println(errMsg)
	runtime.EventsEmit(a.ctx, "error", errMsg)

	a.state = hotkey.StateIdle
	a.hotkeyManager.SetState(hotkey.StateIdle)
	runtime.EventsEmit(a.ctx, "state-changed", "Idle")
}

// ToggleRecording toggles between recording and idle states
func (a *App) ToggleRecording() string {
	switch a.state {
	case hotkey.StateIdle:
		if err := a.StartRecording(); err != nil {
			return "Error: " + err.Error()
		}
		return "Recording"
	case hotkey.StateRecording:
		a.StopRecording()
		return "Processing"
	default:
		return a.state.String()
	}
}

// GetConfig returns the current configuration
func (a *App) GetConfig() map[string]interface{} {
	return map[string]interface{}{
		"hotkey":        a.config.GetHotkey(),
		"whisper_model": a.config.GetWhisperModel(),
		"mode":          a.config.GetMode(),
		"api_key_set":   a.config.GetGeminiAPIKey() != "",
	}
}

// SetAPIKey sets the Gemini API key
func (a *App) SetAPIKey(key string) error {
	a.config.SetGeminiAPIKey(key)
	a.geminiClient.SetAPIKey(key)
	return a.config.Save()
}

// SetHotkey sets the global hotkey
// reloadHotkeys re-initializes the hotkey manager with current config
func (a *App) reloadHotkeys() error {
	hf := a.config.GetHandsFreeHotkey()
	ptt := a.config.GetPushToTalkHotkey()

	if a.hotkeyManager != nil {
		fmt.Printf("Updating hotkeys: HF=%s, PTT=%s\n", hf, ptt)
		return a.hotkeyManager.Update(hf, ptt)
	}
	return fmt.Errorf("hotkey manager not initialized")
}

// SetHotkey sets the global hotkey (Legacy: maps to HandsFree)
func (a *App) SetHotkey(hotkeyStr string) error {
	return a.SetHandsFreeHotkey(hotkeyStr)
}

// SetHandsFreeHotkey sets the hands-free hotkey
func (a *App) SetHandsFreeHotkey(hotkeyStr string) error {
	old := a.config.GetHandsFreeHotkey()
	a.config.SetHandsFreeHotkey(hotkeyStr)

	if err := a.reloadHotkeys(); err != nil {
		fmt.Printf("Error reloading hotkeys (HF): %v\n", err)
		a.config.SetHandsFreeHotkey(old) // Revert on error
		a.reloadHotkeys()                // Restore state
		return err
	}

	return a.config.Save()
}

// SetPushToTalkHotkey sets the push-to-talk hotkey
func (a *App) SetPushToTalkHotkey(hotkeyStr string) error {
	old := a.config.GetPushToTalkHotkey()
	a.config.SetPushToTalkHotkey(hotkeyStr)

	if err := a.reloadHotkeys(); err != nil {
		fmt.Printf("Error reloading hotkeys (PTT): %v\n", err)
		a.config.SetPushToTalkHotkey(old) // Revert on error
		a.reloadHotkeys()                 // Restore state
		return err
	}

	return a.config.Save()
}

// SetWhisperModel sets the Whisper model size
func (a *App) SetWhisperModel(model string) error {
	a.config.SetWhisperModel(model)
	err := a.config.Save()
	if err != nil {
		return err
	}

	// Check if model needs to be downloaded
	a.modelReady = false
	go a.checkModelStatus()
	return nil
}

// SetMode sets the transcription mode (casual/formal)
func (a *App) SetMode(mode string) error {
	a.config.SetMode(mode)
	return a.config.Save()
}

// GetAllModels returns all available models with their download status
func (a *App) GetAllModels() ([]whisper.ModelInfo, error) {
	return a.whisperService.GetAllModels()
}

// DownloadModelByName downloads a specific model by name (cancellable)
func (a *App) DownloadModelByName(modelName string) error {
	a.downloadMu.Lock()

	// Cancel any existing download
	if a.downloadCancel != nil {
		a.downloadCancel()
	}

	// Create new context with cancel
	ctx, cancel := context.WithCancel(context.Background())
	a.downloadCancel = cancel
	a.downloadMu.Unlock()

	err := a.whisperService.DownloadModelWithContext(ctx, modelName, func(downloaded, total int64) {
		progress := float64(downloaded) / float64(total) * 100
		runtime.EventsEmit(a.ctx, "model-download-progress", map[string]interface{}{
			"model":      modelName,
			"downloaded": downloaded,
			"total":      total,
			"progress":   progress,
		})
	})

	// Clear the cancel function
	a.downloadMu.Lock()
	a.downloadCancel = nil
	a.downloadMu.Unlock()

	if err != nil {
		runtime.EventsEmit(a.ctx, "model-download-error", map[string]interface{}{
			"model": modelName,
			"error": err.Error(),
		})
		return err
	}

	runtime.EventsEmit(a.ctx, "model-download-complete", modelName)
	return nil
}

// CancelDownload cancels any active model download
func (a *App) CancelDownload() {
	a.downloadMu.Lock()
	defer a.downloadMu.Unlock()

	if a.downloadCancel != nil {
		fmt.Println("[App] Cancelling download...")
		a.downloadCancel()
		a.downloadCancel = nil
		runtime.EventsEmit(a.ctx, "model-download-cancelled", nil)
	}
}

// DeleteModelByName deletes a specific model
func (a *App) DeleteModelByName(modelName string) error {
	// Don't delete the currently active model
	activeModel := a.config.GetWhisperModel()
	if modelName == activeModel {
		return fmt.Errorf("cannot delete the currently active model")
	}

	err := a.whisperService.DeleteModel(modelName)
	if err != nil {
		return err
	}

	return nil
}

// IsWhisperCLIReady returns whether whisper-cli is available
func (a *App) IsWhisperCLIReady() bool {
	return a.whisperService.IsWhisperCLIInstalled()
}

// EnsureWhisperCLI ensures whisper-cli is installed
func (a *App) EnsureWhisperCLI() error {
	return a.whisperService.EnsureWhisperCLI(nil)
}

// GetHistory returns transcript history
func (a *App) GetHistory(limit int) ([]*history.Transcript, error) {
	if a.historyService == nil {
		return nil, fmt.Errorf("history service not available")
	}
	return a.historyService.GetAll(limit)
}

// SearchHistory searches transcript history
func (a *App) SearchHistory(query string, limit int) ([]*history.Transcript, error) {
	if a.historyService == nil {
		return nil, fmt.Errorf("history service not available")
	}
	return a.historyService.Search(query, limit)
}

// GetTranscript returns a single transcript by ID
func (a *App) GetTranscript(id int64) (*history.Transcript, error) {
	if a.historyService == nil {
		return nil, fmt.Errorf("history service not available")
	}
	return a.historyService.GetByID(id)
}

// DeleteTranscript deletes a transcript by ID
func (a *App) DeleteTranscript(id int64) error {
	if a.historyService == nil {
		return fmt.Errorf("history service not available")
	}
	return a.historyService.Delete(id)
}

// RetryWithGemini re-processes a transcript with a custom instruction
func (a *App) RetryWithGemini(id int64, instruction string) (string, error) {
	if a.historyService == nil {
		return "", fmt.Errorf("history service not available")
	}

	transcript, err := a.historyService.GetByID(id)
	if err != nil {
		return "", err
	}

	// Use raw text if no instruction, otherwise apply instruction
	var newPolished string
	if instruction == "" {
		newPolished, err = a.geminiClient.RefineText(transcript.RawText, a.config.GetMode())
	} else {
		newPolished, err = a.geminiClient.RetryWithInstruction(transcript.PolishedText, instruction)
	}

	if err != nil {
		return "", err
	}

	// Update in database
	if err := a.historyService.UpdatePolishedText(id, newPolished); err != nil {
		return "", err
	}

	return newPolished, nil
}

// CopyToClipboard copies text to clipboard
func (a *App) CopyToClipboard(text string) error {
	if a.injectionService == nil {
		return fmt.Errorf("injection service not available")
	}
	return a.injectionService.CopyToClipboard(text)
}

// OpenHistoryWindow emits event to open history window
func (a *App) OpenHistoryWindow() {
	runtime.EventsEmit(a.ctx, "open-history", nil)
}

// OpenSettings emits event to open settings panel
func (a *App) OpenSettings() {
	runtime.EventsEmit(a.ctx, "open-settings", nil)
}

// Quit closes the application
func (a *App) Quit() {
	runtime.Quit(a.ctx)
}
