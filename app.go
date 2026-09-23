package main

import (
	"context"
	"strings"
	"sync"
	"sync/atomic"
	"voxflow/internal/audio"
	"voxflow/internal/cerebras"
	"voxflow/internal/config"
	"voxflow/internal/events"
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
	modelReady       atomic.Bool
	downloadCancel   context.CancelFunc
	downloadMu       sync.Mutex

	warningsMu      sync.Mutex
	domReady        bool
	pendingWarnings []string
}

func NewApp() *App {
	cfg := config.GetInstance()
	app := &App{
		ctx:              context.Background(),
		config:           cfg,
		audioRecorder:    audio.NewRecorder(),
		whisperService:   whisper.NewService(),
		geminiClient:     gemini.NewClient(cfg.GetGeminiAPIKey(), cfg.GetGeminiModel()),
		openRouterClient: openrouter.NewClient(cfg.GetOpenRouterAPIKey()),
		groqClient:       groq.NewClient(cfg.GetGroqAPIKey()),
		cerebrasClient:   cerebras.NewClient(cfg.GetCerebrasAPIKey()),
		localClient:      localclient.NewClient(cfg.GetLocalURL()),
	}
	app.whisperService.SetLanguage(cfg.GetWhisperLanguage())
	app.whisperService.SetThreads(cfg.GetWhisperThreads())
	app.whisperService.SetPrompt(cfg.GetVocabulary())
	llm.SetVocabulary(cfg.GetVocabulary())
	app.windowMgr = window.NewManager(app.ctx, cfg)

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
		OnState:        func(s hotkey.State) { window.SetStatusItemState(s.String()) },
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
		return a.config.GetGeminiModel()
	}
}

func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	a.windowMgr.SetContext(ctx)
	a.pipeline.SetContext(ctx)
	a.pipeline.RestoreAudio()

	window.FloatEverywhere()
	a.windowMgr.WatchFrame()

	openApp := func() {
		a.HideMiniMode()
		runtime.WindowShow(ctx) // status item clicks never activate an accessory app
	}
	window.InstallStatusItem(window.StatusItemCallbacks{
		ToggleRecording: func() {
			// Same as the hotkey path: the frontmost app is the paste target.
			if a.pipeline.State() == hotkey.StateIdle {
				go a.pipeline.CaptureRecordingTarget()
			}
			a.ToggleRecording()
		},
		OpenApp:      openApp,
		OpenSettings: func() { openApp(); a.OpenSettings() },
		Quit:         a.Quit,
	})

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
	go a.ensureValidModel(a.config.GetLLMProvider())

	hfHotkey := a.config.GetHandsFreeHotkey()
	pttHotkey := a.config.GetPushToTalkHotkey()

	logger.Infof("Starting hotkey manager: HF=%s, PTT=%s", hfHotkey, pttHotkey)
	if err := a.hotkeyManager.Start(hfHotkey, pttHotkey); err != nil {
		logger.Errorf("Failed to register hotkeys: %v", err)
		a.warn("Couldn't register a shortcut (" + strings.ReplaceAll(err.Error(), "\n", "; ") + "). Choose another in Settings.")
	}
}

// warn shows a warning toast, holding it until the frontend has loaded if needed.
func (a *App) warn(message string) {
	a.warningsMu.Lock()
	defer a.warningsMu.Unlock()
	if !a.domReady {
		a.pendingWarnings = append(a.pendingWarnings, message)
		return
	}
	emitWarning(a.ctx, message)
}

func (a *App) onDomReady(ctx context.Context) {
	a.warningsMu.Lock()
	defer a.warningsMu.Unlock()
	a.domReady = true
	for _, message := range a.pendingWarnings {
		emitWarning(ctx, message)
	}
	a.pendingWarnings = nil
}

func emitWarning(ctx context.Context, message string) {
	runtime.EventsEmit(ctx, events.Toast, map[string]interface{}{"message": message, "type": "warning"})
}

func (a *App) shutdown(ctx context.Context) {
	a.pipeline.RestoreAudio()
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

// LogFrontendError records errors caught by the frontend's ErrorBoundary and global handlers.
func (a *App) LogFrontendError(message, stack string) {
	logger.Errorf("[Frontend] %s\n%s", message, stack)
}
