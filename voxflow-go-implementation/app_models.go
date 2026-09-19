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

	if err := a.whisperService.LoadModel(modelSize); err != nil {
		runtime.EventsEmit(a.ctx, events.ModelLoadError, err.Error())
		return err
	}

	a.modelReady.Store(true)
	runtime.EventsEmit(a.ctx, events.ModelStatus, map[string]interface{}{
		"downloaded": true,
		"loaded":     true,
		"model":      modelSize,
	})

	return nil
}

func (a *App) DownloadModelByName(modelName string) error {
	a.downloadMu.Lock()
	if a.downloadCancel != nil {
		a.downloadCancel()
	}
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
