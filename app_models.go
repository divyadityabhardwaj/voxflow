package main

import (
	"context"
	"fmt"
	"voxflow/internal/events"
	"voxflow/internal/logger"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

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

	if err := a.whisperService.LoadModel(modelSize); err != nil {
		logger.Errorf("Failed to load model: %v", err)
		runtime.EventsEmit(a.ctx, events.ModelStatus, map[string]interface{}{
			"downloaded": true,
			"loaded":     false,
			"error":      err.Error(),
		})
		return
	}

	a.modelReady.Store(true)
	runtime.EventsEmit(a.ctx, events.ModelStatus, map[string]interface{}{
		"downloaded": true,
		"loaded":     true,
		"model":      modelSize,
	})

	go a.optimizeWhisperRuntime()
}

func (a *App) optimizeWhisperRuntime() {
	a.whisperService.SetLanguage(a.config.GetWhisperLanguage())
	a.whisperService.SetThreads(a.config.GetWhisperThreads())
	if err := a.whisperService.WarmUp(); err != nil {
		logger.Warnf("[Whisper] Warm-up skipped: %v", err)
	}
}

func (a *App) IsModelReady() bool {
	return a.modelReady.Load()
}

func (a *App) IsModelDownloaded() bool {
	modelSize := a.config.GetWhisperModel()
	downloaded, _ := a.whisperService.IsModelDownloaded(modelSize)
	return downloaded
}

// modelDownloadEvent is the payload of ModelDownloadProgress, ModelDownloadError and
// ModelDownloadComplete.
type modelDownloadEvent struct {
	Model      string  `json:"model"`
	Downloaded int64   `json:"downloaded"`
	Total      int64   `json:"total"`
	Progress   float64 `json:"progress"` // percent
	Error      string  `json:"error"`
}

// DownloadModel downloads the configured model (cancellable via CancelDownload),
// then loads and warms it up like a launch with the model already present.
func (a *App) DownloadModel() error {
	if err := a.DownloadModelByName(a.config.GetWhisperModel()); err != nil {
		return err
	}
	a.checkModelStatus()
	if !a.modelReady.Load() {
		return fmt.Errorf("the speech model downloaded but could not be loaded")
	}
	return nil
}

func (a *App) DownloadModelByName(modelName string) error {
	a.downloadMu.Lock()
	if a.downloadCancel != nil {
		a.downloadCancel()
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	a.downloadCancel = cancel
	a.downloadMu.Unlock()

	ev := modelDownloadEvent{Model: modelName}
	err := a.whisperService.DownloadModelWithContext(ctx, modelName, func(downloaded, total int64) {
		ev.Downloaded, ev.Total = downloaded, total
		ev.Progress = float64(downloaded) / float64(total) * 100
		runtime.EventsEmit(a.ctx, events.ModelDownloadProgress, ev)
	})

	a.downloadMu.Lock()
	a.downloadCancel = nil
	a.downloadMu.Unlock()

	if err != nil {
		ev.Error = err.Error()
		runtime.EventsEmit(a.ctx, events.ModelDownloadError, ev)
		return err
	}

	runtime.EventsEmit(a.ctx, events.ModelDownloadComplete, ev)
	return nil
}

func (a *App) CancelDownload() {
	a.downloadMu.Lock()
	defer a.downloadMu.Unlock()

	if a.downloadCancel != nil {
		logger.Infof("[App] Cancelling download...")
		a.downloadCancel()
		a.downloadCancel = nil
		runtime.EventsEmit(a.ctx, events.ModelDownloadCancelled, nil)
	}
}

func (a *App) DeleteModelByName(modelName string) error {
	activeModel := a.config.GetWhisperModel()
	if modelName == activeModel {
		return fmt.Errorf("cannot delete the currently active model")
	}
	return a.whisperService.DeleteModel(modelName)
}

func (a *App) IsWhisperCLIReady() bool {
	return a.whisperService.IsWhisperCLIInstalled()
}
