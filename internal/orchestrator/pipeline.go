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

type WindowController interface {
	ShowMini()
	UserExplicitlyMaximized() bool
}

type Pipeline struct {
	ctx context.Context

	config           *config.Config
	audioRecorder    *audio.Recorder
	whisperService   *whisper.Service
	historyService   *history.Service
	injectionService *injection.Service
	hotkeyManager    *hotkey.Manager
	windows          WindowController

	refiner        func() llm.Refiner
	activeLLMModel func() string
	modelReady     func() bool
	onState        func(hotkey.State)

	// Seams for tests; they default to the Wails runtime and the injection service.
	emit     func(name string, data ...interface{})
	inject   func(text string) error
	typeText func(text string) error
	copyText func(text string) error

	stateMu sync.Mutex
	state   hotkey.State

	streamTextMu sync.Mutex
	streamText   string
	streamChunks []streamChunk
	streamJobs   chan streamJob
	streamWG     sync.WaitGroup
	lastEmitTime time.Time

	targetMu          sync.Mutex
	recordingBundleID string
	recordingAppName  string

	volumeMu    sync.Mutex
	savedVolume int
}

type Config struct {
	Ctx            context.Context
	AppConfig      *config.Config
	Audio          *audio.Recorder
	Whisper        *whisper.Service
	History        *history.Service
	Injection      *injection.Service
	Hotkeys        *hotkey.Manager
	Windows        WindowController
	Refiner        func() llm.Refiner
	ActiveLLMModel func() string
	ModelReady     func() bool
	OnState        func(hotkey.State)
}

func New(cfg Config) *Pipeline {
	p := &Pipeline{
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
		onState:          cfg.OnState,
		state:            hotkey.StateIdle,
		savedVolume:      -1,
	}
	p.emit = func(name string, data ...interface{}) {
		runtime.EventsEmit(p.ctx, name, data...)
	}
	if s := cfg.Injection; s != nil {
		p.inject, p.typeText, p.copyText = s.Inject, s.Type, s.CopyToClipboard
	}
	return p
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
	if p.onState != nil {
		p.onState(state)
	}
}

func (p *Pipeline) HandleHotkeyState(state hotkey.State) {
	p.setState(state)
	p.emit(events.StateChanged, state.String())

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

func (p *Pipeline) StartRecording() error {
	if p.modelReady != nil && !p.modelReady() {
		// The hotkey path has already flipped state to Recording; undo it or the
		// pill and menu bar stay red and the next press tries to stop nothing.
		p.resetToIdle()
		err := fmt.Errorf("model not ready")
		p.emit(events.Error, err.Error())
		return err
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
		p.emit(events.Error, err.Error())
		return err
	}

	p.startStreamingTranscription()

	if p.refiner != nil {
		go p.refiner().Prewarm(p.llmModel())
	}

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

	p.emit(events.StateChanged, "Recording")
	p.emit(events.RecordingStarted, nil)
	logger.Infof("Recording started...")

	return nil
}

func (p *Pipeline) CaptureRecordingTarget() {
	// Clear first: osascript takes a while, and a short dictation that stops before
	// it returns must not inherit the previous recording's app rules.
	p.targetMu.Lock()
	p.recordingBundleID, p.recordingAppName = "", ""
	p.targetMu.Unlock()
	bundleID, name, err := macos.FrontmostApp()
	if err != nil {
		logger.Debugf("[Pipeline] Could not detect frontmost app: %v", err)
	} else {
		logger.Infof("[Pipeline] Recording target app: %s (%s)", name, bundleID)
	}
	p.targetMu.Lock()
	p.recordingBundleID, p.recordingAppName = bundleID, name
	p.targetMu.Unlock()
}

func (p *Pipeline) StopRecording() {
	p.setState(hotkey.StateProcessing)
	if p.hotkeyManager != nil {
		p.hotkeyManager.SetState(hotkey.StateProcessing)
	}
	p.emit(events.StateChanged, "Processing")
	p.emit(events.RecordingStopped, nil)
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

		chunkDuration := time.Duration(float64(len(samples)) / 16000.0 * float64(time.Second))
		text, err := p.whisperService.TranscribeSamples(samples)
		if !job.IsFinal {
			audio.RecycleChunk(samples)
		}

		if err != nil {
			logger.Errorf("[Pipeline] Streaming chunk transcription error: %v", err)
			continue
		}
		if text == "" {
			continue
		}

		text = cleanWhisperText(text)

		p.streamTextMu.Lock()
		p.streamChunks = append(p.streamChunks, streamChunk{Start: job.StartTime, Duration: chunkDuration, Text: text})
		p.streamText = mergeStreamingChunks(p.streamChunks)
		currentText := p.streamText

		// Cap partial transcript events at ~10/s to avoid UI lag.
		shouldEmit := job.IsFinal || time.Since(p.lastEmitTime) >= 100*time.Millisecond
		if shouldEmit {
			p.lastEmitTime = time.Now()
		}
		p.streamTextMu.Unlock()

		if shouldEmit {
			p.emit(events.PartialTranscript, map[string]interface{}{
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
		select {
		case p.streamJobs <- streamJob{
			Samples:   samples,
			StartTime: startTime,
			IsFinal:   isFinal,
		}:
		default:
			// Drop oldest chunk if full — must not block PortAudio.
			select {
			case oldJob := <-p.streamJobs:
				if !oldJob.IsFinal {
					audio.RecycleChunk(oldJob.Samples)
				}
			default:
			}

			select {
			case p.streamJobs <- streamJob{
				Samples:   samples,
				StartTime: startTime,
				IsFinal:   isFinal,
			}:
			default:
				if !isFinal {
					audio.RecycleChunk(samples)
				}
			}
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

	if p.streamJobs != nil {
		close(p.streamJobs)
		p.streamJobs = nil
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

	var streamCoversSec float64
	p.streamTextMu.Lock()
	for _, chunk := range p.streamChunks {
		streamCoversSec += chunk.Duration.Seconds()
	}
	streamText = p.streamText
	streamChunkCount = len(p.streamChunks)
	p.streamTextMu.Unlock()

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
		fullSamples := p.audioRecorder.GetBuffer()
		startIndex := int(streamCoversSec * 16000)
		if startIndex < 0 {
			startIndex = 0
		}
		if startIndex > len(fullSamples) {
			startIndex = len(fullSamples)
		}
		tailSamples := fullSamples[startIndex:]

		var tailText string
		if len(tailSamples) >= 1600 {
			logger.Infof("[Pipeline] Transcribing uncovered tail from %.1fs to %.1fs (%.1fs segment)",
				streamCoversSec, audioSec, float64(len(tailSamples))/16000.0)
			maxRetries := 3
			for attempt := 1; attempt <= maxRetries; attempt++ {
				tailText, err = p.whisperService.TranscribeSamples(tailSamples)
				if err != nil {
					p.emitToast("Tail transcription failed: "+err.Error(), "error")
					p.resetToIdle()
					return
				}
				if tailText != "" || attempt == maxRetries {
					break
				}
				logger.Infof("[Pipeline] No speech detected in tail, retrying (%d/%d)...", attempt, maxRetries)
				time.Sleep(200 * time.Millisecond)
			}
		}

		if streamText != "" {
			rawText = strings.TrimSpace(streamText + " " + cleanWhisperText(tailText))
		} else {
			rawText = cleanWhisperText(tailText)
		}

		if rawText == "" && len(fullSamples) > 0 {
			logger.Infof("[Pipeline] Tail transcription empty, falling back to transcribing full WAV file")
			maxRetries := 3
			for attempt := 1; attempt <= maxRetries; attempt++ {
				rawText, err = p.whisperService.Transcribe(wavPath)
				if err != nil {
					p.emitToast("Full transcription fallback failed: "+err.Error(), "error")
					p.resetToIdle()
					return
				}
				if rawText != "" {
					break
				}
				if attempt < maxRetries {
					logger.Infof("[Pipeline] No speech detected in full fallback, retrying (%d/%d)...", attempt, maxRetries)
					time.Sleep(200 * time.Millisecond)
				}
			}
		}
	}
	cleanStart := time.Now()
	rawText = cleanWhisperText(rawText)
	cleanTextDuration = time.Since(cleanStart)
	whisperDuration = time.Since(whisperStart)

	logger.Debugf("[Pipeline] Whisper raw output: %d chars", len(rawText))

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
	llmModel := p.llmModel()

	// Read the target only now: by this point the frontmost-app lookup has had
	// the whole stop/transcribe window to finish.
	targetBundleID, targetAppName := p.RecordingTarget()
	d := p.deliver(rawText, targetBundleID)

	timeMs := d.llmTime.Milliseconds()
	var tps float64
	if timeMs > 0 && d.tokens > 0 {
		tps = float64(d.tokens) / (float64(timeMs) / 1000.0)
	}

	wordCount := len(strings.Fields(d.text))
	totalProcessingTime := time.Since(processingStartTime)
	totalTimeFromStart := audioDuration + totalProcessingTime
	effectiveWPM := 0.0
	effectiveWPS := 0.0
	if totalTimeFromStart > 0 {
		effectiveWPM = float64(wordCount) / totalTimeFromStart.Minutes()
		effectiveWPS = float64(wordCount) / totalTimeFromStart.Seconds()
	}

	if p.historyService != nil {
		go func() {
			if err := p.historyService.SaveAsync(targetAppName, rawText, d.text, llmProvider, llmModel, timeMs, tps, effectiveWPS); err != nil {
				logger.Errorf("Failed to save to history: %v", err)
			}
		}()
	}

	logger.Debugf("[Pipeline] Output: %d chars", len(d.text))

	llmName := providerDisplayName(llmProvider)
	output := fmt.Sprintf(
		"\nProcessing Complete:\n"+
			"Audio captured:        %.2fs\n"+
			"Stop + WAV write:      %.2fs\n"+
			"WAV file size:         %.2f MB\n"+
			"Whisper transcription: %.2fs\n"+
			"Whisper text cleanup:  %.2fs\n"+
			"%s refinement:     %.2fs\n"+
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
		d.llmTime.Seconds(),
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
	p.emit(events.StateChanged, "Idle")
	p.emit(events.ProcessingComplete, map[string]interface{}{
		"polished":         d.text,
		"raw":              rawText,
		"used_raw":         d.usedRaw,
		"elapsed":          totalProcessingTime.Milliseconds(),
		"words_per_second": effectiveWPS,
		"details": map[string]float64{
			"audio":      audioDuration.Seconds(),
			"stop_wav":   stopAndWavDuration.Seconds(),
			"whisper":    whisperDuration.Seconds(),
			"clean_text": cleanTextDuration.Seconds(),
			"llm":        d.llmTime.Seconds(),
			"wav_mb":     float64(wavBytes) / (1024.0 * 1024.0),
		},
	})
}

type delivery struct {
	text    string
	method  string // "paste", "type" or "clipboard"
	usedRaw bool   // raw text on purpose: raw mode, no key, or the LLM judged it already clean
	tokens  int
	llmTime time.Duration
}

// deliver refines rawText as the mode for bundleID asks, then hands it to the
// target app. A refinement failure always falls back to rawText.
func (p *Pipeline) deliver(rawText, bundleID string) delivery {
	mode := p.config.ResolveRefinementMode(bundleID)
	provider := p.config.GetLLMProvider()
	d := delivery{text: rawText, usedRaw: true}

	switch {
	case mode == "raw" || mode == "copy-only":
		logger.Infof("[Pipeline] Refinement mode '%s' for app %q — bypassing LLM", mode, bundleID)
	case !providerConfigured(p.config, provider):
		// Skipping the key in onboarding shouldn't make every dictation show an error.
		logger.Infof("[Pipeline] No %s API key set — pasting raw transcription", provider)
	default:
		p.emit(events.StateChanged, "Refining")
		llmStart := time.Now()
		text, tokens, okToGo, err := p.refiner().RefineText(rawText, p.llmModel())
		d.llmTime = time.Since(llmStart)

		switch {
		case err != nil:
			// Losing the dictation is worse than pasting it unpolished.
			logger.Warnf("[Pipeline] %s refinement failed, using raw transcription: %v", provider, err)
			p.emitToast(provider+" error: "+truncate(err.Error(), maxToastDetail)+" — pasted raw transcription", "warning")
			d.usedRaw = false
		case okToGo:
			d.tokens = tokens
		case text == "":
			p.emitToast("LLM refining failed - using raw transcription", "warning")
			d.tokens, d.usedRaw = tokens, false
		default:
			d.text, d.tokens, d.usedRaw = text, tokens, false
		}
	}

	if p.inject == nil {
		return d
	}
	d.method = p.config.InjectMethodFor(bundleID)
	if mode == "copy-only" {
		d.method = "clipboard"
	}

	var err error
	switch d.method {
	case "clipboard":
		if copyErr := p.copyText(d.text); copyErr != nil {
			logger.Warnf("Could not copy text: %v", copyErr)
		} else {
			logger.Infof("Text copied to clipboard")
		}
	case "type":
		logger.Infof("[Pipeline] Per-app rule: typing keystrokes for %q", bundleID)
		err = p.typeText(d.text)
	default:
		err = p.inject(d.text)
	}
	if err != nil {
		logger.Warnf("Could not inject text: %v", err)
		p.emitToast("Text injection failed — grant Accessibility permission to VoxFlow in System Preferences → Privacy & Security → Accessibility", "error")
	}
	return d
}

// providerConfigured reports whether refinement can be attempted at all. The
// local server needs no key, only a URL.
func providerConfigured(c *config.Config, provider string) bool {
	switch provider {
	case "openrouter":
		return c.GetOpenRouterAPIKey() != ""
	case "groq":
		return c.GetGroqAPIKey() != ""
	case "cerebras":
		return c.GetCerebrasAPIKey() != ""
	case "local":
		return c.GetLocalURL() != ""
	default:
		return c.GetGeminiAPIKey() != ""
	}
}

func (p *Pipeline) llmModel() string {
	if p.activeLLMModel == nil {
		return ""
	}
	return p.activeLLMModel()
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

// Provider errors can carry whole HTML pages or response bodies.
const maxToastDetail = 160

func truncate(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n]) + "…"
}

func (p *Pipeline) emitToast(message, toastType string) {
	p.emit(events.Toast, map[string]interface{}{
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
	p.emit(events.StateChanged, "Idle")

	p.restoreVolume()
}

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

func (p *Pipeline) RecordingTarget() (bundleID, appName string) {
	p.targetMu.Lock()
	defer p.targetMu.Unlock()
	return p.recordingBundleID, p.recordingAppName
}
