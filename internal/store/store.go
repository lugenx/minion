package store

import (
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"fmt"
	"time"

	_ "modernc.org/sqlite"
)

type Store struct {
	db *sql.DB
}

// InitStore initializes the SQLite database at the specified path.
func InitStore(dbPath string) (*Store, error) {
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open sqlite db: %w", err)
	}

	createTableQuery := `
	CREATE TABLE IF NOT EXISTS sent_notifications (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		hash_id TEXT NOT NULL UNIQUE,
		minion_name TEXT NOT NULL,
		processed_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);
	CREATE TABLE IF NOT EXISTS dropped_urls (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		url TEXT NOT NULL,
		minion_name TEXT NOT NULL,
		dropped_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		UNIQUE(url, minion_name)
	);`

	_, err = db.Exec(createTableQuery)
	if err != nil {
		return nil, fmt.Errorf("failed to create table: %w", err)
	}

	return &Store{db: db}, nil
}

// Close closes the database connection.
func (s *Store) Close() error {
	return s.db.Close()
}

// GenerateHash creates a unique fingerprint for a specific notification.
func GenerateHash(minionName, url, title string) string {
	data := fmt.Sprintf("%s|%s|%s", minionName, url, title)
	hash := sha256.Sum256([]byte(data))
	return hex.EncodeToString(hash[:])
}

// HasNotified checks if a notification for this specific item has already been sent.
func (s *Store) HasNotified(hashID string) (bool, error) {
	var count int
	err := s.db.QueryRow("SELECT COUNT(*) FROM sent_notifications WHERE hash_id = ?", hashID).Scan(&count)
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

// MarkNotified inserts the notification hash to prevent duplicate alerts.
func (s *Store) MarkNotified(hashID, minionName string) error {
	_, err := s.db.Exec("INSERT OR IGNORE INTO sent_notifications (hash_id, minion_name, processed_at) VALUES (?, ?, ?)", hashID, minionName, time.Now())
	return err
}

// IsDropped checks if a URL has been permanently dropped by the AI for a specific minion.
func (s *Store) IsDropped(url, minionName string) (bool, error) {
	var count int
	err := s.db.QueryRow("SELECT COUNT(*) FROM dropped_urls WHERE url = ? AND minion_name = ?", url, minionName).Scan(&count)
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

// MarkDropped inserts a URL into the dropped table so it is never scraped again.
func (s *Store) MarkDropped(url, minionName string) error {
	_, err := s.db.Exec("INSERT OR IGNORE INTO dropped_urls (url, minion_name, dropped_at) VALUES (?, ?, ?)", url, minionName, time.Now())
	return err
}
