package main

import (
	"context"
	"regexp"
	"sync"
	"time"
	"voxflow/internal/audio"
	"voxflow/internal/cerebras"
	"voxflow/internal/config"
	"voxflow/internal/gemini"
	"voxflow/internal/groq"
	"voxflow/internal/history"
	"voxflow/internal/hotkey"
	"voxflow/internal/injection"
	"voxflow/internal/llm"
	"voxflow/internal/localclient"
	"voxflow/internal/logger"
	"voxflow/internal/openrouter"
	"voxflow/internal/whisper"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// Mini mode window sizes:
// - collapsed is the minimal visible pill (record + quit only)
// - expanded is the hover-reveal control strip
// Resizing the native window prevents a large transparent hit area.
const (
	miniModeCollapsedW = 84
	miniModeCollapsedH = 44
	miniModeExpandedW  = 140
	miniModeExpandedH  = 44
)

var whisperNoiseMarkerRe = regexp.MustCompile(`(?i)(^|\s)[\[(](audio|music|applause|noise|silence)[\])](\s|$)`)

type streamChunk struct {
	Start time.Duration
	Text  string
}

// App struct holds the application state
type App struct {
	ctx              context.Context
	config           *config.Config
	hotkeyManager    *hotkey.Manager
	state            hotkey.State
	audioRecorder    *audio.Recorder
	whisperService   *whisper.Service
	localClient      *localclient.Client
	geminiClient     *gemini.Client
	openRouterClient *openrouter.Client
	groqClient       *groq.Client
	cerebrasClient   *cerebras.Client
	historyService   *history.Service
	injectionService *injection.Service
	// refiner is the active LLM provider. Swap it when the provider config changes.
	// All pipeline code calls a.refiner.RefineText — no if/else over provider names.
	refiner                 llm.Refiner
	modelReady              bool
	isMiniMode              bool               // Tracks if app is in mini indicator mode
	userExplicitlyMaximized bool               // Tracks if user manually opened full app (don't auto-minimize)
	downloadCancel          context.CancelFunc // Cancel function for active download
	downloadMu              sync.Mutex         // Mutex for download operations
	positionWatchCancel     context.CancelFunc // Cancel function for position polling
	volumeMu                sync.Mutex         // Guards savedVolume
	savedVolume             int                // System volume saved before muting; -1 = not muted

	miniResizeMu     sync.Mutex
	miniResizeCancel context.CancelFunc

	// Streaming transcription state
	streamTextMu sync.Mutex
	streamText   string        // Accumulated streaming text
	streamChunks []streamChunk // Individual chunk results for merging
}

// NewApp creates a new App application struct
func NewApp() *App {
	cfg := config.GetInstance()
	app := &App{
		ctx:              context.Background(),
		config:           cfg,
		state:            hotkey.StateIdle,
		isMiniMode:       true, // Start in mini mode (floating indicator)
		audioRecorder:    audio.NewRecorder(),
		whisperService:   whisper.NewService(),
		geminiClient:     gemini.NewClient(cfg.GetGeminiAPIKey(), cfg.GetGeminiModel()),
		openRouterClient: openrouter.NewClient(cfg.GetOpenRouterAPIKey()),
		groqClient:       groq.NewClient(cfg.GetGroqAPIKey()),
		cerebrasClient:   cerebras.NewClient(cfg.GetCerebrasAPIKey()),
		localClient:      localclient.NewClient(cfg.GetLocalURL()),
		savedVolume:      -1, // -1 = not currently muted by VoxFlow
	}
	app.whisperService.SetLanguage(cfg.GetWhisperLanguage())
	app.whisperService.SetThreads(cfg.GetWhisperThreads())
	app.refiner = app.activeRefiner()

	return app
}

// activeRefiner returns the llm.Refiner for the currently configured provider.
// Call this whenever the provider selection changes.
func (a *App) activeRefiner() llm.Refiner {
	switch a.config.GetLLMProvider() {
	case "openrouter":
		return a.openRouterClient
	case "groq":
		return a.groqClient
	case "cerebras":
		return a.cerebrasClient
	case "local":
		return a.localClient
	default: // "gemini" and anything unrecognised
		return a.geminiClient
	}
}

// activeLLMModel returns the model name string for the currently configured provider.
func (a *App) activeLLMModel() string {
	switch a.config.GetLLMProvider() {
	case "openrouter":
		return a.config.GetOpenRouterModel()
	case "groq":
		return a.config.GetGroqModel()
	case "cerebras":
		return a.config.GetCerebrasModel()
	case "local":
		return a.config.GetLocalModel()
	default: // "gemini"
		model := a.config.GetGeminiModel()
		if model == "" {
			model = "gemini-1.5-flash"
		}
		return model
	}
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
	// NOTE: PortAudio init is deferred until first recording start to avoid
	// background audio threads/polling when the app is idle.

	// Initialize history service
	histService, err := history.NewService()
	if err != nil {
		logger.Warnf("Warning: Failed to initialize history: %v", err)
	} else {
		a.historyService = histService
	}

	// Initialize injection service
	injService, err := injection.NewService(true)
	if err != nil {
		logger.Warnf("Warning: Failed to initialize injection: %v", err)
	} else {
		a.injectionService = injService
		// Check and request Accessibility permission needed for CGEventPost (Cmd+V simulation).
		// This is a one-time prompt — once granted it persists for the app bundle.
		if !injection.IsAccessibilityGranted() {
			logger.Infof("[Injection] Accessibility permission not granted — prompting user")
			injection.PromptAccessibility()
		} else {
			logger.Infof("[Injection] Accessibility permission granted")
		}
	}

	// Clean up any partial model downloads from previous interrupted sessions
	if err := whisper.CleanupPartialDownloads(); err != nil {
		logger.Warnf("Warning: Failed to cleanup partial downloads: %v", err)
	}
	// Check if model is downloaded
	go a.checkModelStatus()

	// Initialize hotkey manager with callback
	a.hotkeyManager = hotkey.NewManager(a.onHotkeyPressed)

	// Register and Start listening for hotkeys
	hfHotkey := a.config.GetHandsFreeHotkey()
	pttHotkey := a.config.GetPushToTalkHotkey()

	logger.Infof("Starting hotkey manager with: HF=%s, PTT=%s", hfHotkey, pttHotkey)
	if err := a.hotkeyManager.Start(hfHotkey, pttHotkey); err != nil {
		logger.Errorf("Failed to start hotkey listener: %v", err)
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

// GetStatus returns the current application status
func (a *App) GetStatus() string {
	return a.state.String()
}

// IsMiniMode returns whether the app is in mini indicator mode
func (a *App) IsMiniMode() bool {
	return a.isMiniMode
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
