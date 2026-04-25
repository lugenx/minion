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

func InitStore(dbPath string) (*Store, error) {
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open sqlite db: %w", err)
	}

	// Enable Write-Ahead Logging (WAL) for high concurrency
	_, err = db.Exec("PRAGMA journal_mode=WAL;")
	if err != nil {
		return nil, fmt.Errorf("failed to enable WAL mode: %w", err)
	}

	// Set a 5-second busy timeout to queue simultaneous writes instead of crashing
	_, err = db.Exec("PRAGMA busy_timeout=5000;")
	if err != nil {
		return nil, fmt.Errorf("failed to set busy timeout: %w", err)
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

func (s *Store) Close() error {
	return s.db.Close()
}

func GenerateHash(minionName, url, title string) string {
	data := fmt.Sprintf("%s|%s|%s", minionName, url, title)
	hash := sha256.Sum256([]byte(data))
	return hex.EncodeToString(hash[:])
}

func (s *Store) HasNotified(hashID string) (bool, error) {
	var count int
	err := s.db.QueryRow("SELECT COUNT(*) FROM sent_notifications WHERE hash_id = ?", hashID).Scan(&count)
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

func (s *Store) MarkNotified(hashID, minionName string) error {
	_, err := s.db.Exec("INSERT OR IGNORE INTO sent_notifications (hash_id, minion_name, processed_at) VALUES (?, ?, ?)", hashID, minionName, time.Now())
	return err
}

func (s *Store) IsDropped(url, minionName string) (bool, error) {
	var count int
	err := s.db.QueryRow("SELECT COUNT(*) FROM dropped_urls WHERE url = ? AND minion_name = ?", url, minionName).Scan(&count)
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

func (s *Store) MarkDropped(url, minionName string) error {
	_, err := s.db.Exec("INSERT OR IGNORE INTO dropped_urls (url, minion_name, dropped_at) VALUES (?, ?, ?)", url, minionName, time.Now())
	return err
}

func (s *Store) ClearMinionState(minionName string) (int64, error) {
	res1, err := s.db.Exec("DELETE FROM sent_notifications WHERE minion_name = ?", minionName)
	if err != nil {
		return 0, err
	}
	res2, err := s.db.Exec("DELETE FROM dropped_urls WHERE minion_name = ?", minionName)
	if err != nil {
		return 0, err
	}

	count1, _ := res1.RowsAffected()
	count2, _ := res2.RowsAffected()
	return count1 + count2, nil
}

func (s *Store) ClearAllState() (int64, error) {
	res1, err := s.db.Exec("DELETE FROM sent_notifications")
	if err != nil {
		return 0, err
	}
	res2, err := s.db.Exec("DELETE FROM dropped_urls")
	if err != nil {
		return 0, err
	}

	count1, _ := res1.RowsAffected()
	count2, _ := res2.RowsAffected()
	return count1 + count2, nil
}
