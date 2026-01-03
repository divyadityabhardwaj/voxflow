package history

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"time"

	_ "modernc.org/sqlite"
)

// Transcript represents a saved transcription
type Transcript struct {
	ID           int64     `json:"id"`
	Timestamp    time.Time `json:"timestamp"`
	AppName      string    `json:"app_name"`
	RawText      string    `json:"raw_text"`
	PolishedText string    `json:"polished_text"`
	Mode         string    `json:"mode"`
}

// Service handles transcript storage and retrieval
type Service struct {
	db *sql.DB
}

// NewService creates a new history service
func NewService() (*Service, error) {
	dbPath, err := getDBPath()
	if err != nil {
		return nil, err
	}

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	s := &Service{db: db}
	if err := s.initDB(); err != nil {
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
		mode TEXT
	);
	CREATE INDEX IF NOT EXISTS idx_timestamp ON transcripts(timestamp DESC);
	`
	_, err := s.db.Exec(query)
	return err
}

// Save saves a new transcript
func (s *Service) Save(appName, rawText, polishedText, mode string) (*Transcript, error) {
	result, err := s.db.Exec(
		"INSERT INTO transcripts (app_name, raw_text, polished_text, mode) VALUES (?, ?, ?, ?)",
		appName, rawText, polishedText, mode,
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
		"SELECT id, timestamp, app_name, raw_text, polished_text, mode FROM transcripts WHERE id = ?",
		id,
	)

	t := &Transcript{}
	var appName, polishedText, mode sql.NullString
	var timestamp string

	err := row.Scan(&t.ID, &timestamp, &appName, &t.RawText, &polishedText, &mode)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("transcript not found")
		}
		return nil, err
	}

	t.Timestamp, _ = time.Parse("2006-01-02 15:04:05", timestamp)
	t.AppName = appName.String
	t.PolishedText = polishedText.String
	t.Mode = mode.String

	return t, nil
}

// GetAll retrieves all transcripts ordered by timestamp desc
func (s *Service) GetAll(limit int) ([]*Transcript, error) {
	query := "SELECT id, timestamp, app_name, raw_text, polished_text, mode FROM transcripts ORDER BY timestamp DESC"
	if limit > 0 {
		query += fmt.Sprintf(" LIMIT %d", limit)
	}

	rows, err := s.db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var transcripts []*Transcript
	for rows.Next() {
		t := &Transcript{}
		var appName, polishedText, mode sql.NullString
		var timestamp string

		err := rows.Scan(&t.ID, &timestamp, &appName, &t.RawText, &polishedText, &mode)
		if err != nil {
			return nil, err
		}

		t.Timestamp, _ = time.Parse("2006-01-02 15:04:05", timestamp)
		t.AppName = appName.String
		t.PolishedText = polishedText.String
		t.Mode = mode.String

		transcripts = append(transcripts, t)
	}

	return transcripts, nil
}

// Search searches transcripts by text content
func (s *Service) Search(query string, limit int) ([]*Transcript, error) {
	searchQuery := "%" + query + "%"
	sqlQuery := `
		SELECT id, timestamp, app_name, raw_text, polished_text, mode 
		FROM transcripts 
		WHERE raw_text LIKE ? OR polished_text LIKE ?
		ORDER BY timestamp DESC
	`
	if limit > 0 {
		sqlQuery += fmt.Sprintf(" LIMIT %d", limit)
	}

	rows, err := s.db.Query(sqlQuery, searchQuery, searchQuery)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var transcripts []*Transcript
	for rows.Next() {
		t := &Transcript{}
		var appName, polishedText, mode sql.NullString
		var timestamp string

		err := rows.Scan(&t.ID, &timestamp, &appName, &t.RawText, &polishedText, &mode)
		if err != nil {
			return nil, err
		}

		t.Timestamp, _ = time.Parse("2006-01-02 15:04:05", timestamp)
		t.AppName = appName.String
		t.PolishedText = polishedText.String
		t.Mode = mode.String

		transcripts = append(transcripts, t)
	}

	return transcripts, nil
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
