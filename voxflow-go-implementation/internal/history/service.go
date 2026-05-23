package history

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"voxflow/internal/logger"

	_ "modernc.org/sqlite"
)

// Transcript represents a saved transcription
type Transcript struct {
	ID                int64     `json:"id"`
	Timestamp         time.Time `json:"timestamp"`
	AppName           string    `json:"app_name"`
	RawText           string    `json:"raw_text"`
	PolishedText      string    `json:"polished_text"`
	Mode              string    `json:"mode"`
	LLMProvider       string    `json:"llm_provider"`
	LLMModel          string    `json:"llm_model"`
	TranslationTimeMs int64     `json:"translation_time_ms"`
	TokensPerSecond   float64   `json:"tokens_per_second"`
	WordsPerSecond    float64   `json:"words_per_second"`
}

// Service handles transcript storage and retrieval
type Service struct {
	db *sql.DB
}

const MaxHistoryLimit = 10000
const MaxPageSize = 100

// NewService creates a new history service using the default user configuration directory path.
func NewService() (*Service, error) {
	dbPath, err := getDBPath()
	if err != nil {
		return nil, err
	}
	return NewServiceWithPath(dbPath)
}

// NewServiceWithPath creates a new history service using a custom SQLite database file path.
// This is extremely helpful for testing.
func NewServiceWithPath(dbPath string) (*Service, error) {
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open database at %s: %w", dbPath, err)
	}

	s := &Service{db: db}
	if err := s.initDB(); err != nil {
		db.Close()
		return nil, err
	}

	return s, nil
}

// getDBPath returns the path to the SQLite database
func getDBPath() (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	configDir := filepath.Join(homeDir, ".voxflow")
	if err := os.MkdirAll(configDir, 0755); err != nil {
		return "", err
	}
	return filepath.Join(configDir, "history.db"), nil
}

// initDB initializes the database schema
func (s *Service) initDB() error {
	query := `
	CREATE TABLE IF NOT EXISTS transcripts (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		timestamp DATETIME DEFAULT CURRENT_TIMESTAMP,
		app_name TEXT,
		raw_text TEXT NOT NULL,
		polished_text TEXT,
		mode TEXT,
		llm_provider TEXT,
		llm_model TEXT,
		translation_time_ms INTEGER,
		tokens_per_second REAL,
		words_per_second REAL
	);
	CREATE INDEX IF NOT EXISTS idx_timestamp ON transcripts(timestamp DESC);
	CREATE INDEX IF NOT EXISTS idx_raw_text ON transcripts(raw_text);
	CREATE INDEX IF NOT EXISTS idx_polished_text ON transcripts(polished_text);
	`
	_, err := s.db.Exec(query)
	if err != nil {
		return err
	}

	// Migrations for existing databases
	migrations := []string{
		"ALTER TABLE transcripts ADD COLUMN llm_provider TEXT;",
		"ALTER TABLE transcripts ADD COLUMN llm_model TEXT;",
		"ALTER TABLE transcripts ADD COLUMN translation_time_ms INTEGER;",
		"ALTER TABLE transcripts ADD COLUMN tokens_per_second REAL;",
		"ALTER TABLE transcripts ADD COLUMN words_per_second REAL;",
	}

	for _, m := range migrations {
		if _, err := s.db.Exec(m); err != nil {
			// "duplicate column" is expected when the DB was already migrated — ignore it.
			// Any other error (syntax error, constraint violation, etc.) is a real problem.
			if !strings.Contains(err.Error(), "duplicate column") {
				return fmt.Errorf("migration failed: %w\nSQL: %s", err, m)
			}
		}
	}

	return nil
}

// Save saves a new transcript
func (s *Service) Save(appName, rawText, polishedText, provider, model string, timeMs int64, tps, wps float64) (*Transcript, error) {
	result, err := s.db.Exec(
		"INSERT INTO transcripts (timestamp, app_name, raw_text, polished_text, mode, llm_provider, llm_model, translation_time_ms, tokens_per_second, words_per_second) VALUES (datetime('now'), ?, ?, ?, ?, ?, ?, ?, ?, ?)",
		appName, rawText, polishedText, "", provider, model, timeMs, tps, wps,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to save transcript: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return nil, err
	}

	return s.GetByID(id)
}

// GetByID retrieves a transcript by ID
func (s *Service) GetByID(id int64) (*Transcript, error) {
	row := s.db.QueryRow(
		"SELECT id, timestamp, app_name, raw_text, polished_text, mode, llm_provider, llm_model, translation_time_ms, tokens_per_second, words_per_second FROM transcripts WHERE id = ?",
		id,
	)

	t := &Transcript{}
	var appName, polishedText, mode, provider, model sql.NullString
	var timeMs sql.NullInt64
	var tps sql.NullFloat64
	var wps sql.NullFloat64

	err := row.Scan(&t.ID, &t.Timestamp, &appName, &t.RawText, &polishedText, &mode, &provider, &model, &timeMs, &tps, &wps)
	if err != nil {
		var timestamp string
		err = s.db.QueryRow("SELECT timestamp FROM transcripts WHERE id = ?", id).Scan(&timestamp)
		if err == nil {
			t.Timestamp = parseTimestamp(timestamp)
		} else {
			return nil, err
		}
	}

	t.AppName = appName.String
	t.PolishedText = polishedText.String
	t.Mode = mode.String
	t.LLMProvider = provider.String
	t.LLMModel = model.String
	t.TranslationTimeMs = timeMs.Int64
	t.TokensPerSecond = tps.Float64
	t.WordsPerSecond = wps.Float64

	return t, nil
}

// GetAll retrieves all transcripts ordered by timestamp desc
func (s *Service) GetAll(limit int) ([]*Transcript, error) {
	if limit <= 0 || limit > MaxHistoryLimit {
		limit = MaxHistoryLimit
	}

	query := "SELECT id, timestamp, app_name, raw_text, polished_text, mode, llm_provider, llm_model, translation_time_ms, tokens_per_second, words_per_second FROM transcripts ORDER BY timestamp DESC LIMIT ?"

	rows, err := s.db.Query(query, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var transcripts []*Transcript
	for rows.Next() {
		t, err := s.scanTranscript(rows)
		if err != nil {
			return nil, err
		}
		transcripts = append(transcripts, t)
	}

	return transcripts, nil
}

// Search searches transcripts by text content
func (s *Service) Search(query string, limit int) ([]*Transcript, error) {
	searchQuery := "%" + query + "%"
	sqlQuery := `
		SELECT id, timestamp, app_name, raw_text, polished_text, mode, llm_provider, llm_model, translation_time_ms, tokens_per_second, words_per_second
		FROM transcripts
		WHERE raw_text LIKE ? OR polished_text LIKE ?
		ORDER BY timestamp DESC
	`
	if limit <= 0 || limit > MaxHistoryLimit {
		limit = MaxHistoryLimit
	}

	sqlQuery = strings.TrimSpace(sqlQuery) + " LIMIT ?"

	rows, err := s.db.Query(sqlQuery, searchQuery, searchQuery, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var transcripts []*Transcript
	for rows.Next() {
		t, err := s.scanTranscript(rows)
		if err != nil {
			return nil, err
		}
		transcripts = append(transcripts, t)
	}

	return transcripts, nil
}

// scanTranscript scans the current row into a Transcript struct.
func (s *Service) scanTranscript(rows *sql.Rows) (*Transcript, error) {
	t := &Transcript{}
	var appName, polishedText, mode, provider, model sql.NullString
	var timeMs sql.NullInt64
	var tps sql.NullFloat64
	var wps sql.NullFloat64

	if err := rows.Scan(&t.ID, &t.Timestamp, &appName, &t.RawText, &polishedText, &mode, &provider, &model, &timeMs, &tps, &wps); err != nil {
		return nil, err
	}

	t.AppName = appName.String
	t.PolishedText = polishedText.String
	t.Mode = mode.String
	t.LLMProvider = provider.String
	t.LLMModel = model.String
	t.TranslationTimeMs = timeMs.Int64
	t.TokensPerSecond = tps.Float64
	t.WordsPerSecond = wps.Float64

	return t, nil
}

// GetPage returns a page of transcripts using cursor-based pagination.
// If cursorTS.IsZero(), it starts from the newest entries.
// Returns the transcripts and the cursor (timestamp,id) to use for the next page (older results).
func (s *Service) GetPage(cursorTS time.Time, cursorID int64, limit int) ([]*Transcript, time.Time, int64, error) {
	if limit <= 0 || limit > MaxPageSize {
		limit = MaxPageSize
	}

	var rows *sql.Rows
	var err error

	base := "SELECT id, timestamp, app_name, raw_text, polished_text, mode, llm_provider, llm_model, translation_time_ms, tokens_per_second, words_per_second FROM transcripts"

	if cursorTS.IsZero() {
		query := base + " ORDER BY timestamp DESC, id DESC LIMIT ?"
		rows, err = s.db.Query(query, limit)
	} else {
		query := base + " WHERE (timestamp < ? OR (timestamp = ? AND id < ?)) ORDER BY timestamp DESC, id DESC LIMIT ?"
		rows, err = s.db.Query(query, cursorTS, cursorTS, cursorID, limit)
	}
	if err != nil {
		return nil, time.Time{}, 0, err
	}
	defer rows.Close()

	var transcripts []*Transcript
	var lastTS time.Time
	var lastID int64
	for rows.Next() {
		t, err := s.scanTranscript(rows)
		if err != nil {
			return nil, time.Time{}, 0, err
		}
		transcripts = append(transcripts, t)
		lastTS = t.Timestamp
		lastID = t.ID
	}

	return transcripts, lastTS, lastID, nil
}

// SearchPage searches transcripts with cursor-based pagination.
func (s *Service) SearchPage(q string, cursorTS time.Time, cursorID int64, limit int) ([]*Transcript, time.Time, int64, error) {
	if limit <= 0 || limit > MaxPageSize {
		limit = MaxPageSize
	}

	searchQuery := "%" + q + "%"
	base := `SELECT id, timestamp, app_name, raw_text, polished_text, mode, llm_provider, llm_model, translation_time_ms, tokens_per_second, words_per_second FROM transcripts WHERE (raw_text LIKE ? OR polished_text LIKE ?)`

	var rows *sql.Rows
	var err error
	if cursorTS.IsZero() {
		query := base + " ORDER BY timestamp DESC, id DESC LIMIT ?"
		rows, err = s.db.Query(query, searchQuery, searchQuery, limit)
	} else {
		query := base + " AND (timestamp < ? OR (timestamp = ? AND id < ?)) ORDER BY timestamp DESC, id DESC LIMIT ?"
		rows, err = s.db.Query(query, searchQuery, searchQuery, cursorTS, cursorTS, cursorID, limit)
	}
	if err != nil {
		return nil, time.Time{}, 0, err
	}
	defer rows.Close()

	var transcripts []*Transcript
	var lastTS time.Time
	var lastID int64
	for rows.Next() {
		t, err := s.scanTranscript(rows)
		if err != nil {
			return nil, time.Time{}, 0, err
		}
		transcripts = append(transcripts, t)
		lastTS = t.Timestamp
		lastID = t.ID
	}

	return transcripts, lastTS, lastID, nil
}

// UpdatePolishedText updates the polished text for a transcript
func (s *Service) UpdatePolishedText(id int64, polishedText string) error {
	_, err := s.db.Exec(
		"UPDATE transcripts SET polished_text = ? WHERE id = ?",
		polishedText, id,
	)
	return err
}

// Delete deletes a transcript by ID
func (s *Service) Delete(id int64) error {
	_, err := s.db.Exec("DELETE FROM transcripts WHERE id = ?", id)
	return err
}

// DeleteAll deletes all transcripts
func (s *Service) DeleteAll() error {
	_, err := s.db.Exec("DELETE FROM transcripts")
	return err
}

// Close closes the database connection
func (s *Service) Close() error {
	if s.db != nil {
		return s.db.Close()
	}
	return nil
}

// GetCount returns the total number of transcripts
func (s *Service) GetCount() (int, error) {
	var count int
	err := s.db.QueryRow("SELECT COUNT(*) FROM transcripts").Scan(&count)
	return count, err
}

// parseTimestamp attempts to parse a timestamp string from multiple formats
func parseTimestamp(ts string) time.Time {
	formats := []string{
		"2006-01-02 15:04:05",
		"2006-01-02T15:04:05Z",
		time.RFC3339,
	}
	for _, f := range formats {
		if t, err := time.Parse(f, ts); err == nil {
			return t
		}
	}
	logger.Warnf("Warning: Failed to parse timestamp '%s', using current time", ts)
	return time.Now()
}
