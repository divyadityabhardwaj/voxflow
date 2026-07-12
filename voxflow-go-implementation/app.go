package main

import (
	"context"
	"sync"
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
	"voxflow/internal/orchestrator"
	"voxflow/internal/whisper"
	"voxflow/internal/window"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// App struct holds the application state
type App struct {
	ctx              context.Context
	config           *config.Config
	hotkeyManager    *hotkey.Manager
	windowMgr        *window.Manager
	pipeline         *orchestrator.Pipeline
	audioRecorder    *audio.Recorder
	whisperService   *whisper.Service
	localClient      *localclient.Client
	geminiClient     *gemini.Client
	openRouterClient *openrouter.Client
	groqClient       *groq.Client
	cerebrasClient   *cerebras.Client
	historyService   *history.Service
	injectionService *injection.Service
	modelReady       bool
	downloadCancel   context.CancelFunc
	downloadMu       sync.Mutex
}

// NewApp creates a new App application struct
func NewApp() *App {
	cfg := config.GetInstance()
	app := &App{
		ctx:            context.Background(),
		config:         cfg,
		audioRecorder:  audio.NewRecorder(),
		whisperService: whisper.NewService(),
		geminiClient:   gemini.NewClient(cfg.GetGeminiAPIKey(), cfg.GetGeminiModel()),
		openRouterClient: openrouter.NewClient(cfg.GetOpenRouterAPIKey()),
		groqClient:     groq.NewClient(cfg.GetGroqAPIKey()),
		cerebrasClient: cerebras.NewClient(cfg.GetCerebrasAPIKey()),
		localClient:    localclient.NewClient(cfg.GetLocalURL()),
	}
	app.whisperService.SetLanguage(cfg.GetWhisperLanguage())
	app.whisperService.SetThreads(cfg.GetWhisperThreads())
	app.windowMgr = window.NewManager(app.ctx, cfg)

	// Initialize services that do not require Wails context
	if histService, err := history.NewService(); err != nil {
		logger.Warnf("Warning: Failed to initialize history: %v", err)
	} else {
		app.historyService = histService
	}

	if injService, err := injection.NewService(true); err != nil {
		logger.Warnf("Warning: Failed to initialize injection: %v", err)
	} else {
		app.injectionService = injService
	}

	app.hotkeyManager = hotkey.NewManager(app.onHotkeyPressed)

	app.rebuildPipeline()
	return app
}

func (a *App) rebuildPipeline() {
	a.pipeline = orchestrator.New(orchestrator.Config{
		Ctx:            a.ctx,
		AppConfig:      a.config,
		Audio:          a.audioRecorder,
		Whisper:        a.whisperService,
		History:        a.historyService,
		Injection:      a.injectionService,
		Hotkeys:        a.hotkeyManager,
		Windows:        a.windowMgr,
		Refiner:        a.activeRefiner,
		ActiveLLMModel: a.activeLLMModel,
		ModelReady:     a.IsModelReady,
	})
}

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
	default:
		return a.geminiClient
	}
}

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
	default:
		model := a.config.GetGeminiModel()
		if model == "" {
			model = "gemini-2.0-flash-lite"
		}
		return model
	}
}

func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	a.windowMgr.SetContext(ctx)
	a.pipeline.SetContext(ctx)

	window.FloatEverywhere()

	if !a.config.GetOnboardingCompleted() {
		a.windowMgr.HideMini()
	} else if a.windowMgr.IsMiniMode() {
		a.windowMgr.StartupMiniMode()
	}

	if a.injectionService != nil {
		if !a.config.GetOnboardingCompleted() && !injection.IsAccessibilityGranted() {
			logger.Infof("[Injection] Accessibility not granted — onboarding will prompt")
		} else if !injection.IsAccessibilityGranted() {
			selection, dialogErr := runtime.MessageDialog(a.ctx, runtime.MessageDialogOptions{
				Type:          runtime.QuestionDialog,
				Title:         "Accessibility Permission Required",
				Message:       "VoxFlow uses macOS Accessibility to simulate a Cmd+V paste and inject your refined text directly into target applications.\n\nPlease click \"Grant Permission\", then enable \"VoxFlow\" in the System Settings window that opens.",
				Buttons:       []string{"Grant Permission", "Later"},
				DefaultButton: "Grant Permission",
			})
			if dialogErr == nil && selection == "Grant Permission" {
				injection.PromptAccessibility()
			}
		}
	}

	if err := whisper.CleanupPartialDownloads(); err != nil {
		logger.Warnf("Warning: Failed to cleanup partial downloads: %v", err)
	}

	if err := audio.CleanupTempFiles(); err != nil {
		logger.Warnf("Warning: Failed to cleanup stale temp audio files: %v", err)
	}
	go a.checkModelStatus()

	hfHotkey := a.config.GetHandsFreeHotkey()
	pttHotkey := a.config.GetPushToTalkHotkey()

	logger.Infof("Starting hotkey manager: HF=%s, PTT=%s", hfHotkey, pttHotkey)
	if err := a.hotkeyManager.Start(hfHotkey, pttHotkey); err != nil {
		logger.Errorf("Failed to start hotkey listener: %v", err)
	}
}

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
	a.windowMgr.Shutdown()
	a.config.Save()
}

func (a *App) GetStatus() string {
	return a.pipeline.State().String()
}

func (a *App) IsMiniMode() bool {
	return a.windowMgr.IsMiniMode()
}

func (a *App) Quit() {
	runtime.Quit(a.ctx)
}

func (a *App) ToggleFullscreen() {
	if runtime.WindowIsFullscreen(a.ctx) {
		runtime.WindowUnfullscreen(a.ctx)
	} else {
		runtime.WindowFullscreen(a.ctx)
	}
}

func (a *App) IsFullscreen() bool {
	return runtime.WindowIsFullscreen(a.ctx)
}

func (a *App) ToggleMaximize() {
	if runtime.WindowIsMaximised(a.ctx) {
		runtime.WindowUnmaximise(a.ctx)
	} else {
		runtime.WindowMaximise(a.ctx)
	}
}

func (a *App) IsMaximized() bool {
	return runtime.WindowIsMaximised(a.ctx)
}

func (a *App) Minimize() {
	runtime.WindowMinimise(a.ctx)
}

func (a *App) IsMinimized() bool {
	return runtime.WindowIsMinimised(a.ctx)
}
