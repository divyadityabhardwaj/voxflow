package main

import (
	"context"
	"fmt"
	goruntime "runtime"
	"strings"
	"voxflow/internal/events"
	"voxflow/internal/logger"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

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

	if err := a.whisperService.LoadModel(modelSize); err != nil {
		logger.Errorf("Failed to load model: %v", err)
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

	go a.optimizeWhisperRuntime()
}

func (a *App) whisperRuntimeProfileKey() string {
	return fmt.Sprintf("%s-%s-cpu%d-%s", goruntime.GOOS, goruntime.GOARCH, goruntime.NumCPU(), a.config.GetWhisperModel())
}

func (a *App) optimizeWhisperRuntime() {
	language := strings.TrimSpace(a.config.GetWhisperLanguage())
	if language == "" {
		language = "en"
	}
	a.whisperService.SetLanguage(language)

	profileKey := a.whisperRuntimeProfileKey()
	threads := a.config.GetWhisperThreads()
	if threads <= 0 || a.config.GetWhisperProfile() != profileKey {
		bestThreads, err := a.whisperService.BenchmarkBestThreads()
		if err != nil {
			logger.Warnf("[Whisper] Thread autotune skipped: %v", err)
		} else if bestThreads > 0 {
			threads = bestThreads
			a.config.SetWhisperThreads(bestThreads)
			a.config.SetWhisperProfile(profileKey)
			a.whisperService.SetThreads(bestThreads)
			if err := a.config.Save(); err != nil {
				logger.Errorf("[Whisper] Failed to persist thread autotune: %v", err)
			}
			logger.Infof("[Whisper] Autotuned threads: %d (%s)", bestThreads, profileKey)
		}
	} else {
		a.whisperService.SetThreads(threads)
	}

	if err := a.whisperService.WarmUp(); err != nil {
		logger.Warnf("[Whisper] Warm-up skipped: %v", err)
	}
}

// IsModelReady returns whether the Whisper model is ready
func (a *App) IsModelReady() bool {
	return a.modelReady
}

// IsModelDownloaded checks if the current Whisper model is downloaded
func (a *App) IsModelDownloaded() bool {
	modelSize := a.config.GetWhisperModel()
	downloaded, _ := a.whisperService.IsModelDownloaded(modelSize)
	return downloaded
}

// DownloadModel downloads the currently configured Whisper model
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

	a.modelReady = true
	runtime.EventsEmit(a.ctx, events.ModelStatus, map[string]interface{}{
		"downloaded": true,
		"loaded":     true,
		"model":      modelSize,
	})

	return nil
}

// DownloadModelByName downloads a specific Whisper model by name (cancellable)
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

// CancelDownload cancels any active Whisper model download
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

// DeleteModelByName deletes a specific Whisper model
func (a *App) DeleteModelByName(modelName string) error {
	activeModel := a.config.GetWhisperModel()
	if modelName == activeModel {
		return fmt.Errorf("cannot delete the currently active model")
	}
	return a.whisperService.DeleteModel(modelName)
}

// IsWhisperCLIReady returns whether whisper-cli is available
func (a *App) IsWhisperCLIReady() bool {
	return a.whisperService.IsWhisperCLIInstalled()
}

// EnsureWhisperCLI ensures whisper-cli is installed
func (a *App) EnsureWhisperCLI() error {
	return a.whisperService.EnsureWhisperCLI(nil)
}
