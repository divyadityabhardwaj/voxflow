package history

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestHistoryService(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "voxflow_history_test_*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	dbPath := filepath.Join(tempDir, "test_history.db")

	s, err := NewServiceWithPath(dbPath)
	if err != nil {
		t.Fatalf("failed to initialize history service with custom path: %v", err)
	}
	defer s.Close()

	// 1. Verify initially empty
	count, err := s.GetCount()
	if err != nil {
		t.Fatalf("GetCount failed: %v", err)
	}
	if count != 0 {
		t.Errorf("expected 0 transcripts, got %d", count)
	}

	// 2. Save transcript
	raw := "Hello this is a test transcription."
	polished := "Hello, this is a test transcription."
	provider := "gemini"
	model := "gemini-1.5-flash"
	timeMs := int64(450)
	tps := 45.5
	wps := 8.2

	t1, err := s.Save("TestApp", raw, polished, provider, model, timeMs, tps, wps)
	if err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	if t1.ID <= 0 {
		t.Errorf("expected valid ID, got %d", t1.ID)
	}
	if t1.RawText != raw {
		t.Errorf("expected raw %q, got %q", raw, t1.RawText)
	}
	if t1.PolishedText != polished {
		t.Errorf("expected polished %q, got %q", polished, t1.PolishedText)
	}
	if t1.LLMProvider != provider {
		t.Errorf("expected provider %q, got %q", provider, t1.LLMProvider)
	}

	// Verify count is 1
	count, _ = s.GetCount()
	if count != 1 {
		t.Errorf("expected 1 transcript, got %d", count)
	}

	// 3. Retrieve by ID
	t2, err := s.GetByID(t1.ID)
	if err != nil {
		t.Fatalf("GetByID failed: %v", err)
	}
	if t2.RawText != raw || t2.PolishedText != polished {
		t.Errorf("retrieved mismatch fields")
	}

	// 4. Update polished text
	newPolished := "Hello, this is a refined test transcription!"
	err = s.UpdatePolishedText(t1.ID, newPolished)
	if err != nil {
		t.Fatalf("UpdatePolishedText failed: %v", err)
	}

	t3, _ := s.GetByID(t1.ID)
	if t3.PolishedText != newPolished {
		t.Errorf("expected updated polished text %q, got %q", newPolished, t3.PolishedText)
	}

	// 5. Search
	results, err := s.Search("refined", 10)
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}
	if len(results) != 1 {
		t.Errorf("expected 1 search result, got %d", len(results))
	}

	resultsEmpty, _ := s.Search("nonexistent", 10)
	if len(resultsEmpty) != 0 {
		t.Errorf("expected 0 search results, got %d", len(resultsEmpty))
	}

	// 6. Pagination (GetPage / SearchPage)
	page, nextTS, nextID, err := s.GetPage(time.Time{}, 0, 10)
	if err != nil {
		t.Fatalf("GetPage failed: %v", err)
	}
	if len(page) != 1 {
		t.Errorf("expected 1 item on page, got %d", len(page))
	}
	if nextTS.IsZero() || nextID != t1.ID {
		t.Errorf("invalid cursor returned")
	}

	// 7. Delete
	err = s.Delete(t1.ID)
	if err != nil {
		t.Fatalf("Delete failed: %v", err)
	}

	count, _ = s.GetCount()
	if count != 0 {
		t.Errorf("expected 0 transcripts after delete, got %d", count)
	}
}
