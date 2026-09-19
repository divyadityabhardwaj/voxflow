package main

import (
	"fmt"
	"time"
	"voxflow/internal/events"
	"voxflow/internal/history"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

func (a *App) GetHistory(limit int) ([]*history.Transcript, error) {
	if a.historyService == nil {
		return nil, fmt.Errorf("history service not available")
	}
	return a.historyService.GetAll(limit)
}

type HistoryPage struct {
	Transcripts  []*history.Transcript `json:"transcripts"`
	NextCursorTS string                `json:"next_cursor_ts"`
	NextCursorID int64                 `json:"next_cursor_id"`
}

func (a *App) GetHistoryPage(cursorTS string, cursorID int64, limit int) (*HistoryPage, error) {
	if a.historyService == nil {
		return nil, fmt.Errorf("history service not available")
	}

	var ts time.Time
	var err error
	if cursorTS != "" {
		ts, err = time.Parse(time.RFC3339, cursorTS)
		if err != nil {
			return nil, fmt.Errorf("invalid cursor timestamp: %w", err)
		}
	}

	transcripts, nextTS, nextID, err := a.historyService.GetPage(ts, cursorID, limit)
	if err != nil {
		return nil, err
	}

	var nextTSStr string
	if !nextTS.IsZero() {
		nextTSStr = nextTS.UTC().Format(time.RFC3339)
	}

	return &HistoryPage{Transcripts: transcripts, NextCursorTS: nextTSStr, NextCursorID: nextID}, nil
}

func (a *App) SearchHistoryPage(query string, cursorTS string, cursorID int64, limit int) (*HistoryPage, error) {
	if a.historyService == nil {
		return nil, fmt.Errorf("history service not available")
	}

	var ts time.Time
	var err error
	if cursorTS != "" {
		ts, err = time.Parse(time.RFC3339, cursorTS)
		if err != nil {
			return nil, fmt.Errorf("invalid cursor timestamp: %w", err)
		}
	}

	transcripts, nextTS, nextID, err := a.historyService.SearchPage(query, ts, cursorID, limit)
	if err != nil {
		return nil, err
	}

	var nextTSStr string
	if !nextTS.IsZero() {
		nextTSStr = nextTS.UTC().Format(time.RFC3339)
	}

	return &HistoryPage{Transcripts: transcripts, NextCursorTS: nextTSStr, NextCursorID: nextID}, nil
}

func (a *App) DeleteTranscript(id int64) error {
	if a.historyService == nil {
		return fmt.Errorf("history service not available")
	}
	return a.historyService.Delete(id)
}

func (a *App) ClearAllHistory() error {
	if a.historyService == nil {
		return fmt.Errorf("history service not available")
	}
	return a.historyService.DeleteAll()
}

func (a *App) RetryRefinement(id int64, instruction string) (string, error) {
	if a.historyService == nil {
		return "", fmt.Errorf("history service not available")
	}

	transcript, err := a.historyService.GetByID(id)
	if err != nil {
		return "", err
	}

	activeModel := a.activeLLMModel()

	var newPolished string
	if instruction == "" {
		newPolished, _, _, err = a.activeRefiner().RefineText(transcript.RawText, activeModel)
	} else {
		newPolished, err = a.activeRefiner().RetryWithInstruction(transcript.PolishedText, instruction, activeModel)
	}

	if err != nil {
		return "", err
	}

	if err := a.historyService.UpdatePolishedText(id, newPolished); err != nil {
		return "", err
	}

	return newPolished, nil
}

func (a *App) CopyToClipboard(text string) error {
	if a.injectionService == nil {
		return fmt.Errorf("injection service not available")
	}
	return a.injectionService.CopyToClipboard(text)
}

func (a *App) InjectText(text string) error {
	if a.injectionService == nil {
		return fmt.Errorf("injection service not available")
	}
	return a.injectionService.Inject(text)
}

func (a *App) OpenHistoryWindow() {
	runtime.EventsEmit(a.ctx, events.OpenHistory, nil)
}

func (a *App) OpenSettings() {
	runtime.EventsEmit(a.ctx, events.OpenSettings, nil)
}
