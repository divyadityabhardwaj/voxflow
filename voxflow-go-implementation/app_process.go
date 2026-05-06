package main

import (
	"fmt"
	"os"
	"sort"
	"strings"
	"time"
	"voxflow/internal/audio"
	"voxflow/internal/events"
	"voxflow/internal/hotkey"
	"voxflow/internal/logger"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// onHotkeyPressed is called when the global hotkey is pressed
func (a *App) onHotkeyPressed(state hotkey.State) {
	a.state = state
	runtime.EventsEmit(a.ctx, events.StateChanged, state.String())

	switch state {
	case hotkey.StateRecording:
		if !a.userExplicitlyMaximized {
			a.ShowMiniMode()
		}
		a.StartRecording()
	case hotkey.StateProcessing:
		a.StopRecording()
		// Note: HideMiniMode is called after processing completes in processRecording()
	case hotkey.StateIdle:
		// Keep mini mode by default. Only return to full view if the user
		// explicitly maximized the app.
		if !a.userExplicitlyMaximized {
			a.ShowMiniMode()
		}
	}
}

// StartRecording begins audio capture
func (a *App) StartRecording() error {
	if !a.modelReady {
		return fmt.Errorf("model not ready")
	}

	a.state = hotkey.StateRecording
	a.hotkeyManager.SetState(hotkey.StateRecording)

	if err := a.audioRecorder.Start(); err != nil {
		a.state = hotkey.StateIdle
		a.hotkeyManager.SetState(hotkey.StateIdle)
		runtime.EventsEmit(a.ctx, events.Error, err.Error())
		return err
	}

	// Start streaming transcription (chunks processed in background)
	a.StartStreamingTranscription()

	// Mute system audio asynchronously so the PortAudio stream starts capturing
	// immediately — MuteSystemAudio calls osascript which takes ~50ms.
	// Audio captured during that window is harmless silence at the very start.
	go func() {
		vol := audio.MuteSystemAudio()
		a.volumeMu.Lock()
		a.savedVolume = vol
		a.volumeMu.Unlock()
	}()

	runtime.EventsEmit(a.ctx, events.StateChanged, "Recording")
	runtime.EventsEmit(a.ctx, events.RecordingStarted, nil)
	logger.Infof("Recording started...")

	return nil
}

func cleanWhisperText(text string) string {
	text = strings.TrimSpace(text)
	if text == "" {
		return ""
	}
	text = whisperNoiseMarkerRe.ReplaceAllString(text, " ")
	text = strings.Join(strings.Fields(text), " ")
	return strings.TrimSpace(text)
}

// StopRecording stops audio capture and begins processing
func (a *App) StopRecording() {
	a.state = hotkey.StateProcessing
	a.hotkeyManager.SetState(hotkey.StateProcessing)
	runtime.EventsEmit(a.ctx, events.StateChanged, "Processing")
	runtime.EventsEmit(a.ctx, events.RecordingStopped, nil)
	logger.Infof("Recording stopped, processing...")

	go a.processRecording()
}

// StartStreamingTranscription sets up chunk-based transcription during recording
func (a *App) StartStreamingTranscription() {
	a.streamTextMu.Lock()
	a.streamText = ""
	a.streamChunks = make([]streamChunk, 0)
	a.streamTextMu.Unlock()

	// Set up chunk callback for streaming transcription
	a.audioRecorder.SetChunkCallback(func(samples []int16, startTime time.Duration, isFinal bool) {
		if len(samples) < 1600 { // Less than 100ms of audio, skip
			return
		}

		text, err := a.whisperService.TranscribeSamples(samples)
		if err != nil || text == "" {
			return
		}

		// Clean the text
		text = cleanWhisperText(text)

		// Emit for real-time display (frontend can show partial results)
		a.streamTextMu.Lock()
		a.streamChunks = append(a.streamChunks, streamChunk{Start: startTime, Text: text})
		a.streamText = mergeStreamingChunks(a.streamChunks)
		currentText := a.streamText
		a.streamTextMu.Unlock()

		// Emit partial result to frontend
		runtime.EventsEmit(a.ctx, events.PartialTranscript, map[string]interface{}{
			"text":      currentText,
			"timestamp": time.Now().Unix(),
		})
	})
}

// mergeStreamingChunks combines chunks, removing overlapping/duplicate words
func mergeStreamingChunks(chunks []streamChunk) string {
	if len(chunks) == 0 {
		return ""
	}

	ordered := make([]streamChunk, len(chunks))
	copy(ordered, chunks)
	sort.SliceStable(ordered, func(i, j int) bool {
		return ordered[i].Start < ordered[j].Start
	})

	if len(ordered) == 1 {
		return ordered[0].Text
	}

	// Simple merging: just concatenate with space
	// More sophisticated merging could use word-level deduplication
	// but Whisper's word-level timestamps would be needed for that
	result := ordered[0].Text
	for i := 1; i < len(ordered); i++ {
		current := ordered[i].Text
		if result != "" && current != "" {
			// Check if last word of result equals first word of next chunk
			resultWords := strings.Fields(result)
			nextWords := strings.Fields(current)
			if len(resultWords) > 0 && len(nextWords) > 0 {
				lastWord := resultWords[len(resultWords)-1]
				firstWord := nextWords[0]
				if strings.ToLower(lastWord) == strings.ToLower(firstWord) {
					// Skip the first word (it's a duplicate from overlap)
					result += " " + strings.Join(nextWords[1:], " ")
				} else {
					result += " " + current
				}
			} else {
				result += " " + current
			}
		} else if current != "" {
			result += current
		}
	}
	return strings.TrimSpace(result)
}

// processRecording handles the transcription and refinement pipeline
func (a *App) processRecording() {
	// Ensure system audio is always restored, even on panic
	defer func() {
		a.volumeMu.Lock()
		vol := a.savedVolume
		a.savedVolume = -1
		a.volumeMu.Unlock()
		if vol >= 0 {
			audio.UnmuteSystemAudio(vol)
		}
	}()

	processingStartTime := time.Now()

	// Get streaming text that was accumulated (for logging/display)
	a.streamTextMu.Lock()
	streamText := a.streamText
	streamChunkCount := len(a.streamChunks)
	a.streamTextMu.Unlock()

	if streamChunkCount > 0 {
		logger.Infof("[App] Streaming transcription: %d chunks processed, %d chars accumulated", streamChunkCount, len(streamText))
	}

	var stopAndWavDuration time.Duration
	var cleanTextDuration time.Duration
	wavBytes := int64(0)

	// Capture audio duration before stopping (buffer is still valid after Stop until next Start)
	audioDuration := a.audioRecorder.GetDuration()

	// Stop recording and get WAV file
	stopAndWavStart := time.Now()
	wavPath, err := a.audioRecorder.Stop()
	stopAndWavDuration = time.Since(stopAndWavStart)
	if err != nil {
		a.emitToast("Failed to stop recording: "+err.Error(), "error")
		a.resetToIdle()
		return
	}
	defer a.audioRecorder.ClearChunkCallback()
	if info, statErr := os.Stat(wavPath); statErr == nil {
		wavBytes = info.Size()
	}
	defer os.Remove(wavPath) // Clean up temp file

	var rawText string
	var whisperDuration time.Duration

	// --- Streaming-first transcription strategy ---
	// If streaming produced chunks covering the full audio duration, use that
	// accumulated text directly and skip the post-stop full-file Whisper call
	// (saves 1–3 s of latency). Fall back to full transcription when streaming
	// coverage is incomplete or the merged text is empty.
	streamCoversSec := float64(streamChunkCount) * audio.ChunkDuration
	audioSec := audioDuration.Seconds()
	hasFullCoverage := streamChunkCount > 0 &&
		streamText != "" &&
		streamCoversSec >= audioSec*0.85 // 85% threshold to allow last-chunk jitter

	whisperStart := time.Now()
	if hasFullCoverage {
		logger.Infof("[App] Using streaming transcript (%d chunks, %.1fs coverage of %.1fs audio)",
			streamChunkCount, streamCoversSec, audioSec)
		rawText = streamText
	} else {
		// Full-file transcription fallback (original behaviour).
		maxRetries := 3
		for attempt := 1; attempt <= maxRetries; attempt++ {
			rawText, err = a.whisperService.Transcribe(wavPath)
			if err != nil {
				a.emitToast("Transcription failed: "+err.Error(), "error")
				a.resetToIdle()
				return
			}
			if rawText != "" {
				break
			}
			if attempt < maxRetries {
				logger.Infof("[App] No speech detected, retrying (%d/%d)...", attempt, maxRetries)
				time.Sleep(200 * time.Millisecond)
			}
		}
	}
	cleanStart := time.Now()
	rawText = cleanWhisperText(rawText)
	cleanTextDuration = time.Since(cleanStart)
	whisperDuration = time.Since(whisperStart)

	logger.Debugf("[App] Whisper raw output (%d chars):\n%s", len(rawText), rawText)

	if rawText == "" {
		a.emitToast("No audio was captured. Please try speaking louder or check your microphone.", "warning")
		a.resetToIdle()
		return
	}

	// Check for Whisper's blank audio markers
	if rawText == "[BLANK_AUDIO]" || rawText == "(blank audio)" || rawText == "[NO SPEECH]" {
		a.emitToast("No speech detected. Please try speaking into your microphone.", "warning")
		a.resetToIdle()
		return
	}

	// Resolve active model name for the current provider.
	llmProvider := a.config.GetLLMProvider()
	llmModel := a.activeLLMModel()

	// Single dispatch point — no if/else over provider names.
	var polishedText string
	var tokenCount int
	var okToGo bool
	llmStart := time.Now()
	polishedText, tokenCount, okToGo, err = a.refiner.RefineText(rawText, llmModel)
	llmDuration := time.Since(llmStart)

	if err != nil {
		a.emitToast(llmProvider+" error: "+err.Error(), "error")
		a.resetToIdle()
		return
	}

	if okToGo {
		polishedText = rawText
	} else if polishedText == "" {
		polishedText = rawText
		a.emitToast("LLM refining failed - using raw transcription", "warning")
	}

	// Calculate speed
	timeMs := llmDuration.Milliseconds()
	var tps float64 = 0
	if timeMs > 0 && tokenCount > 0 {
		tps = float64(tokenCount) / (float64(timeMs) / 1000.0)
	}

	// Count words and calculate overall transcription speed.
	wordCount := 0
	if len(strings.Fields(polishedText)) > 0 {
		wordCount = len(strings.Fields(polishedText))
	}
	totalProcessingTime := time.Since(processingStartTime)
	totalTimeFromStart := audioDuration + totalProcessingTime
	effectiveWPM := 0.0
	effectiveWPS := 0.0
	if totalTimeFromStart > 0 {
		effectiveWPM = float64(wordCount) / totalTimeFromStart.Minutes()
		effectiveWPS = float64(wordCount) / totalTimeFromStart.Seconds()
	}

	// Save to history (only polished text is shown, but we still save raw for potential future use)
	var historyDuration time.Duration
	if a.historyService != nil {
		historyStart := time.Now()
		_, err := a.historyService.Save("", rawText, polishedText, llmProvider, llmModel, timeMs, tps, effectiveWPS)
		historyDuration = time.Since(historyStart)
		if err != nil {
			logger.Errorf("Failed to save to history: %v", err)
		}
	}

	// Copy to clipboard asynchronously (fast, non-blocking).
	if a.injectionService != nil {
		go func() {
			_ = a.injectionService.CopyToClipboard(polishedText)
			logger.Infof("Text copied to clipboard")
		}()
	}

	// Inject text synchronously BEFORE emitting ProcessingComplete.
	// This ensures the UI state flips to "Done" only after the text is actually
	// in the target app — preventing a race where the user starts typing
	// while injection is still pending.
	if a.injectionService != nil {
		if err := a.injectionService.Inject(polishedText); err != nil {
			logger.Warnf("Could not inject text (no active cursor?): %v", err)
			a.emitToast("Text injection failed — grant Accessibility permission to VoxFlow in System Preferences → Privacy & Security → Accessibility", "error")
		}
	}

	logger.Debugf("[App] LLM refined output (%d chars):\n%s", len(polishedText), polishedText)

	// Create formatted output string
	llmName := "Gemini"
	if llmProvider == "openrouter" {
		llmName = "OpenRouter"
	} else if llmProvider == "groq" {
		llmName = "Groq"
	} else if llmProvider == "cerebras" {
		llmName = "Cerebras"
	} else if llmProvider == "local" {
		llmName = "Local"
	}

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

	// Reset state (but DON'T hide mini mode - let user stay in mini mode if they started there)
	a.state = hotkey.StateIdle
	a.hotkeyManager.SetState(hotkey.StateIdle)
	runtime.EventsEmit(a.ctx, events.StateChanged, "Idle")
	runtime.EventsEmit(a.ctx, events.ProcessingComplete, map[string]interface{}{
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

// emitToast sends a toast notification to the frontend
func (a *App) emitToast(message string, toastType string) {
	runtime.EventsEmit(a.ctx, events.Toast, map[string]interface{}{
		"message": message,
		"type":    toastType,
	})
}

// resetToIdle resets the app state to idle (stays in current window mode)
func (a *App) resetToIdle() {
	a.state = hotkey.StateIdle
	a.hotkeyManager.SetState(hotkey.StateIdle)
	runtime.EventsEmit(a.ctx, events.StateChanged, "Idle")

	// Restore system audio if it was muted by VoxFlow (e.g., when processing fails)
	a.volumeMu.Lock()
	vol := a.savedVolume
	a.savedVolume = -1
	a.volumeMu.Unlock()
	if vol >= 0 {
		go func() {
			audio.UnmuteSystemAudio(vol)
		}()
	}
}

// ToggleRecording toggles between recording and idle states
func (a *App) ToggleRecording() string {
	switch a.state {
	case hotkey.StateIdle:
		if err := a.StartRecording(); err != nil {
			return "Error: " + err.Error()
		}
		return "Recording"
	case hotkey.StateRecording:
		a.StopRecording()
		return "Processing"
	default:
		return a.state.String()
	}
}
