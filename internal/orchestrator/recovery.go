package orchestrator

import (
	"context"
	"errors"
	"time"
	"voxflow/internal/events"
	"voxflow/internal/hotkey"
	"voxflow/internal/whisper"
)

const recoveryLifetime = 5 * time.Minute

type recordedAudio struct {
	samples                []int16
	duration, stopDuration time.Duration
	wavBytes               int64
	selection              string
}

type failedRecording struct {
	audio   *recordedAudio
	message string
	expires time.Time
	timer   *time.Timer
}

// FailedRecordingInfo exposes availability, never audio or selected text.
type FailedRecordingInfo struct {
	Available bool   `json:"available"`
	Message   string `json:"message"`
	ExpiresAt int64  `json:"expires_at"`
}

func (p *Pipeline) GetFailedRecording() FailedRecordingInfo {
	p.lifecycleMu.Lock()
	defer p.lifecycleMu.Unlock()
	if p.failed != nil && !time.Now().Before(p.failed.expires) {
		p.clearFailedLocked()
	}
	return p.failedInfoLocked()
}

func (p *Pipeline) failedInfoLocked() FailedRecordingInfo {
	if p.failed == nil {
		return FailedRecordingInfo{}
	}
	return FailedRecordingInfo{true, p.failed.message, p.failed.expires.UnixMilli()}
}

func (p *Pipeline) keepFailed(stream *streamSession, recording *recordedAudio, message string) {
	p.lifecycleMu.Lock()
	defer p.lifecycleMu.Unlock()
	if stream.ctx.Err() != nil {
		return
	}
	p.clearFailedLocked()
	f := &failedRecording{audio: recording, message: message, expires: time.Now().Add(recoveryLifetime)}
	p.failed = f
	f.timer = time.AfterFunc(recoveryLifetime, func() {
		p.lifecycleMu.Lock()
		defer p.lifecycleMu.Unlock()
		if p.failed == f {
			p.clearFailedLocked()
		}
	})
	p.emit(events.FailedRecordingChanged, p.failedInfoLocked())
}

func (p *Pipeline) clearFailedLocked() {
	if p.failed == nil {
		return
	}
	if p.failed.timer != nil {
		p.failed.timer.Stop()
	}
	p.failed = nil
	p.emit(events.FailedRecordingChanged, FailedRecordingInfo{})
}

func (p *Pipeline) DiscardFailedRecording() {
	p.lifecycleMu.Lock()
	defer p.lifecycleMu.Unlock()
	p.clearFailedLocked()
}

// Retry uses the current speech settings (including a repaired key). Its result
// stays in VoxFlow: the original insertion point or selection may no longer exist.
func (p *Pipeline) RetryFailedRecording() error {
	p.lifecycleMu.Lock()
	defer p.lifecycleMu.Unlock()
	if p.State() != hotkey.StateIdle {
		return errors.New("Wait for the current dictation to finish")
	}
	if p.failed == nil || !time.Now().Before(p.failed.expires) {
		p.clearFailedLocked()
		return errors.New("That recording has expired. Please record again")
	}
	var transcribe Transcribe
	if p.cloudTranscriber != nil {
		var err error
		transcribe, err = p.cloudTranscriber()
		if err != nil {
			return err
		}
	}
	provider, model := "groq", whisper.GroqModel
	if transcribe == nil {
		if p.modelReady != nil && !p.modelReady() {
			return errors.New("The speech model isn't ready. Check Speech settings")
		}
		transcribe = p.whisperService.TranscribeSamplesContext
		provider, model = "local", p.config.GetWhisperModel()
	}
	recording := p.failed.audio
	p.clearFailedLocked()
	ctx, cancel := context.WithCancel(context.Background())
	stream := &streamSession{ctx: ctx, cancel: cancel, jobs: make(chan streamJob), transcribe: transcribe, speechProvider: provider, speechModel: model}
	stream.close(false)
	p.stream = stream
	p.setState(hotkey.StateProcessing)
	go func() {
		defer p.finishProcessing(stream)
		p.processAudio(stream, recording, true, time.Now())
	}()
	return nil
}
