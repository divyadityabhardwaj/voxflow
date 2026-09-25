package main

import (
	"context"
	"strings"
	"sync"
	"sync/atomic"
	"voxflow/internal/audio"
	"voxflow/internal/config"
	"voxflow/internal/events"
	"voxflow/internal/history"
	"voxflow/internal/hotkey"
	"voxflow/internal/injection"
	"voxflow/internal/llm"
	"voxflow/internal/logger"
	"voxflow/internal/macos"
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
	llmClients       map[string]*llm.OpenAIClient
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
		ctx:            context.Background(),
		config:         cfg,
		audioRecorder:  audio.NewRecorder(),
		whisperService: whisper.NewService(),
		llmClients:     map[string]*llm.OpenAIClient{},
	}
	for _, p := range llm.Providers {
		app.llmClients[p.ID] = llm.NewClient(p, cfg.GetAPIKey(p.ID))
	}
	app.llmClients["local"].SetServerURL(cfg.GetLocalURL())
	app.audioRecorder.SetInputDevice(cfg.GetInputDevice())
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
	app.hotkeyManager.OnCancel = app.onHotkeyCancel
	app.hotkeyManager.OnHoldStart = func() { app.pipeline.StartHeldRecording() }
	app.hotkeyManager.OnHoldConfirmed = func() { app.pipeline.ConfirmHold() }
	app.hotkeyManager.OnEditStart = func() { _ = app.pipeline.StartEdit() }

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

func (a *App) llmClient(provider string) *llm.OpenAIClient {
	return a.llmClients[llm.ProviderByID(provider).ID]
}

func (a *App) activeRefiner() llm.Refiner {
	return a.llmClient(a.config.GetLLMProvider())
}

func (a *App) activeLLMModel() string {
	return a.config.GetModel(a.config.GetLLMProvider())
}

func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	macos.StartAppTracking()
	a.windowMgr.SetContext(ctx)
	a.pipeline.SetContext(ctx)
	a.pipeline.RestoreAudio()

	window.FloatEverywhere()
	a.windowMgr.WatchFrame()

	window.InstallStatusItem(window.StatusItemCallbacks{
		ToggleRecording: func() { a.ToggleRecording() },
		CancelRecording: a.CancelRecording,
		OpenApp:         a.showMainWindow,
		OpenSettings:    func() { a.showMainWindow(); a.OpenSettings() },
		Quit:            a.Quit,
		OpenUpdate:      a.openUpdate,
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

	if w := a.config.LoadWarning(); w != "" {
		a.warn(w)
	}

	if err := whisper.CleanupPartialDownloads(); err != nil {
		logger.Warnf("Warning: Failed to cleanup partial downloads: %v", err)
	}

	if err := audio.CleanupTempFiles(); err != nil {
		logger.Warnf("Warning: Failed to cleanup stale temp audio files: %v", err)
	}
	go a.checkModelStatus()
	go a.watchForUpdates()
	go a.ensureValidModel(a.config.GetLLMProvider())

	hfHotkey := a.config.GetHandsFreeHotkey()
	pttHotkey := a.config.GetPushToTalkHotkey()

	logger.Infof("Starting hotkey manager: HF=%s, PTT=%s", hfHotkey, pttHotkey)
	if err := a.hotkeyManager.Start(hfHotkey, pttHotkey, a.config.GetEditHotkey(), a.config.GetPushToTalkKey()); err != nil {
		logger.Errorf("Failed to register hotkeys: %v", err)
		a.warn("Couldn't register a shortcut (" + strings.ReplaceAll(err.Error(), "\n", "; ") + "). Choose another in Settings.")
	}
	if a.config.GetOnboardingCompleted() && !a.hotkeyManager.HoldKeyActive() {
		a.warn("Hold-to-talk needs Accessibility permission for VoxFlow — using " + pttHotkey + " until it's granted.")
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

func (a *App) showMainWindow() {
	a.HideMiniMode()
	runtime.WindowShow(a.ctx) // status item clicks never activate an accessory app
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
