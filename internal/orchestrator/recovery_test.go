package orchestrator

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"
	"voxflow/internal/config"
	"voxflow/internal/events"
	"voxflow/internal/hotkey"
)

func TestFailedDictationRetriesSameAudioWithoutPasting(t *testing.T) {
	rec := &recorder{}
	p := newTestPipeline(&config.Config{RefinementMode: "raw"}, &stubRefiner{}, rec, nil)
	ctx, cancel := context.WithCancel(context.Background())
	stream := &streamSession{ctx: ctx, cancel: cancel, jobs: make(chan streamJob), transcribe: func(context.Context, []int16) (string, error) { return "", errors.New("offline") }}
	p.stream, p.state = stream, hotkey.StateProcessing
	audio := &recordedAudio{samples: speech, duration: time.Second}
	p.processAudio(stream, audio, false, time.Now())
	p.finishProcessing(stream)
	if !p.GetFailedRecording().Available {
		t.Fatal("failed recording not recoverable")
	}
	if len(rec.sent) != 0 {
		t.Fatal("failure pasted text")
	}
	done := make(chan struct{})
	emit := p.emit
	var result map[string]interface{}
	p.emit = func(name string, data ...interface{}) {
		emit(name, data...)
		if name == events.ProcessingComplete {
			result = data[0].(map[string]interface{})
		}
		if name == events.StateChanged && data[0] == "Idle" {
			close(done)
		}
	}
	p.cloudTranscriber = func() (Transcribe, error) {
		return func(ctx context.Context, samples []int16) (string, error) {
			if !reflect.DeepEqual(samples, speech) {
				return "", errors.New("audio changed")
			}
			return "Recovered dictation.", nil
		}, nil
	}
	if err := p.RetryFailedRecording(); err != nil {
		t.Fatal(err)
	}
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("retry did not finish")
	}
	p.lifecycleMu.Lock() // finalizer is fully done before inspecting its events
	defer p.lifecycleMu.Unlock()
	if len(rec.sent) != 0 {
		t.Fatal("retry pasted into a stale target")
	}
	if result == nil || result["polished"] != "Recovered dictation." || result["method"] != "none" {
		t.Fatalf("result: %v", result)
	}
	if p.failed != nil {
		t.Fatal("successful retry retained failed audio")
	}
}

func TestRecoveryDiscardExpiryAndMissingKey(t *testing.T) {
	p := newTestPipeline(&config.Config{}, &stubRefiner{}, &recorder{}, nil)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	stream := &streamSession{ctx: ctx}
	p.keepFailed(stream, &recordedAudio{samples: speech}, "offline")
	p.cloudTranscriber = func() (Transcribe, error) { return nil, errors.New("missing key") }
	if p.RetryFailedRecording() == nil || !p.GetFailedRecording().Available {
		t.Fatal("missing key should leave recovery available")
	}
	p.DiscardFailedRecording()
	if p.GetFailedRecording().Available {
		t.Fatal("discard retained audio")
	}
	p.keepFailed(stream, &recordedAudio{samples: speech}, "offline")
	p.failed.expires = time.Now().Add(-time.Second)
	if p.RetryFailedRecording() == nil || p.GetFailedRecording().Available {
		t.Fatal("expired audio was retried")
	}
	cancel()
	p.keepFailed(stream, &recordedAudio{samples: speech}, "cancelled")
	if p.GetFailedRecording().Available {
		t.Fatal("cancelled recording was retained")
	}
}
