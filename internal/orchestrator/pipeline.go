package orchestrator

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
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

// targetApp finds the app a recording will paste into.
var targetApp = macos.TargetApp

// Anything shorter (0.1 s) is not worth a Whisper call.
const minTranscribeSamples = 1600

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
	emit        func(name string, data ...interface{})
	inject      func(text string) error
	typeText    func(text string) error
	copyText    func(text string) error
	focusTarget func(target macos.AppInfo) string // nil skips the focus check
	micStatus   func() string                     // nil skips the permission check

	// lifecycleMu serialises start, stop and cancel, so a stop or second start
	// that races a slow microphone open waits for it instead of interleaving.
	lifecycleMu sync.Mutex
	stateMu     sync.Mutex
	state       hotkey.State
	stream      *streamSession // guarded by lifecycleMu

	targetMu sync.Mutex
	target   macos.AppInfo

	muteMu   sync.Mutex
	muteWant bool
	muteSync sync.Mutex // serialises osascript calls and guards weMuted
	weMuted  bool
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
	}
	p.emit = func(name string, data ...interface{}) {
		runtime.EventsEmit(p.ctx, name, data...)
	}
	if s := cfg.Injection; s != nil {
		p.inject, p.typeText, p.copyText = s.Inject, s.Type, s.CopyToClipboard
	}
	p.focusTarget = focusTarget
	p.micStatus = macos.MicrophoneStatus
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

// setState publishes a state to the hotkey manager, the menu bar and the frontend.
func (p *Pipeline) setState(state hotkey.State) {
	p.stateMu.Lock()
	p.state = state
	p.stateMu.Unlock()
	if p.hotkeyManager != nil {
		p.hotkeyManager.SetState(state)
	}
	if p.onState != nil {
		p.onState(state)
	}
	p.emit(events.StateChanged, state.String())
}

func (p *Pipeline) HandleHotkeyState(state hotkey.State) {
	switch state {
	case hotkey.StateRecording:
		_ = p.StartRecording()
	case hotkey.StateProcessing:
		p.StopRecording()
	case hotkey.StateIdle:
		if p.windows != nil && !p.windows.UserExplicitlyMaximized() {
			p.windows.ShowMini()
		}
	}
}

// StartRecording is the single entry point for every way of starting a
// dictation. It does nothing unless the pipeline is idle.
func (p *Pipeline) StartRecording() error {
	p.lifecycleMu.Lock()
	defer p.lifecycleMu.Unlock()
	if p.State() != hotkey.StateIdle {
		return nil
	}

	if p.modelReady != nil && !p.modelReady() {
		// The hotkey manager has already moved to Recording; put it back.
		p.resetToIdle()
		err := fmt.Errorf("model not ready")
		p.emit(events.Error, err.Error())
		return err
	}

	if err := p.checkMicrophone(); err != nil {
		p.resetToIdle()
		p.emitSettingsToast(err.Error(), "error", "microphone")
		return err
	}

	p.targetMu.Lock()
	p.target = macos.AppInfo{}
	p.targetMu.Unlock()
	p.captureTarget()
	if p.windows != nil && !p.windows.UserExplicitlyMaximized() {
		p.windows.ShowMini()
	}

	if err := p.audioRecorder.Start(); err != nil {
		if !p.audioRecorder.IsRecording() {
			p.resetToIdle()
			p.emit(events.Error, err.Error())
			return err
		}
		logger.Warnf("[Pipeline] Recorder was still running, continuing: %v", err)
	}
	if name := p.audioRecorder.MissingDevice(); name != "" {
		p.emitToast(name+" isn't connected — using the default microphone", "warning")
	}

	p.stream = p.startStreamingTranscription()

	if p.refiner != nil {
		go p.refiner().Prewarm(p.llmModel())
	}
	if p.config.GetMuteSystemAudio() {
		p.setMuted(true)
	}

	// Only now, with the microphone open, tell the user to start talking.
	p.setState(hotkey.StateRecording)
	p.emit(events.RecordingStarted, nil)
	logger.Infof("Recording started...")

	return nil
}

// captureTarget records the app the dictation is for. It runs again at stop,
// since the user may click into the real target while speaking.
func (p *Pipeline) captureTarget() {
	app, err := targetApp()
	if err != nil {
		logger.Debugf("[Pipeline] Could not detect target app: %v", err)
		return
	}
	logger.Infof("[Pipeline] Recording target app: %s (%s)", app.Name, app.BundleID)
	p.targetMu.Lock()
	p.target = app
	p.targetMu.Unlock()
}

// checkMicrophone asks for microphone access the first time, and fails when
// it has been refused.
func (p *Pipeline) checkMicrophone() error {
	if p.micStatus == nil {
		return nil
	}
	switch p.micStatus() {
	case "denied", "restricted":
		return errMicrophoneDenied
	case "notDetermined":
		granted := make(chan bool, 1)
		go func() { granted <- macos.RequestMicrophoneAccess() }() // never on the main thread
		if !<-granted {
			return errMicrophoneDenied
		}
	}
	return nil
}

var errMicrophoneDenied = errors.New("Microphone access is off — turn on VoxFlow in System Settings › Privacy & Security › Microphone")

func (p *Pipeline) StopRecording() {
	p.lifecycleMu.Lock()
	defer p.lifecycleMu.Unlock()
	if p.State() != hotkey.StateRecording {
		return
	}
	p.captureTarget()
	p.setState(hotkey.StateProcessing)
	p.emit(events.RecordingStopped, nil)
	logger.Infof("Recording stopped, processing...")

	stream := p.stream
	p.stream = nil
	go p.processRecording(stream)
}

// CancelRecording discards the current recording: nothing is transcribed, pasted or saved.
func (p *Pipeline) CancelRecording() {
	p.cancelRecording(false)
}

// DiscardRecording cancels without a toast: the hold key turned out to be part
// of a shortcut, so the user never meant to dictate.
func (p *Pipeline) DiscardRecording() {
	p.cancelRecording(true)
}

func (p *Pipeline) cancelRecording(silent bool) {
	p.lifecycleMu.Lock()
	defer p.lifecycleMu.Unlock()
	if p.State() != hotkey.StateRecording {
		return
	}

	stream := p.stream
	p.stream = nil
	wavPath, err := p.audioRecorder.Stop()
	p.audioRecorder.ClearChunkCallback()
	stream.close(true)
	if err == nil {
		os.Remove(wavPath)
	}

	p.resetToIdle()
	if !silent {
		p.emitToast("Cancelled", "info")
	}
	logger.Infof("Recording cancelled")
}

type streamJob struct {
	Samples   []int16
	StartTime time.Duration
	IsFinal   bool
}

func recycle(job streamJob) {
	if !job.IsFinal {
		audio.RecycleChunk(job.Samples)
	}
}

// streamSession transcribes one recording's chunks while it is still being recorded.
type streamSession struct {
	jobs chan streamJob
	wg   sync.WaitGroup

	mu        sync.Mutex
	closed    bool
	cancelled bool
	spans     []span
	lastEmit  time.Time
}

// send never blocks the audio thread, and drops chunks that arrive after close.
func (s *streamSession) send(job streamJob) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		recycle(job)
		return
	}
	select {
	case s.jobs <- job:
		return
	default:
	}
	// Full: drop the oldest chunk. Its range is transcribed again at stop.
	select {
	case old := <-s.jobs:
		recycle(old)
	default:
	}
	select {
	case s.jobs <- job:
	default:
		recycle(job)
	}
}

func (s *streamSession) close(cancel bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.closed {
		s.closed = true
		close(s.jobs)
	}
	s.cancelled = s.cancelled || cancel
}

func (p *Pipeline) streamingWorker(s *streamSession) {
	defer s.wg.Done()

	for job := range s.jobs {
		start := int(job.StartTime * audio.SampleRate / time.Second)
		sp := span{start: start, end: start + len(job.Samples)}

		s.mu.Lock()
		cancelled := s.cancelled
		s.mu.Unlock()
		if cancelled || len(job.Samples) < minTranscribeSamples {
			recycle(job)
			continue
		}

		text, err := p.whisperService.TranscribeSamples(job.Samples)
		if err != nil {
			logger.Errorf("[Pipeline] Streaming chunk transcription error: %v", err)
		}
		sp.text, sp.ok = cleanWhisperChunk(text, p.config.GetVocabulary(), job.Samples), err == nil
		recycle(job)

		s.mu.Lock()
		if s.cancelled {
			s.mu.Unlock()
			continue
		}
		s.spans = append(s.spans, sp)
		currentText := joinSpans(s.spans)
		// Cap partial transcript events at ~10/s to avoid UI lag.
		shouldEmit := job.IsFinal || time.Since(s.lastEmit) >= 100*time.Millisecond
		if shouldEmit {
			s.lastEmit = time.Now()
		}
		s.mu.Unlock()

		if shouldEmit {
			p.emit(events.PartialTranscript, map[string]interface{}{
				"text":      currentText,
				"timestamp": time.Now().Unix(),
			})
		}
	}
}

func (p *Pipeline) startStreamingTranscription() *streamSession {
	s := &streamSession{jobs: make(chan streamJob, 64)}
	s.wg.Add(1)
	go p.streamingWorker(s)

	p.audioRecorder.SetChunkCallback(func(samples []int16, startTime time.Duration, isFinal bool) {
		s.send(streamJob{Samples: samples, StartTime: startTime, IsFinal: isFinal})
	})
	return s
}

// span is a stretch of the recording in samples, with its transcript when ok.
type span struct {
	start, end int
	text       string
	ok         bool
}

// assembleTranscript transcribes, once each, the stretches of [0,total) that no
// ok span covers, then joins all text in time order. A gap that fails is left
// out and reported, so text that was already transcribed is never thrown away.
func assembleTranscript(spans []span, total int, transcribe func(from, to int) (string, error)) (string, error) {
	var covered []span
	for _, s := range spans {
		if s.ok {
			covered = append(covered, s)
		}
	}
	slices.SortStableFunc(covered, func(a, b span) int { return cmp.Compare(a.start, b.start) })

	var all []span
	var errs []error
	fill := func(from, to int) {
		if to-from < minTranscribeSamples {
			return
		}
		text, err := transcribe(from, to)
		if err != nil {
			errs = append(errs, err)
			return
		}
		all = append(all, span{start: from, end: to, text: cleanWhisperText(text), ok: true})
	}

	pos := 0
	for _, s := range covered {
		fill(pos, min(s.start, total))
		all = append(all, s)
		pos = max(pos, s.end)
	}
	fill(pos, total)

	return joinSpans(all), errors.Join(errs...)
}

func joinSpans(spans []span) string {
	sorted := slices.Clone(spans)
	slices.SortStableFunc(sorted, func(a, b span) int { return cmp.Compare(a.start, b.start) })
	var parts []string
	for _, s := range sorted {
		if s.ok && s.text != "" {
			parts = append(parts, s.text)
		}
	}
	return strings.Join(parts, " ")
}

func (p *Pipeline) processRecording(stream *streamSession) {
	processingStartTime := time.Now()

	stopAndWavStart := time.Now()
	wavPath, err := p.audioRecorder.Stop()
	stopAndWavDuration := time.Since(stopAndWavStart)
	// Capture is over, so give the user their sound back before transcription and refinement.
	p.setMuted(false)

	p.audioRecorder.ClearChunkCallback()
	stream.close(false)
	stream.wg.Wait()

	if err != nil {
		p.emitToast("Failed to stop recording: "+err.Error(), "error")
		p.resetToIdle()
		return
	}
	var wavBytes int64
	if info, statErr := os.Stat(wavPath); statErr == nil {
		wavBytes = info.Size()
	}
	defer os.Remove(wavPath)

	audioDuration := p.audioRecorder.GetDuration()

	if p.audioRecorder.AllSilent() {
		p.emitSettingsToast(errMicrophoneDenied.Error(), "warning", "microphone")
		p.resetToIdle()
		return
	}
	if !p.audioRecorder.HasAudioActivity() {
		p.emitToast("No speech detected. Please try speaking louder or check your microphone.", "warning")
		p.resetToIdle()
		return
	}

	whisperStart := time.Now()
	samples := p.audioRecorder.GetBuffer()
	stream.mu.Lock()
	spans := slices.Clone(stream.spans)
	stream.mu.Unlock()
	logger.Infof("[Pipeline] Streaming transcription: %d chunks", len(spans))

	rawText, err := assembleTranscript(spans, len(samples), func(from, to int) (string, error) {
		logger.Infof("[Pipeline] Transcribing %.1fs-%.1fs not covered by streaming",
			float64(from)/audio.SampleRate, float64(to)/audio.SampleRate)
		text, err := p.whisperService.TranscribeSamples(samples[from:to])
		return cleanWhisperChunk(text, p.config.GetVocabulary(), samples[from:to]), err
	})
	cleanStart := time.Now()
	rawText = cleanWhisperText(rawText)
	cleanTextDuration := time.Since(cleanStart)
	whisperDuration := time.Since(whisperStart)

	if err != nil {
		logger.Errorf("[Pipeline] Transcription failed: %v", err)
		if rawText == "" {
			p.emitToast("Transcription failed: "+truncate(err.Error(), maxToastDetail), "error")
			p.resetToIdle()
			return
		}
		p.emitToast("Part of the recording couldn't be transcribed", "warning")
	}

	logger.Debugf("[Pipeline] Whisper raw output: %d chars", len(rawText))

	if rawText == "" {
		p.emitToast("No audio was captured. Please try speaking louder or check your microphone.", "warning")
		p.resetToIdle()
		return
	}

	llmProvider := p.config.GetLLMProvider()
	llmModel := p.llmModel()

	target := p.Target()
	d := p.deliver(rawText, target.BundleID)

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
			if err := p.historyService.SaveAsync(target.Name, rawText, d.text, llmProvider, llmModel, timeMs, tps, effectiveWPS); err != nil {
				logger.Errorf("Failed to save to history: %v", err)
			}
		}()
	}

	logger.Debugf("[Pipeline] Output: %d chars", len(d.text))

	llmName := llm.ProviderByID(llmProvider).Name
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
	p.emit(events.ProcessingComplete, map[string]interface{}{
		"polished":         d.text,
		"raw":              rawText,
		"used_raw":         d.usedRaw,
		"elapsed":          totalProcessingTime.Milliseconds(),
		"words_per_second": effectiveWPS,
		"target_app":       target.Name,
		"method":           d.method,
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
	case !p.config.HasAPIKey(provider):
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

	d.method = p.handOff(d.text, d.method)
	return d
}

// handOff delivers text by method and returns how it actually arrived:
// "paste", "type" or "clipboard".
func (p *Pipeline) handOff(text, method string) string {
	if method != "clipboard" && p.focusTarget != nil {
		if warning := p.focusTarget(p.Target()); warning != "" {
			p.copyOrLog(text)
			p.emitToast(warning, "warning")
			return "clipboard"
		}
	}

	var err error
	switch method {
	case "clipboard":
		p.copyOrLog(text)
		return method
	case "type":
		err = p.typeText(text)
	default:
		method = "paste"
		err = p.inject(text)
	}
	switch {
	case err == nil:
		return method
	case errors.Is(err, injection.ErrNoAccessibility): // the text is already on the clipboard
		p.emitSettingsToast("Couldn't paste — text copied. Press ⌘V, and allow VoxFlow in Accessibility settings", "warning", "accessibility")
	default:
		logger.Warnf("Could not inject text: %v", err)
		p.copyOrLog(text)
		p.emitToast("Couldn't paste — text copied, press ⌘V", "warning")
	}
	return "clipboard"
}

func (p *Pipeline) copyOrLog(text string) {
	if err := p.copyText(text); err != nil {
		logger.Warnf("Could not copy text: %v", err)
	}
}

// focusTarget brings target back to the front if VoxFlow took focus, and
// returns a warning instead when the text should not be pasted.
func focusTarget(target macos.AppInfo) string {
	if target.PID <= 0 {
		return ""
	}
	front, err := macos.FrontmostAppInfo()
	switch {
	case err != nil || front.PID == target.PID:
		return ""
	case macos.IsSelf(front):
		if macos.ActivateApp(target.PID, 500*time.Millisecond) {
			return ""
		}
		return "Couldn't switch back to " + target.Name + " — text copied, press ⌘V"
	default:
		return "Focus moved to " + front.Name + " — text copied, press ⌘V"
	}
}

func (p *Pipeline) llmModel() string {
	if p.activeLLMModel == nil {
		return ""
	}
	return p.activeLLMModel()
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

// emitSettingsToast names the Privacy & Security pane ("microphone" or
// "accessibility") that fixes the problem, so the UI can offer to open it.
func (p *Pipeline) emitSettingsToast(message, toastType, pane string) {
	p.emit(events.Toast, map[string]interface{}{
		"message":  message,
		"type":     toastType,
		"settings": pane,
	})
}

func (p *Pipeline) resetToIdle() {
	p.setState(hotkey.StateIdle)
	p.setMuted(false)
}

func (p *Pipeline) ToggleRecording() string {
	switch p.State() {
	case hotkey.StateIdle:
		if err := p.StartRecording(); err != nil {
			return "Error: " + err.Error()
		}
	case hotkey.StateRecording:
		p.StopRecording()
	}
	return p.State().String()
}

func (p *Pipeline) Target() macos.AppInfo {
	p.targetMu.Lock()
	defer p.targetMu.Unlock()
	return p.target
}

// setMuted asks for the system output to be muted or restored. Each call runs
// in the background and applies the latest request, so a quick start/stop can't
// leave the output muted.
func (p *Pipeline) setMuted(want bool) {
	p.muteMu.Lock()
	p.muteWant = want
	p.muteMu.Unlock()
	go p.syncMute()
}

func (p *Pipeline) syncMute() {
	p.muteSync.Lock()
	defer p.muteSync.Unlock()
	p.muteMu.Lock()
	want := p.muteWant
	p.muteMu.Unlock()

	switch {
	case want && !p.weMuted:
		p.weMuted = muteOutput()
	case !want && p.weMuted:
		unmuteOutput()
		p.weMuted = false
	}
}

// RestoreAudio unmutes the output if this session, or one that crashed, muted it.
func (p *Pipeline) RestoreAudio() {
	p.muteMu.Lock()
	p.muteWant = false
	p.muteMu.Unlock()

	p.muteSync.Lock()
	defer p.muteSync.Unlock()
	if _, err := os.Stat(muteMarkerPath()); p.weMuted || err == nil {
		unmuteOutput()
	}
	p.weMuted = false
}

// The mute flag is used instead of the volume level: the level is never
// touched, so the worst a crash can leave behind is a mute, and the marker
// file lets the next launch undo that too.
func muteMarkerPath() string {
	dir, err := config.GetConfigDir()
	if err != nil {
		return ""
	}
	return filepath.Join(dir, "muted-by-voxflow")
}

// muteOutput mutes the system output unless it already is, and reports whether it did.
func muteOutput() bool {
	out, err := exec.Command("osascript", "-e", `if output muted of (get volume settings) is true then return "already"
set volume with output muted
return "muted"`).Output()
	if err != nil {
		logger.Errorf("[Audio] Could not mute system audio: %v", err)
		return false
	}
	if strings.TrimSpace(string(out)) != "muted" {
		return false
	}
	if err := os.WriteFile(muteMarkerPath(), nil, 0600); err != nil {
		logger.Warnf("[Audio] Could not record mute marker: %v", err)
	}
	logger.Infof("[Audio] Muted system audio")
	return true
}

func unmuteOutput() {
	if err := exec.Command("osascript", "-e", "set volume without output muted").Run(); err != nil {
		logger.Errorf("[Audio] Could not unmute system audio: %v", err)
		return // keep the marker so the next launch tries again
	}
	os.Remove(muteMarkerPath())
	logger.Infof("[Audio] Unmuted system audio")
}
