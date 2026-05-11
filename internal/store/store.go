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
	CREATE TABLE IF NOT EXISTS discarded_urls (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		url TEXT NOT NULL,
		minion_name TEXT NOT NULL,
		discarded_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		UNIQUE(url, minion_name)
	);
	CREATE TABLE IF NOT EXISTS active_jobs (
		minion_filename TEXT PRIMARY KEY,
		started_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);
	CREATE TABLE IF NOT EXISTS minion_status (
		minion_filename TEXT PRIMARY KEY,
		is_active BOOLEAN
	);
	CREATE TABLE IF NOT EXISTS run_queue (
		minion_filename TEXT PRIMARY KEY,
		requested_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);
	CREATE TABLE IF NOT EXISTS abort_queue (
		minion_filename TEXT PRIMARY KEY,
		requested_at DATETIME DEFAULT CURRENT_TIMESTAMP
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

func (s *Store) IsDiscarded(url, minionFilename string) (bool, error) {
	var count int
	err := s.db.QueryRow("SELECT COUNT(*) FROM discarded_urls WHERE url = ? AND minion_name = ?", url, minionFilename).Scan(&count)
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

func (s *Store) MarkDiscarded(url, minionFilename string) error {
	_, err := s.db.Exec("INSERT OR IGNORE INTO discarded_urls (url, minion_name, discarded_at) VALUES (?, ?, ?)", url, minionFilename, time.Now())
	return err
}

func (s *Store) MarkJobActive(minionFilename string) error {
	_, err := s.db.Exec("INSERT OR REPLACE INTO active_jobs (minion_filename, started_at) VALUES (?, ?)", minionFilename, time.Now())
	return err
}

func (s *Store) MarkJobDone(minionFilename string) error {
	_, err := s.db.Exec("DELETE FROM active_jobs WHERE minion_filename = ?", minionFilename)
	return err
}

func (s *Store) SetMinionStatus(minionFilename string, isActive bool) error {
	_, err := s.db.Exec("INSERT OR REPLACE INTO minion_status (minion_filename, is_active) VALUES (?, ?)", minionFilename, isActive)
	return err
}

func (s *Store) GetActiveMinions() (map[string]bool, error) {
	rows, err := s.db.Query("SELECT minion_filename FROM minion_status WHERE is_active = 1")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	active := make(map[string]bool)
	for rows.Next() {
		var filename string
		if err := rows.Scan(&filename); err == nil {
			active[filename] = true
		}
	}
	return active, nil
}

func (s *Store) GetMinionStatus(minionFilename string) bool {
	var isActive bool
	err := s.db.QueryRow("SELECT is_active FROM minion_status WHERE minion_filename = ?", minionFilename).Scan(&isActive)
	if err != nil {
		return false
	}
	return isActive
}

func (s *Store) QueueRun(minionFilename string) error {
	_, err := s.db.Exec("INSERT OR IGNORE INTO run_queue (minion_filename, requested_at) VALUES (?, ?)", minionFilename, time.Now())
	return err
}

func (s *Store) DequeueRun(minionFilename string) error {
	_, err := s.db.Exec("DELETE FROM run_queue WHERE minion_filename = ?", minionFilename)
	return err
}

func (s *Store) GetRunQueue() ([]string, error) {
	rows, err := s.db.Query("SELECT minion_filename, requested_at FROM run_queue ORDER BY requested_at ASC")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var queue []string
	var toDequeue []string

	for rows.Next() {
		var filename string
		var reqAtStr string
		if err := rows.Scan(&filename, &reqAtStr); err == nil {
			var parsed time.Time
			var parseErr error

			// Try standard formats
			parsed, parseErr = time.Parse(time.RFC3339Nano, reqAtStr)
			if parseErr != nil {
				parsed, parseErr = time.Parse("2006-01-02 15:04:05", reqAtStr)
			}
			if parseErr != nil {
				parsed, parseErr = time.Parse("2006-01-02 15:04:05.999999999-07:00", reqAtStr)
			}

			// Defense in depth: silently discard if stale
			if parseErr == nil && time.Since(parsed) >= 2*time.Minute {
				toDequeue = append(toDequeue, filename)
				continue
			}

			queue = append(queue, filename)
		}
	}

	for _, f := range toDequeue {
		_, _ = s.db.Exec("DELETE FROM run_queue WHERE minion_filename = ?", f)
	}

	return queue, nil
}

func (s *Store) QueueAbort(minionFilename string) error {
	_, err := s.db.Exec("INSERT OR IGNORE INTO abort_queue (minion_filename, requested_at) VALUES (?, ?)", minionFilename, time.Now())
	return err
}

func (s *Store) DequeueAbort(minionFilename string) error {
	_, err := s.db.Exec("DELETE FROM abort_queue WHERE minion_filename = ?", minionFilename)
	return err
}

func (s *Store) GetAbortQueue() ([]string, error) {
	rows, err := s.db.Query("SELECT minion_filename, requested_at FROM abort_queue ORDER BY requested_at ASC")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var queue []string
	var toDequeue []string

	for rows.Next() {
		var filename string
		var reqAtStr string
		if err := rows.Scan(&filename, &reqAtStr); err == nil {
			var parsed time.Time
			var parseErr error

			// Try standard formats
			parsed, parseErr = time.Parse(time.RFC3339Nano, reqAtStr)
			if parseErr != nil {
				parsed, parseErr = time.Parse("2006-01-02 15:04:05", reqAtStr)
			}
			if parseErr != nil {
				parsed, parseErr = time.Parse("2006-01-02 15:04:05.999999999-07:00", reqAtStr)
			}

			// Defense in depth: silently discard if stale
			if parseErr == nil && time.Since(parsed) >= 2*time.Minute {
				toDequeue = append(toDequeue, filename)
				continue
			}

			queue = append(queue, filename)
		}
	}

	for _, f := range toDequeue {
		_, _ = s.db.Exec("DELETE FROM abort_queue WHERE minion_filename = ?", f)
	}

	return queue, nil
}

func (s *Store) GetActiveJobs() (map[string]bool, error) {
	rows, err := s.db.Query("SELECT minion_filename FROM active_jobs")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	active := make(map[string]bool)
	for rows.Next() {
		var filename string
		if err := rows.Scan(&filename); err == nil {
			active[filename] = true
		}
	}
	return active, nil
}

func (s *Store) ClearMinionState(minionFilename string) (int64, error) {
	res1, err := s.db.Exec("DELETE FROM scraped_pages WHERE minion_filename = ?", minionFilename)
	if err != nil {
		return 0, err
	}
	res2, err := s.db.Exec("DELETE FROM discarded_urls WHERE minion_name = ?", minionFilename)
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
	res2, err := s.db.Exec("DELETE FROM discarded_urls")
	if err != nil {
		return 0, err
	}
	_, _ = s.db.Exec("DELETE FROM active_jobs")

	count1, _ := res1.RowsAffected()
	count2, _ := res2.RowsAffected()
	return count1 + count2, nil
}

func (s *Store) ClearActiveJobs() error {
	_, err := s.db.Exec("DELETE FROM active_jobs")
	return err
}
