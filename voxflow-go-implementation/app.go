package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"
	"voxflow/internal/audio"
	"voxflow/internal/cerebras"
	"voxflow/internal/config"
	"voxflow/internal/events"
	"voxflow/internal/gemini"
	"voxflow/internal/groq"
	"voxflow/internal/history"
	"voxflow/internal/hotkey"
	"voxflow/internal/injection"
	"voxflow/internal/localclient"
	"voxflow/internal/localgguf"
	"voxflow/internal/logger"
	"voxflow/internal/ollama"
	"voxflow/internal/openrouter"
	"voxflow/internal/whisper"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// App struct holds the application state
type App struct {
	ctx                     context.Context
	config                  *config.Config
	hotkeyManager           *hotkey.Manager
	state                   hotkey.State
	audioRecorder           *audio.Recorder
	whisperService          *whisper.Service
	localGGUFService        *localgguf.Service
	localClient             *localclient.Client
	ollamaService           *ollama.Service
	geminiClient            *gemini.Client
	openRouterClient        *openrouter.Client
	groqClient              *groq.Client
	cerebrasClient          *cerebras.Client
	historyService          *history.Service
	injectionService        *injection.Service
	modelReady              bool
	isMiniMode              bool               // Tracks if app is in mini indicator mode
	userExplicitlyMaximized bool               // Tracks if user manually opened full app (don't auto-minimize)
	downloadCancel          context.CancelFunc // Cancel function for active download
	downloadMu              sync.Mutex         // Mutex for download operations
	positionWatchCancel     context.CancelFunc // Cancel function for position polling
	localDownloadCancel     context.CancelFunc // Cancel function for local model download
	localDownloadMu         sync.Mutex         // Mutex for local download operations
	localDownloadingModel   string             // Track which model is currently downloading
	volumeMu                sync.Mutex         // Guards savedVolume
	savedVolume             int                // System volume saved before muting; -1 = not muted
}

// NewApp creates a new App application struct
func NewApp() *App {
	cfg := config.GetInstance()
	app := &App{
		config:           cfg,
		state:            hotkey.StateIdle,
		isMiniMode:       true, // Start in mini mode (floating indicator)
		audioRecorder:    audio.NewRecorder(),
		whisperService:   whisper.NewService(),
		localGGUFService: localgguf.NewService(),
		ollamaService:    ollama.NewService(),
		geminiClient:     gemini.NewClient(cfg.GetGeminiAPIKey(), cfg.GetGeminiModel()),
		openRouterClient: openrouter.NewClient(cfg.GetOpenRouterAPIKey()),
		groqClient:       groq.NewClient(cfg.GetGroqAPIKey()),
		cerebrasClient:   cerebras.NewClient(cfg.GetCerebrasAPIKey()),
		savedVolume:      -1, // -1 = not currently muted by VoxFlow
	}
	app.localClient = localclient.NewClient(app.ollamaService.BaseURL())

	return app
}

// SetGeminiModel sets the Gemini model
func (a *App) SetGeminiModel(model string) error {
	a.config.SetGeminiModel(model)
	a.geminiClient.SetModel(model)
	return a.config.Save()
}

// GetGeminiModel returns the current Gemini model
func (a *App) GetGeminiModel() string {
	return a.config.GetGeminiModel()
}

type CheckResult struct {
	LatencyMs int64   `json:"latency"`
	TPS       float64 `json:"tps"`
}

// GetGeminiModels returns all available Gemini models
func (a *App) GetGeminiModels() ([]string, error) {
	return a.geminiClient.ListModels()
}

// CheckGeminiModel tests a Gemini model and returns latency and TPS
func (a *App) CheckGeminiModel(model string) (*CheckResult, error) {
	latency, tps, err := a.geminiClient.CheckModel(model)
	if err != nil {
		return nil, err
	}
	return &CheckResult{LatencyMs: latency, TPS: tps}, nil
}

// GetOpenRouterModels returns all available free OpenRouter models
func (a *App) GetOpenRouterModels() ([]string, error) {
	return a.openRouterClient.GetFreeModels()
}

// GetOpenRouterModelDescriptions returns descriptions for all OpenRouter models
func (a *App) GetOpenRouterModelDescriptions() map[string]string {
	return openrouter.ModelDescriptions
}

// CheckOpenRouterModel tests an OpenRouter model and returns latency and TPS
func (a *App) CheckOpenRouterModel(model string) (*CheckResult, error) {
	latency, tps, err := a.openRouterClient.CheckModel(model)
	if err != nil {
		return nil, err
	}
	return &CheckResult{LatencyMs: latency, TPS: tps}, nil
}

// SetOpenRouterAPIKey sets the OpenRouter API key
func (a *App) SetOpenRouterAPIKey(key string) error {
	a.config.SetOpenRouterAPIKey(key)
	a.openRouterClient.SetAPIKey(key)
	return a.config.Save()
}

// SetLLMProvider sets the LLM provider (gemini or openrouter)
func (a *App) SetLLMProvider(provider string) error {
	a.config.SetLLMProvider(provider)
	return a.config.Save()
}

// GetLLMProvider returns the current LLM provider
func (a *App) GetLLMProvider() string {
	return a.config.GetLLMProvider()
}

// SetOpenRouterModel sets the OpenRouter model
func (a *App) SetOpenRouterModel(model string) error {
	a.config.SetOpenRouterModel(model)
	return a.config.Save()
}

// GetOpenRouterModel returns the current OpenRouter model
func (a *App) GetOpenRouterModel() string {
	return a.config.GetOpenRouterModel()
}

// GetGroqModels returns all available Groq models
func (a *App) GetGroqModels() ([]string, error) {
	return a.groqClient.GetModels()
}

// GetGroqModelDescriptions returns descriptions for all Groq models
func (a *App) GetGroqModelDescriptions() map[string]string {
	return groq.ModelDescriptions
}

// CheckGroqModel tests a Groq model and returns latency and TPS
func (a *App) CheckGroqModel(model string) (*CheckResult, error) {
	latency, tps, err := a.groqClient.CheckModel(model)
	if err != nil {
		return nil, err
	}
	return &CheckResult{LatencyMs: latency, TPS: tps}, nil
}

// SetGroqAPIKey sets the Groq API key
func (a *App) SetGroqAPIKey(key string) error {
	a.config.SetGroqAPIKey(key)
	a.groqClient.SetAPIKey(key)
	a.groqClient.ClearModelsCache()
	return a.config.Save()
}

// SetGroqModel sets the Groq model
func (a *App) SetGroqModel(model string) error {
	a.config.SetGroqModel(model)
	return a.config.Save()
}

// GetGroqModel returns the current Groq model
func (a *App) GetGroqModel() string {
	return a.config.GetGroqModel()
}

// GetCerebrasModels returns all available Cerebras models
func (a *App) GetCerebrasModels() ([]string, error) {
	return a.cerebrasClient.GetModels()
}

// GetCerebrasModelDescriptions returns descriptions for all Cerebras models
func (a *App) GetCerebrasModelDescriptions() map[string]string {
	return cerebras.ModelDescriptions
}

// CheckCerebrasModel tests a Cerebras model and returns latency and TPS
func (a *App) CheckCerebrasModel(model string) (*CheckResult, error) {
	latency, tps, err := a.cerebrasClient.CheckModel(model)
	if err != nil {
		return nil, err
	}
	return &CheckResult{LatencyMs: latency, TPS: tps}, nil
}

// CheckLocalModel tests a local model and returns latency and TPS
func (a *App) CheckLocalModel(model string) (*CheckResult, error) {
	alias := localgguf.GetOllamaModelAlias(model)
	latency, tps, err := a.localClient.CheckModel(alias)
	if err != nil {
		return nil, err
	}
	return &CheckResult{LatencyMs: latency, TPS: tps}, nil
}

// SetCerebrasAPIKey sets the Cerebras API key
func (a *App) SetCerebrasAPIKey(key string) error {
	a.config.SetCerebrasAPIKey(key)
	a.cerebrasClient.SetAPIKey(key)
	a.cerebrasClient.ClearModelsCache()
	return a.config.Save()
}

// SetCerebrasModel sets the Cerebras model
func (a *App) SetCerebrasModel(model string) error {
	a.config.SetCerebrasModel(model)
	return a.config.Save()
}

// GetCerebrasModel returns the current Cerebras model
func (a *App) GetCerebrasModel() string {
	return a.config.GetCerebrasModel()
}

// startup is called when the app starts
func (a *App) startup(ctx context.Context) {
	a.ctx = ctx

	// Make the floating indicator visible on all spaces and over fullscreen apps
	MakeWindowFloatEverywhere()

	// If starting in mini mode, ensure position is restored and watcher is started
	if a.isMiniMode {
		// Restore saved position if available
		x, y := a.config.GetMiniModePosition()
		if x != 0 || y != 0 {
			runtime.WindowSetPosition(a.ctx, x, y)
			// Ensure size is correct too, just in case
			runtime.WindowSetMinSize(a.ctx, 200, 60)
			runtime.WindowSetMaxSize(a.ctx, 200, 60)
			runtime.WindowSetSize(a.ctx, 200, 60)
		}

		// Start watching position
		a.startPositionWatch()
	}

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
	if err := localgguf.CleanupPartialDownloads(); err != nil {
		fmt.Printf("Warning: Failed to cleanup local GGUF partial downloads: %v\n", err)
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
	if a.ollamaService != nil {
		a.ollamaService.Stop()
	}
	if a.historyService != nil {
		a.historyService.Close()
	}
	// Stop position watcher
	if a.positionWatchCancel != nil {
		a.positionWatchCancel()
	}

	// Save window position if we are shutting down in mini mode
	if a.isMiniMode {
		a.saveCurrentMiniModePosition()
	}

	a.config.Save()
}

// checkModelStatus checks if the Whisper model is downloaded and loads it
func (a *App) checkModelStatus() {
	modelSize := a.config.GetWhisperModel()
	downloaded, _ := a.whisperService.IsModelDownloaded(modelSize)

	if !downloaded {
		runtime.EventsEmit(a.ctx, events.ModelStatus, map[string]interface{}{
			"downloaded": false,
			"model":      modelSize,
		})
		return
	}

	// Try to load the model
	if err := a.whisperService.LoadModel(modelSize); err != nil {
		fmt.Printf("Failed to load model: %v\n", err)
		runtime.EventsEmit(a.ctx, events.ModelStatus, map[string]interface{}{
			"downloaded": true,
			"loaded":     false,
			"error":      err.Error(),
		})
		return
	}

	a.modelReady = true
	runtime.EventsEmit(a.ctx, events.ModelStatus, map[string]interface{}{
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
		runtime.EventsEmit(a.ctx, events.ModelDownloadProgress, map[string]interface{}{
			"downloaded": downloaded,
			"total":      total,
			"progress":   progress,
		})
	})

	if err != nil {
		runtime.EventsEmit(a.ctx, events.ModelDownloadError, err.Error())
		return err
	}

	// Load the model after download
	if err := a.whisperService.LoadModel(modelSize); err != nil {
		runtime.EventsEmit(a.ctx, events.ModelLoadError, err.Error())
		return err
	}

	a.modelReady = true
	runtime.EventsEmit(a.ctx, events.ModelStatus, map[string]interface{}{
		"downloaded": true,
		"loaded":     true,
		"model":      modelSize,
	})

	return nil
}

// captureTargetApp returns the bundle ID of the currently frontmost application.
// This must be called BEFORE recording starts (i.e., before Voxflow steals focus),
// so that text can be injected back into the correct app after processing.
func (a *App) captureTargetApp() string {
	script := `
		tell application "System Events"
			set frontApp to first application process whose frontmost is true
			return bundle identifier of frontApp
		end tell
	`
	cmd := exec.Command("osascript", "-e", script)
	out, err := cmd.Output()
	if err != nil {
		fmt.Printf("[Injection] Failed to capture target app: %v\n", err)
		return ""
	}
	bundleID := strings.TrimSpace(string(out))
	fmt.Printf("[Injection] Target app bundle ID: %s\n", bundleID)
	return bundleID
}

// onHotkeyPressed is called when the global hotkey is pressed
func (a *App) onHotkeyPressed(state hotkey.State) {
	a.state = state
	runtime.EventsEmit(a.ctx, events.StateChanged, state.String())

	switch state {
	case hotkey.StateRecording:
		// Capture the frontmost app bundle ID NOW — before Voxflow grabs focus during processing
		if a.injectionService != nil {
			bundleID := a.captureTargetApp()
			a.injectionService.SetTargetApp(bundleID)
		}
		// Only switch to mini mode if user hasn't explicitly maximized the app
		if !a.userExplicitlyMaximized {
			a.ShowMiniMode()
		}
		a.StartRecording()
	case hotkey.StateProcessing:
		a.StopRecording()
		// Note: HideMiniMode is called after processing completes in processRecording()
	case hotkey.StateIdle:
		if !a.userExplicitlyMaximized {
			a.HideMiniMode()
		}
	}
}

// ShowMiniMode switches the window to a small floating indicator
func (a *App) ShowMiniMode() {
	if a.isMiniMode {
		return
	}
	a.isMiniMode = true
	a.userExplicitlyMaximized = false // User explicitly minimized

	// Re-apply floating behavior (in case coming from full app mode)
	MakeWindowFloatEverywhere()

	// Resize to small indicator and lock size
	runtime.WindowSetMinSize(a.ctx, 200, 60)
	runtime.WindowSetMaxSize(a.ctx, 200, 60)
	runtime.WindowSetSize(a.ctx, 200, 60)

	// Restore saved position if available
	x, y := a.config.GetMiniModePosition()
	if x != 0 || y != 0 {
		runtime.WindowSetPosition(a.ctx, x, y)
	}

	runtime.WindowSetAlwaysOnTop(a.ctx, true)
	runtime.EventsEmit(a.ctx, events.MiniMode, true)

	// Start watching position for changes
	a.startPositionWatch()

	fmt.Println("[App] Switched to mini mode")
}

// startPositionWatch starts a goroutine to poll and save window position
func (a *App) startPositionWatch() {
	// Stop existing watcher if any
	if a.positionWatchCancel != nil {
		a.positionWatchCancel()
	}

	ctx, cancel := context.WithCancel(context.Background())
	a.positionWatchCancel = cancel

	go func() {
		ticker := time.NewTicker(500 * time.Millisecond)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				// Get current position
				rx, ry := runtime.WindowGetPosition(a.ctx)
				// Get saved position
				cx, cy := a.config.GetMiniModePosition()

				// If changed, save
				if rx != cx || ry != cy {
					a.config.SetMiniModePosition(rx, ry)
					a.config.Save() // Save to disk to persist across crashes
					// Avoid spamming logs, but useful for debug
					// fmt.Printf("[App] Auto-saved position: %d, %d\n", rx, ry)
				}
			}
		}
	}()
}

// saveCurrentMiniModePosition saves the current window position to config if in mini mode
func (a *App) saveCurrentMiniModePosition() {
	if a.isMiniMode {
		x, y := runtime.WindowGetPosition(a.ctx)
		a.config.SetMiniModePosition(x, y)
		a.config.Save()
		fmt.Printf("[App] Saved mini mode position: %d, %d\n", x, y)
	}
}

// startMaximizedPositionWatch starts a goroutine to poll and save maximized window position and size
func (a *App) startMaximizedPositionWatch() {
	// Stop existing watcher if any
	if a.positionWatchCancel != nil {
		a.positionWatchCancel()
	}

	ctx, cancel := context.WithCancel(context.Background())
	a.positionWatchCancel = cancel

	go func() {
		ticker := time.NewTicker(500 * time.Millisecond)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				// Get current position
				rx, ry := runtime.WindowGetPosition(a.ctx)
				// Get current size - returns (width, height)
				rw, rh := runtime.WindowGetSize(a.ctx)
				// Get saved position and size
				cx, cy := a.config.GetMaximizedWindowPosition()
				cw, ch := a.config.GetMaximizedWindowSize()

				// If changed, save
				if rx != cx || ry != cy || rw != cw || rh != ch {
					a.config.SetMaximizedWindowPosition(rx, ry)
					a.config.SetMaximizedWindowSize(rw, rh)
					a.config.Save() // Save to disk to persist across crashes
				}
			}
		}
	}()
}

// HideMiniMode restores the window to normal size
func (a *App) HideMiniMode() {
	if !a.isMiniMode {
		return
	}

	// Stop position watching
	if a.positionWatchCancel != nil {
		a.positionWatchCancel()
		a.positionWatchCancel = nil
	}

	// Save current position one last time
	a.saveCurrentMiniModePosition()

	a.isMiniMode = false
	a.userExplicitlyMaximized = true // User explicitly opened full app

	// Reset window behavior to normal (not floating over fullscreen)
	ResetWindowBehavior()

	// Restore saved maximized position and size if available
	savedX, savedY := a.config.GetMaximizedWindowPosition()
	savedW, savedH := a.config.GetMaximizedWindowSize()

	// Set normal window size limits
	runtime.WindowSetMinSize(a.ctx, 800, 600)
	runtime.WindowSetMaxSize(a.ctx, 0, 0)

	// Restore saved size or use default
	if savedW > 0 && savedH > 0 {
		runtime.WindowSetSize(a.ctx, savedW, savedH)
	} else {
		runtime.WindowSetSize(a.ctx, 900, 600)
	}

	// Restore saved position or center if not saved
	if savedX != 0 || savedY != 0 {
		runtime.WindowSetPosition(a.ctx, savedX, savedY)
	} else {
		runtime.WindowCenter(a.ctx)
	}

	runtime.WindowSetAlwaysOnTop(a.ctx, false)
	runtime.EventsEmit(a.ctx, events.MiniMode, false)

	// Start watching maximized window position/size
	a.startMaximizedPositionWatch()

	fmt.Println("[App] Restored normal mode")
}

// IsMiniMode returns whether the app is in mini indicator mode
func (a *App) IsMiniMode() bool {
	return a.isMiniMode
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
		runtime.EventsEmit(a.ctx, events.Error, err.Error())
		return err
	}

	// Mute all system audio while we record (browsers, games, Spotify, YouTube — everything).
	// Run in a goroutine so the recording indicator shows immediately.
	go func() {
		vol := audio.MuteSystemAudio()
		a.volumeMu.Lock()
		a.savedVolume = vol
		a.volumeMu.Unlock()
	}()

	runtime.EventsEmit(a.ctx, events.StateChanged, "Recording")
	runtime.EventsEmit(a.ctx, events.RecordingStarted, nil)
	fmt.Println("Recording started...")
	return nil
}

// StopRecording stops audio capture and begins processing
func (a *App) StopRecording() {
	a.state = hotkey.StateProcessing
	a.hotkeyManager.SetState(hotkey.StateProcessing)
	runtime.EventsEmit(a.ctx, events.StateChanged, "Processing")
	runtime.EventsEmit(a.ctx, events.RecordingStopped, nil)
	fmt.Println("Recording stopped, processing...")

	go a.processRecording()
}

// processRecording handles the transcription and refinement pipeline
func (a *App) processRecording() {
	processingStartTime := time.Now()

	// Capture audio duration before stopping (buffer is still valid after Stop until next Start)
	audioDuration := a.audioRecorder.GetDuration()

	// Stop recording and get WAV file
	wavPath, err := a.audioRecorder.Stop()
	if err != nil {
		a.emitToast("Failed to stop recording: "+err.Error(), "error")
		a.resetToIdle()
		return
	}
	defer os.Remove(wavPath) // Clean up temp file

	// Transcribe with Whisper - retry up to 3 times if no audio detected
	var rawText string
	var whisperDuration time.Duration
	maxRetries := 3

	whisperStart := time.Now()
	for attempt := 1; attempt <= maxRetries; attempt++ {
		rawText, err = a.whisperService.Transcribe(wavPath)
		if err != nil {
			a.emitToast("Transcription failed: "+err.Error(), "error")
			a.resetToIdle()
			return
		}

		if rawText != "" {
			break // Successfully got transcription
		}

		if attempt < maxRetries {
			fmt.Printf("[App] No speech detected, retrying (%d/%d)...\n", attempt, maxRetries)
			time.Sleep(500 * time.Millisecond)
		}
	}
	whisperDuration = time.Since(whisperStart)

	fmt.Printf("\n[App] Whisper raw output (%d chars):\n%s\n", len(rawText), rawText)

	if rawText == "" {
		a.emitToast("No audio was captured. Please try speaking louder or check your microphone.", "warning")
		a.resetToIdle()
		return
	}

	// Check for Whisper's blank audio markers
	if rawText == "[BLANK_AUDIO]" || rawText == "(blank audio)" || rawText == "[NO SPEECH]" {
		a.emitToast("No speech detected. Please try speaking into your microphone.", "warning")
		a.resetToIdle()
		return
	}

	// Refine with Gemini or OpenRouter - based on provider setting
	mode := a.config.GetMode()
	llmProvider := a.config.GetLLMProvider()

	var polishedText string
	var tokenCount int
	var llmDuration time.Duration
	var llmStart time.Time

	var llmModel string

	if llmProvider == "openrouter" {
		llmModel = a.config.GetOpenRouterModel()
		llmStart = time.Now()
		polishedText, tokenCount, err = a.openRouterClient.RefineText(rawText, llmModel, mode)
		llmDuration = time.Since(llmStart)
	} else if llmProvider == "groq" {
		llmModel = a.config.GetGroqModel()
		llmStart = time.Now()
		polishedText, tokenCount, err = a.groqClient.RefineText(rawText, llmModel, mode)
		llmDuration = time.Since(llmStart)
	} else if llmProvider == "cerebras" {
		llmModel = a.config.GetCerebrasModel()
		llmStart = time.Now()
		polishedText, tokenCount, err = a.cerebrasClient.RefineText(rawText, llmModel, mode)
		llmDuration = time.Since(llmStart)
	} else if llmProvider == "local" {
		llmModel = localgguf.GetOllamaModelAlias(a.config.GetLocalModel())
		llmStart = time.Now()
		ensureErr := a.ensureLocalModelServer()
		if ensureErr != nil {
			err = ensureErr
			llmDuration = time.Since(llmStart)
			polishedText = ""
			tokenCount = 0
		} else {
			polishedText, tokenCount, err = a.localClient.RefineText(rawText, llmModel, mode)
			llmDuration = time.Since(llmStart)
		}
	} else {
		// Default to Gemini which gets its model from its own internal state or config later
		llmModel = a.config.GetGeminiModel()
		if llmModel == "" {
			llmModel = "gemini-1.5-flash"
		}
		llmStart = time.Now()
		polishedText, tokenCount, err = a.geminiClient.RefineText(rawText, mode)
		llmDuration = time.Since(llmStart)
	}

	if err != nil {
		a.emitToast(llmProvider+" error: "+err.Error(), "error")
		a.resetToIdle()
		return
	}

	// Calculate speed
	timeMs := llmDuration.Milliseconds()
	var tps float64 = 0
	if timeMs > 0 && tokenCount > 0 {
		tps = float64(tokenCount) / (float64(timeMs) / 1000.0)
	}

	// Save to history (only polished text is shown, but we still save raw for potential future use)
	if a.historyService != nil {
		_, err := a.historyService.Save("", rawText, polishedText, mode, llmProvider, llmModel, timeMs, tps)
		if err != nil {
			fmt.Printf("Failed to save to history: %v\n", err)
		}
	}

	// Always copy to clipboard first, then inject at cursor and resume media
	if a.injectionService != nil {
		// Run in goroutine to not block timing log if clipboard is slow (unlikely but safe)
		go func() {
			a.injectionService.CopyToClipboard(polishedText)
			fmt.Printf("Text copied to clipboard\n")

			// Inject at cursor using the pre-captured target app bundle ID
			if err := a.injectionService.Inject(polishedText); err != nil {
				fmt.Printf("Could not inject text (no active cursor?): %v\n", err)
			}

			// Restore system audio volume that was muted before recording
			a.volumeMu.Lock()
			vol := a.savedVolume
			a.savedVolume = -1
			a.volumeMu.Unlock()
			audio.UnmuteSystemAudio(vol)
		}()
	}

	fmt.Printf("\n[App] LLM refined output (%d chars):\n%s\n", len(polishedText), polishedText)

	totalProcessingTime := time.Since(processingStartTime)

	// Create formatted output string
	llmName := "Gemini"
	if llmProvider == "openrouter" {
		llmName = "OpenRouter"
	} else if llmProvider == "groq" {
		llmName = "Groq"
	} else if llmProvider == "cerebras" {
		llmName = "Cerebras"
	} else if llmProvider == "local" {
		llmName = "Local"
	}
	output := fmt.Sprintf(
		"\nProcessing Complete:\n"+
			"Audio captured:        %.2fs\n"+
			"Whisper transcription: %.2fs\n"+
			"%s refinement:     %.2fs\n"+
			"Tokens per second:     %.1f t/s\n"+
			"Total processing:      %.2fs\n",
		audioDuration.Seconds(),
		whisperDuration.Seconds(),
		llmName,
		llmDuration.Seconds(),
		tps,
		totalProcessingTime.Seconds(),
	)
	fmt.Println(output)

	// Reset state (but DON'T hide mini mode - let user stay in mini mode if they started there)
	a.state = hotkey.StateIdle
	a.hotkeyManager.SetState(hotkey.StateIdle)
	runtime.EventsEmit(a.ctx, events.StateChanged, "Idle")
	runtime.EventsEmit(a.ctx, events.ProcessingComplete, map[string]interface{}{
		"polished": polishedText,
		"elapsed":  totalProcessingTime.Milliseconds(),
		"details": map[string]float64{
			"audio":   audioDuration.Seconds(),
			"whisper": whisperDuration.Seconds(),
			"llm":     llmDuration.Seconds(),
		},
	})
}

// emitToast sends a toast notification to the frontend
func (a *App) emitToast(message string, toastType string) {
	runtime.EventsEmit(a.ctx, events.Toast, map[string]interface{}{
		"message": message,
		"type":    toastType,
	})
}

// resetToIdle resets the app state to idle (stays in current window mode)
func (a *App) resetToIdle() {
	a.state = hotkey.StateIdle
	a.hotkeyManager.SetState(hotkey.StateIdle)
	runtime.EventsEmit(a.ctx, events.StateChanged, "Idle")

	// Restore system audio if it was muted by VoxFlow (e.g., when processing fails)
	a.volumeMu.Lock()
	vol := a.savedVolume
	a.savedVolume = -1
	a.volumeMu.Unlock()
	if vol >= 0 {
		go func() {
			audio.UnmuteSystemAudio(vol)
		}()
	}
}

// handleError handles errors during processing
func (a *App) handleError(message string, err error) {
	errMsg := message
	if err != nil {
		errMsg = fmt.Sprintf("%s: %v", message, err)
	}
	fmt.Println(errMsg)
	runtime.EventsEmit(a.ctx, events.Error, errMsg)

	a.state = hotkey.StateIdle
	a.hotkeyManager.SetState(hotkey.StateIdle)
	a.HideMiniMode()
	runtime.EventsEmit(a.ctx, events.StateChanged, "Idle")
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
		"hotkey":                 a.config.GetHotkey(),
		"hands_free_hotkey":      a.config.GetHandsFreeHotkey(),
		"push_to_talk_hotkey":    a.config.GetPushToTalkHotkey(),
		"whisper_model":          a.config.GetWhisperModel(),
		"gemini_model":           a.config.GetGeminiModel(),
		"mode":                   a.config.GetMode(),
		"api_key_set":            a.config.GetGeminiAPIKey() != "",
		"llm_provider":           a.config.GetLLMProvider(),
		"openrouter_model":       a.config.GetOpenRouterModel(),
		"openrouter_api_key_set": a.config.GetOpenRouterAPIKey() != "",
		"groq_model":             a.config.GetGroqModel(),
		"groq_api_key_set":       a.config.GetGroqAPIKey() != "",
		"cerebras_model":         a.config.GetCerebrasModel(),
		"cerebras_api_key_set":   a.config.GetCerebrasAPIKey() != "",

		"local_model": a.config.GetLocalModel(),
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
		runtime.EventsEmit(a.ctx, events.ModelDownloadProgress, map[string]interface{}{
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
		runtime.EventsEmit(a.ctx, events.ModelDownloadError, map[string]interface{}{
			"model": modelName,
			"error": err.Error(),
		})
		return err
	}

	runtime.EventsEmit(a.ctx, events.ModelDownloadComplete, modelName)
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
		runtime.EventsEmit(a.ctx, events.ModelDownloadCancelled, nil)
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

// GetLocalModels returns all available local GGUF models
func (a *App) GetLocalModels() ([]localgguf.ModelInfo, error) {
	models, err := a.localGGUFService.GetAllModels()
	if err != nil {
		return nil, err
	}

	installed, err := a.ollamaService.ListInstalledModels(context.Background())
	if err != nil {
		logger.Warnf("[App] Failed to query Ollama for installed models: %v", err)
		installed = make(map[string]int64)
	}

	for i := range models {
		alias := localgguf.GetOllamaModelAlias(models[i].Name)
		if size, ok := installed[alias]; ok {
			models[i].Downloaded = true
			models[i].Size = size
		} else {
			models[i].Downloaded = false
		}
		models[i].FilePath = fmt.Sprintf("ollama:%s", alias)
	}

	return models, nil
}

// DownloadLocalModel downloads a specific local GGUF model (cancellable)
func (a *App) DownloadLocalModel(modelName string) error {
	logger.Infof("[App] DownloadLocalModel called for: %s", modelName)

	a.localDownloadMu.Lock()

	if a.localDownloadCancel != nil {
		logger.Debugf("[App] Cancelling existing download...")
		a.localDownloadCancel()
	}

	ctx, cancel := context.WithCancel(context.Background())
	a.localDownloadCancel = cancel
	a.localDownloadingModel = modelName
	a.localDownloadMu.Unlock()

	// Ensure the Ollama server is running before attempting to pull
	if err := a.ensureLocalModelServer(); err != nil {
		logger.Errorf("[App] Failed to start Ollama server for download: %v", err)
		a.localDownloadMu.Lock()
		a.localDownloadCancel = nil
		a.localDownloadingModel = ""
		a.localDownloadMu.Unlock()

		runtime.EventsEmit(a.ctx, events.LocalModelDownloadError, map[string]interface{}{
			"model": modelName,
			"error": fmt.Sprintf("Failed to start Ollama server: %v", err),
		})
		return err
	}

	alias := localgguf.GetOllamaModelAlias(modelName)
	logger.Debugf("[App] Starting Ollama pull for alias: %s", alias)

	err := a.ollamaService.PullModel(ctx, alias, func(downloaded, total int64) {
		progress := 0.0
		if total > 0 {
			progress = float64(downloaded) / float64(total) * 100
		}
		logger.Debugf("[App] Download progress: %.1f%% (%d/%d units)", progress, downloaded, total)
		runtime.EventsEmit(a.ctx, events.LocalModelDownloadProgress, map[string]interface{}{
			"model":      modelName,
			"downloaded": downloaded,
			"total":      total,
			"progress":   progress,
		})
	})

	a.localDownloadMu.Lock()
	a.localDownloadCancel = nil
	a.localDownloadingModel = ""
	a.localDownloadMu.Unlock()

	if err != nil {
		logger.Errorf("[App] Download failed for %s [%s]: %v", modelName, alias, err)
		runtime.EventsEmit(a.ctx, events.LocalModelDownloadError, map[string]interface{}{
			"model": modelName,
			"error": err.Error(),
		})
		return err
	}

	logger.Infof("[App] Download complete for model: %s", modelName)
	runtime.EventsEmit(a.ctx, events.LocalModelDownloadComplete, modelName)
	return nil
}

// CancelLocalModelDownload cancels any active local model download
func (a *App) CancelLocalModelDownload() {
	a.localDownloadMu.Lock()
	defer a.localDownloadMu.Unlock()

	if a.localDownloadCancel != nil {
		logger.Warn("[App] Cancelling local model download...")
		a.localDownloadCancel()
		a.localDownloadCancel = nil
		a.localDownloadingModel = ""
		runtime.EventsEmit(a.ctx, events.LocalModelDownloadCancelled, nil)
	}
}

// GetActiveLocalDownload returns the name of the currently downloading local model, or empty string if none.
func (a *App) GetActiveLocalDownload() string {
	a.localDownloadMu.Lock()
	defer a.localDownloadMu.Unlock()
	return a.localDownloadingModel
}

// DeleteLocalModel deletes a specific local GGUF model
func (a *App) DeleteLocalModel(modelName string) error {
	activeModel := a.config.GetLocalModel()
	if modelName == activeModel {
		return fmt.Errorf("cannot delete the currently active model")
	}

	alias := localgguf.GetOllamaModelAlias(modelName)
	if err := a.ollamaService.RemoveModel(context.Background(), alias); err != nil {
		return err
	}

	return nil
}

// SetLocalModel sets the active local GGUF model
func (a *App) SetLocalModel(modelName string) error {
	alias := localgguf.GetOllamaModelAlias(modelName)
	installed, err := a.ollamaService.IsModelInstalled(context.Background(), alias)
	if err != nil {
		return err
	}
	if !installed {
		return fmt.Errorf("model %s is not installed. Please download it first", modelName)
	}

	a.config.SetLocalModel(modelName)
	err = a.config.Save()
	if err != nil {
		return err
	}

	if err := a.ensureLocalModelServer(); err != nil {
		return err
	}

	runtime.EventsEmit(a.ctx, events.LocalModelStatus, map[string]interface{}{
		"downloaded": true,
		"loaded":     true,
		"model":      modelName,
	})

	return nil
}

// GetLocalModel returns the currently selected local model
func (a *App) GetLocalModel() string {
	return a.config.GetLocalModel()
}

func (a *App) ensureLocalModelServer() error {
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	return a.ollamaService.EnsureServer(ctx)
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

// ClearAllHistory deletes all transcripts
func (a *App) ClearAllHistory() error {
	if a.historyService == nil {
		return fmt.Errorf("history service not available")
	}
	return a.historyService.DeleteAll()
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
		newPolished, _, err = a.geminiClient.RefineText(transcript.RawText, a.config.GetMode())
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
	runtime.EventsEmit(a.ctx, events.OpenHistory, nil)
}

// OpenSettings emits event to open settings panel
func (a *App) OpenSettings() {
	runtime.EventsEmit(a.ctx, events.OpenSettings, nil)
}

// Quit closes the application
func (a *App) Quit() {
	runtime.Quit(a.ctx)
}

// ToggleFullscreen toggles fullscreen mode (true macOS fullscreen)
func (a *App) ToggleFullscreen() {
	if runtime.WindowIsFullscreen(a.ctx) {
		runtime.WindowUnfullscreen(a.ctx)
	} else {
		runtime.WindowFullscreen(a.ctx)
	}
}

// IsFullscreen returns whether the window is currently fullscreen
func (a *App) IsFullscreen() bool {
	return runtime.WindowIsFullscreen(a.ctx)
}

// ToggleMaximize toggles between normal and maximized window state
func (a *App) ToggleMaximize() {
	if runtime.WindowIsMaximised(a.ctx) {
		runtime.WindowUnmaximise(a.ctx)
	} else {
		runtime.WindowMaximise(a.ctx)
	}
}

// IsMaximized returns whether the window is currently maximized
func (a *App) IsMaximized() bool {
	return runtime.WindowIsMaximised(a.ctx)
}

// Minimize minimizes the window to the dock
func (a *App) Minimize() {
	runtime.WindowMinimise(a.ctx)
}

// IsMinimized returns whether the window is currently minimized
func (a *App) IsMinimized() bool {
	return runtime.WindowIsMinimised(a.ctx)
}
