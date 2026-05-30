package orchestrator

import (
	"context"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"
	"voxflow/internal/audio"
	"voxflow/internal/config"
	"voxflow/internal/events"
	"voxflow/internal/history"
	"voxflow/internal/hotkey"
	"voxflow/internal/injection"
	"voxflow/internal/llm"
	"voxflow/internal/logger"
	"voxflow/internal/macos"
	"voxflow/internal/whisper"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// WindowController abstracts mini/full window transitions used during recording.
type WindowController interface {
	ShowMini()
	UserExplicitlyMaximized() bool
}

// Pipeline runs the recording → transcription → refinement → injection flow.
type Pipeline struct {
	ctx context.Context

	config           *config.Config
	audioRecorder    *audio.Recorder
	whisperService   *whisper.Service
	historyService   *history.Service
	injectionService *injection.Service
	hotkeyManager    *hotkey.Manager
	windows          WindowController

	refiner       func() llm.Refiner
	activeLLMModel func() string
	modelReady    func() bool

	stateMu sync.Mutex
	state   hotkey.State

	streamTextMu sync.Mutex
	streamText   string
	streamChunks []streamChunk
	streamJobs   chan streamJob
	streamWG     sync.WaitGroup
	lastEmitTime time.Time

	recordingBundleID string
	recordingAppName  string

	volumeMu    sync.Mutex
	savedVolume int
}

// Config wires dependencies into a Pipeline.
type Config struct {
	Ctx              context.Context
	AppConfig        *config.Config
	Audio            *audio.Recorder
	Whisper          *whisper.Service
	History          *history.Service
	Injection        *injection.Service
	Hotkeys          *hotkey.Manager
	Windows          WindowController
	Refiner          func() llm.Refiner
	ActiveLLMModel   func() string
	ModelReady       func() bool
}

// New creates a recording pipeline.
func New(cfg Config) *Pipeline {
	return &Pipeline{
		ctx:              cfg.Ctx,
		config:           cfg.AppConfig,
		audioRecorder:    cfg.Audio,
		whisperService:   cfg.Whisper,
		historyService:   cfg.History,
		injectionService: cfg.Injection,
		hotkeyManager:    cfg.Hotkeys,
		windows:          cfg.Windows,
		refiner:          cfg.Refiner,
		activeLLMModel:   cfg.ActiveLLMModel,
		modelReady:       cfg.ModelReady,
		state:            hotkey.StateIdle,
		savedVolume:      -1,
	}
}

func (p *Pipeline) SetContext(ctx context.Context) {
	p.ctx = ctx
}

func (p *Pipeline) State() hotkey.State {
	p.stateMu.Lock()
	defer p.stateMu.Unlock()
	return p.state
}

func (p *Pipeline) setState(state hotkey.State) {
	p.stateMu.Lock()
	p.state = state
	p.stateMu.Unlock()
}

// HandleHotkeyState reacts to global hotkey state transitions.
func (p *Pipeline) HandleHotkeyState(state hotkey.State) {
	p.setState(state)
	runtime.EventsEmit(p.ctx, events.StateChanged, state.String())

	switch state {
	case hotkey.StateRecording:
		if p.windows != nil && !p.windows.UserExplicitlyMaximized() {
			p.windows.ShowMini()
		}
		_ = p.StartRecording()
	case hotkey.StateProcessing:
		p.StopRecording()
	case hotkey.StateIdle:
		if p.windows != nil && !p.windows.UserExplicitlyMaximized() {
			p.windows.ShowMini()
		}
	}
}

// StartRecording begins audio capture.
func (p *Pipeline) StartRecording() error {
	if p.modelReady != nil && !p.modelReady() {
		return fmt.Errorf("model not ready")
	}

	p.setState(hotkey.StateRecording)
	if p.hotkeyManager != nil {
		p.hotkeyManager.SetState(hotkey.StateRecording)
	}

	if err := p.audioRecorder.Start(); err != nil {
		p.setState(hotkey.StateIdle)
		if p.hotkeyManager != nil {
			p.hotkeyManager.SetState(hotkey.StateIdle)
		}
		runtime.EventsEmit(p.ctx, events.Error, err.Error())
		return err
	}

	p.startStreamingTranscription()

	if p.config.GetMuteSystemAudio() {
		p.volumeMu.Lock()
		p.savedVolume = -2 // Mute is in progress
		p.volumeMu.Unlock()

		go func() {
			vol := audio.MuteSystemAudio()
			p.volumeMu.Lock()
			defer p.volumeMu.Unlock()
			if p.savedVolume == -1 {
				// The pipeline has already stopped/errored out and restoreVolume was called.
				// We must immediately restore the volume.
				if vol >= 0 {
					go audio.UnmuteSystemAudio(vol)
				}
			} else {
				p.savedVolume = vol
			}
		}()
	}

	runtime.EventsEmit(p.ctx, events.StateChanged, "Recording")
	runtime.EventsEmit(p.ctx, events.RecordingStarted, nil)
	logger.Infof("Recording started...")

	return nil
}

// CaptureRecordingTarget records the frontmost app before VoxFlow takes focus.
// Call this from the hotkey callback before switching to mini mode.
func (p *Pipeline) CaptureRecordingTarget() {
	p.recordingBundleID = ""
	p.recordingAppName = ""
	bundleID, name, err := macos.FrontmostApp()
	if err != nil {
		logger.Debugf("[Pipeline] Could not detect frontmost app: %v", err)
		return
	}
	p.recordingBundleID = bundleID
	p.recordingAppName = name
	logger.Infof("[Pipeline] Recording target app: %s (%s)", name, bundleID)
}

// StopRecording stops capture and begins async processing.
func (p *Pipeline) StopRecording() {
	p.setState(hotkey.StateProcessing)
	if p.hotkeyManager != nil {
		p.hotkeyManager.SetState(hotkey.StateProcessing)
	}
	runtime.EventsEmit(p.ctx, events.StateChanged, "Processing")
	runtime.EventsEmit(p.ctx, events.RecordingStopped, nil)
	logger.Infof("Recording stopped, processing...")

	go p.processRecording()
}

type streamJob struct {
	Samples   []int16
	StartTime time.Duration
	IsFinal   bool
}

func (p *Pipeline) streamingWorker() {
	defer p.streamWG.Done()

	for job := range p.streamJobs {
		samples := job.Samples
		if len(samples) < 1600 {
			if !job.IsFinal {
				audio.RecycleChunk(samples)
			}
			continue
		}

		text, err := p.whisperService.TranscribeSamples(samples)
		if !job.IsFinal {
			audio.RecycleChunk(samples)
		}

		if err != nil || text == "" {
			continue
		}

		text = cleanWhisperText(text)

		p.streamTextMu.Lock()
		p.streamChunks = append(p.streamChunks, streamChunk{Start: job.StartTime, Text: text})
		p.streamText = mergeStreamingChunks(p.streamChunks)
		currentText := p.streamText
		
		// Rate-limit/throttle partial transcript event emissions to prevent visual UI lag (max 10 events/sec)
		shouldEmit := job.IsFinal || time.Since(p.lastEmitTime) >= 100*time.Millisecond
		if shouldEmit {
			p.lastEmitTime = time.Now()
		}
		p.streamTextMu.Unlock()

		if shouldEmit {
			runtime.EventsEmit(p.ctx, events.PartialTranscript, map[string]interface{}{
				"text":      currentText,
				"timestamp": time.Now().Unix(),
			})
		}
	}
}

func (p *Pipeline) startStreamingTranscription() {
	p.streamTextMu.Lock()
	p.streamText = ""
	p.streamChunks = make([]streamChunk, 0)
	p.streamTextMu.Unlock()

	p.streamJobs = make(chan streamJob, 64)
	p.streamWG.Add(1)
	go p.streamingWorker()

	p.audioRecorder.SetChunkCallback(func(samples []int16, startTime time.Duration, isFinal bool) {
		p.streamJobs <- streamJob{
			Samples:   samples,
			StartTime: startTime,
			IsFinal:   isFinal,
		}
	})
}

func (p *Pipeline) processRecording() {
	defer p.restoreVolume()

	processingStartTime := time.Now()

	var stopAndWavDuration time.Duration
	var cleanTextDuration time.Duration
	wavBytes := int64(0)

	audioDuration := p.audioRecorder.GetDuration()

	stopAndWavStart := time.Now()
	wavPath, err := p.audioRecorder.Stop()
	stopAndWavDuration = time.Since(stopAndWavStart)

	// Close the streaming worker channel and wait for any remaining transcriptions to finish.
	if p.streamJobs != nil {
		close(p.streamJobs)
		p.streamWG.Wait()
	}

	p.streamTextMu.Lock()
	streamText := p.streamText
	streamChunkCount := len(p.streamChunks)
	p.streamTextMu.Unlock()

	if streamChunkCount > 0 {
		logger.Infof("[Pipeline] Streaming transcription: %d chunks, %d chars", streamChunkCount, len(streamText))
	}
	if err != nil {
		p.emitToast("Failed to stop recording: "+err.Error(), "error")
		p.resetToIdle()
		return
	}
	defer p.audioRecorder.ClearChunkCallback()
	if info, statErr := os.Stat(wavPath); statErr == nil {
		wavBytes = info.Size()
	}
	defer os.Remove(wavPath)

	if !p.audioRecorder.HasAudioActivity() {
		p.emitToast("No speech detected. Please try speaking louder or check your microphone.", "warning")
		p.resetToIdle()
		return
	}

	var rawText string
	var whisperDuration time.Duration

	streamCoversSec := float64(streamChunkCount) * audio.ChunkDuration
	audioSec := audioDuration.Seconds()
	hasFullCoverage := streamChunkCount > 0 &&
		streamText != "" &&
		streamCoversSec >= audioSec*0.85

	whisperStart := time.Now()
	if hasFullCoverage {
		logger.Infof("[Pipeline] Using streaming transcript (%d chunks, %.1fs of %.1fs)",
			streamChunkCount, streamCoversSec, audioSec)
		rawText = streamText
	} else {
		maxRetries := 3
		for attempt := 1; attempt <= maxRetries; attempt++ {
			rawText, err = p.whisperService.Transcribe(wavPath)
			if err != nil {
				p.emitToast("Transcription failed: "+err.Error(), "error")
				p.resetToIdle()
				return
			}
			if rawText != "" {
				break
			}
			if attempt < maxRetries {
				logger.Infof("[Pipeline] No speech detected, retrying (%d/%d)...", attempt, maxRetries)
				time.Sleep(200 * time.Millisecond)
			}
		}
	}
	cleanStart := time.Now()
	rawText = cleanWhisperText(rawText)
	cleanTextDuration = time.Since(cleanStart)
	whisperDuration = time.Since(whisperStart)

	logger.Debugf("[Pipeline] Whisper raw output (%d chars):\n%s", len(rawText), rawText)

	if rawText == "" {
		p.emitToast("No audio was captured. Please try speaking louder or check your microphone.", "warning")
		p.resetToIdle()
		return
	}

	if rawText == "[BLANK_AUDIO]" || rawText == "(blank audio)" || rawText == "[NO SPEECH]" {
		p.emitToast("No speech detected. Please try speaking into your microphone.", "warning")
		p.resetToIdle()
		return
	}

	llmProvider := p.config.GetLLMProvider()
	llmModel := p.activeLLMModel()

	mode := p.config.ResolveRefinementMode(p.recordingBundleID)

	var polishedText string
	var tokenCount int
	var okToGo bool
	var llmDuration time.Duration

	if mode == "raw" || mode == "copy-only" {
		polishedText = rawText
		okToGo = true
		logger.Infof("[Pipeline] Refinement mode '%s' for app %q — bypassing LLM", mode, p.recordingBundleID)
	} else {
		refiner := p.refiner()
		llmStart := time.Now()
		polishedText, tokenCount, okToGo, err = refiner.RefineText(rawText, llmModel)
		llmDuration = time.Since(llmStart)

		if err != nil {
			p.emitToast(llmProvider+" error: "+err.Error(), "error")
			p.resetToIdle()
			return
		}

		if okToGo {
			polishedText = rawText
		} else if polishedText == "" {
			polishedText = rawText
			p.emitToast("LLM refining failed - using raw transcription", "warning")
		}
	}

	timeMs := llmDuration.Milliseconds()
	var tps float64
	if timeMs > 0 && tokenCount > 0 {
		tps = float64(tokenCount) / (float64(timeMs) / 1000.0)
	}

	wordCount := len(strings.Fields(polishedText))
	totalProcessingTime := time.Since(processingStartTime)
	totalTimeFromStart := audioDuration + totalProcessingTime
	effectiveWPM := 0.0
	effectiveWPS := 0.0
	if totalTimeFromStart > 0 {
		effectiveWPM = float64(wordCount) / totalTimeFromStart.Minutes()
		effectiveWPS = float64(wordCount) / totalTimeFromStart.Seconds()
	}

	var historyDuration time.Duration
	if p.historyService != nil {
		historyStart := time.Now()
		_, err := p.historyService.Save("", rawText, polishedText, llmProvider, llmModel, timeMs, tps, effectiveWPS)
		historyDuration = time.Since(historyStart)
		if err != nil {
			logger.Errorf("Failed to save to history: %v", err)
		}
	}

	if p.injectionService != nil {
		go func() {
			_ = p.injectionService.CopyToClipboard(polishedText)
			logger.Infof("Text copied to clipboard")
		}()
	}

	shouldPaste := mode != "copy-only" && p.config.ShouldInjectPaste(p.recordingBundleID)
	if p.injectionService != nil && shouldPaste {
		if err := p.injectionService.Inject(polishedText); err != nil {
			logger.Warnf("Could not inject text: %v", err)
			p.emitToast("Text injection failed — grant Accessibility permission to VoxFlow in System Preferences → Privacy & Security → Accessibility", "error")
		}
	} else if mode != "copy-only" && !shouldPaste {
		logger.Infof("[Pipeline] Per-app rule: clipboard-only for %q", p.recordingBundleID)
	}

	logger.Debugf("[Pipeline] Output (%d chars):\n%s", len(polishedText), polishedText)

	llmName := providerDisplayName(llmProvider)
	output := fmt.Sprintf(
		"\nProcessing Complete:\n"+
			"Audio captured:        %.2fs\n"+
			"Stop + WAV write:      %.2fs\n"+
			"WAV file size:         %.2f MB\n"+
			"Whisper transcription: %.2fs\n"+
			"Whisper text cleanup:  %.2fs\n"+
			"%s refinement:     %.2fs\n"+
			"History DB write:      %.2fs\n"+
			"Tokens per second:     %.1f t/s\n"+
			"Words per second:      %.2f w/s\n"+
			"Total processing:      %.2fs\n"+
			"Effective WPM:        %.0f\n",
		audioDuration.Seconds(),
		stopAndWavDuration.Seconds(),
		float64(wavBytes)/(1024.0*1024.0),
		whisperDuration.Seconds(),
		cleanTextDuration.Seconds(),
		llmName,
		llmDuration.Seconds(),
		historyDuration.Seconds(),
		tps,
		effectiveWPS,
		totalProcessingTime.Seconds(),
		effectiveWPM,
	)
	logger.Infof("%s", output)

	p.setState(hotkey.StateIdle)
	if p.hotkeyManager != nil {
		p.hotkeyManager.SetState(hotkey.StateIdle)
	}
	runtime.EventsEmit(p.ctx, events.StateChanged, "Idle")
	runtime.EventsEmit(p.ctx, events.ProcessingComplete, map[string]interface{}{
		"polished":         polishedText,
		"raw":              rawText,
		"used_raw":         okToGo,
		"elapsed":          totalProcessingTime.Milliseconds(),
		"words_per_second": effectiveWPS,
		"details": map[string]float64{
			"audio":      audioDuration.Seconds(),
			"stop_wav":   stopAndWavDuration.Seconds(),
			"whisper":    whisperDuration.Seconds(),
			"clean_text": cleanTextDuration.Seconds(),
			"llm":        llmDuration.Seconds(),
			"history":    historyDuration.Seconds(),
			"wav_mb":     float64(wavBytes) / (1024.0 * 1024.0),
		},
	})
}

func providerDisplayName(provider string) string {
	switch provider {
	case "openrouter":
		return "OpenRouter"
	case "groq":
		return "Groq"
	case "cerebras":
		return "Cerebras"
	case "local":
		return "Local"
	default:
		return "Gemini"
	}
}

func (p *Pipeline) emitToast(message, toastType string) {
	runtime.EventsEmit(p.ctx, events.Toast, map[string]interface{}{
		"message": message,
		"type":    toastType,
	})
}

func (p *Pipeline) restoreVolume() {
	p.volumeMu.Lock()
	vol := p.savedVolume
	p.savedVolume = -1
	p.volumeMu.Unlock()

	if vol >= 0 {
		go audio.UnmuteSystemAudio(vol)
	}
}

func (p *Pipeline) resetToIdle() {
	p.setState(hotkey.StateIdle)
	if p.hotkeyManager != nil {
		p.hotkeyManager.SetState(hotkey.StateIdle)
	}
	runtime.EventsEmit(p.ctx, events.StateChanged, "Idle")

	p.restoreVolume()
}

// ToggleRecording toggles between recording and idle.
func (p *Pipeline) ToggleRecording() string {
	switch p.State() {
	case hotkey.StateIdle:
		if err := p.StartRecording(); err != nil {
			return "Error: " + err.Error()
		}
		return "Recording"
	case hotkey.StateRecording:
		p.StopRecording()
		return "Processing"
	default:
		return p.State().String()
	}
}

// RecordingTarget returns the bundle ID and app name captured at recording start.
func (p *Pipeline) RecordingTarget() (bundleID, appName string) {
	return p.recordingBundleID, p.recordingAppName
}
