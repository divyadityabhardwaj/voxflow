package main

import (
	"fmt"
	"time"
	"voxflow/internal/events"
	"voxflow/internal/history"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// GetHistory returns transcript history
func (a *App) GetHistory(limit int) ([]*history.Transcript, error) {
	if a.historyService == nil {
		return nil, fmt.Errorf("history service not available")
	}
	return a.historyService.GetAll(limit)
}

// HistoryPage is returned for paginated history requests
type HistoryPage struct {
	Transcripts   []*history.Transcript `json:"transcripts"`
	NextCursorTS  string                 `json:"next_cursor_ts"`
	NextCursorID  int64                  `json:"next_cursor_id"`
}

// GetHistoryPage returns a page of transcripts using a cursor (RFC3339 timestamp + id)
// If cursorTS is empty, the newest entries are returned.
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

// SearchHistoryPage searches transcripts with cursor-based pagination.
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

// SearchHistory searches transcript history
func (a *App) SearchHistory(query string, limit int) ([]*history.Transcript, error) {
	if a.historyService == nil {
		return nil, fmt.Errorf("history service not available")
	}
	return a.historyService.Search(query, limit)
}

// GetTranscript returns a single transcript by ID
func (a *App) GetTranscript(id int64) (*history.Transcript, error) {
	if a.historyService == nil {
		return nil, fmt.Errorf("history service not available")
	}
	return a.historyService.GetByID(id)
}

// DeleteTranscript deletes a transcript by ID
func (a *App) DeleteTranscript(id int64) error {
	if a.historyService == nil {
		return fmt.Errorf("history service not available")
	}
	return a.historyService.Delete(id)
}

// ClearAllHistory deletes all transcripts
func (a *App) ClearAllHistory() error {
	if a.historyService == nil {
		return fmt.Errorf("history service not available")
	}
	return a.historyService.DeleteAll()
}

// RetryWithGemini re-processes a transcript with a custom instruction
func (a *App) RetryWithGemini(id int64, instruction string) (string, error) {
	if a.historyService == nil {
		return "", fmt.Errorf("history service not available")
	}

	transcript, err := a.historyService.GetByID(id)
	if err != nil {
		return "", err
	}

	// Use raw text if no instruction, otherwise apply instruction
	var newPolished string
	if instruction == "" {
		newPolished, _, _, err = a.geminiClient.RefineText(transcript.RawText, "")
	} else {
		newPolished, err = a.geminiClient.RetryWithInstruction(transcript.PolishedText, instruction)
	}

	if err != nil {
		return "", err
	}

	// Update in database
	if err := a.historyService.UpdatePolishedText(id, newPolished); err != nil {
		return "", err
	}

	return newPolished, nil
}

// CopyToClipboard copies text to clipboard
func (a *App) CopyToClipboard(text string) error {
	if a.injectionService == nil {
		return fmt.Errorf("injection service not available")
	}
	return a.injectionService.CopyToClipboard(text)
}

// OpenHistoryWindow emits event to open history window
func (a *App) OpenHistoryWindow() {
	runtime.EventsEmit(a.ctx, events.OpenHistory, nil)
}

// OpenSettings emits event to open settings panel
func (a *App) OpenSettings() {
	runtime.EventsEmit(a.ctx, events.OpenSettings, nil)
}
