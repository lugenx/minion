package store

import (
	"database/sql"
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
	CREATE TABLE IF NOT EXISTS scraped_pages (
		url TEXT NOT NULL,
		minion_filename TEXT NOT NULL,
		content_hash TEXT NOT NULL,
		last_scraped_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		PRIMARY KEY (url, minion_filename)
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

func (s *Store) GetPageHash(url, minionFilename string) (string, error) {
	var hash string
	err := s.db.QueryRow("SELECT content_hash FROM scraped_pages WHERE url = ? AND minion_filename = ?", url, minionFilename).Scan(&hash)
	if err != nil {
		if err == sql.ErrNoRows {
			return "", nil
		}
		return "", err
	}
	return hash, nil
}

func (s *Store) UpdatePageHash(url, minionFilename, hash string) error {
	_, err := s.db.Exec(`
		INSERT INTO scraped_pages (url, minion_filename, content_hash, last_scraped_at)
		VALUES (?, ?, ?, ?)
		ON CONFLICT(url, minion_filename) 
		DO UPDATE SET content_hash = excluded.content_hash, last_scraped_at = excluded.last_scraped_at`,
		url, minionFilename, hash, time.Now())
	return err
}

func (s *Store) IsDropped(url, minionFilename string) (bool, error) {
	var count int
	err := s.db.QueryRow("SELECT COUNT(*) FROM dropped_urls WHERE url = ? AND minion_name = ?", url, minionFilename).Scan(&count)
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

func (s *Store) MarkDropped(url, minionFilename string) error {
	_, err := s.db.Exec("INSERT OR IGNORE INTO dropped_urls (url, minion_name, dropped_at) VALUES (?, ?, ?)", url, minionFilename, time.Now())
	return err
}

func (s *Store) ClearMinionState(minionFilename string) (int64, error) {
	res1, err := s.db.Exec("DELETE FROM scraped_pages WHERE minion_filename = ?", minionFilename)
	if err != nil {
		return 0, err
	}
	res2, err := s.db.Exec("DELETE FROM dropped_urls WHERE minion_name = ?", minionFilename)
	if err != nil {
		return 0, err
	}

	count1, _ := res1.RowsAffected()
	count2, _ := res2.RowsAffected()
	return count1 + count2, nil
}

func (s *Store) ClearAllState() (int64, error) {
	res1, err := s.db.Exec("DELETE FROM scraped_pages")
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
