package store

import (
	"database/sql"
	"fmt"
	"time"

	_ "modernc.org/sqlite"

	"minion/internal/character"
)

type Store struct {
	db *sql.DB
}

func InitStore(dbPath string) (*Store, error) {
	// Use DSN pragmas so they apply on every connection the pool opens
	dsn := dbPath + "?_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)"
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to open sqlite db: %w", err)
	}

	// Explicit PRAGMAs as a safety net in case DSN pragmas are not supported
	_, _ = db.Exec("PRAGMA journal_mode=WAL;")
	_, _ = db.Exec("PRAGMA busy_timeout=5000;")

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
	);
	CREATE TABLE IF NOT EXISTS chain_inbox (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		target_minion TEXT NOT NULL,
		item_data TEXT NOT NULL,
		parent_minion TEXT NOT NULL,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);
	CREATE TABLE IF NOT EXISTS character_state (
		minion_filename TEXT PRIMARY KEY,
		total_runs INTEGER DEFAULT 0,
		total_matches INTEGER DEFAULT 0,
		last_results INTEGER DEFAULT 0,
		last_errors INTEGER DEFAULT 0
	);`

	_, err = db.Exec(createTableQuery)
	if err != nil {
		return nil, fmt.Errorf("failed to create tables: %w", err)
	}

	if err := migrateSchema(db); err != nil {
		return nil, fmt.Errorf("schema migration failed: %w", err)
	}

	return &Store{db: db}, nil
}

func migrateSchema(db *sql.DB) error {
	var version int
	_ = db.QueryRow("PRAGMA user_version").Scan(&version)

	if version < 1 {
		if _, err := db.Exec("ALTER TABLE character_state ADD COLUMN character_type TEXT DEFAULT ''"); err != nil {
			return fmt.Errorf("migration v1 alter: %w", err)
		}

		var count int
		_ = db.QueryRow("SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name='pet_state'").Scan(&count)
		if count > 0 {
			_, _ = db.Exec(`
				INSERT OR IGNORE INTO character_state (minion_filename, total_runs, total_matches, last_results, last_errors)
				SELECT minion_filename, total_runs, total_matches, last_results, last_errors FROM pet_state
			`)
		}

		if _, err := db.Exec("PRAGMA user_version = 1"); err != nil {
			return err
		}
	}

	return nil
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
	_, _ = s.db.Exec("DELETE FROM chain_inbox WHERE target_minion = ? OR parent_minion = ?", minionFilename, minionFilename)
	_, _ = s.db.Exec("DELETE FROM run_queue WHERE minion_filename = ?", minionFilename)
	_, _ = s.db.Exec("DELETE FROM character_state WHERE minion_filename = ?", minionFilename)

	count1, _ := res1.RowsAffected()
	count2, _ := res2.RowsAffected()
	return count1 + count2, nil
}

func (s *Store) ClearAllState() (int64, error) {
	var total int64

	res, err := s.db.Exec("DELETE FROM scraped_pages")
	if err != nil {
		return 0, err
	}
	n, _ := res.RowsAffected()
	total += n

	res, err = s.db.Exec("DELETE FROM discarded_urls")
	if err != nil {
		return 0, err
	}
	n, _ = res.RowsAffected()
	total += n

	for _, table := range []string{"active_jobs", "abort_queue", "run_queue", "chain_inbox", "minion_status", "character_state"} {
		res, _ = s.db.Exec("DELETE FROM " + table)
		n, _ = res.RowsAffected()
		total += n
	}

	return total, nil
}

func (s *Store) ClearActiveJobs() error {
	_, err := s.db.Exec("DELETE FROM active_jobs")
	return err
}

func (s *Store) ClearAbortQueue() error {
	_, err := s.db.Exec("DELETE FROM abort_queue")
	return err
}

func (s *Store) ClearRunQueue() error {
	_, err := s.db.Exec("DELETE FROM run_queue")
	return err
}

func (s *Store) EnqueueChainData(targetMinion, itemData, parentMinion string) error {
	_, err := s.db.Exec(
		"INSERT INTO chain_inbox (target_minion, item_data, parent_minion, created_at) VALUES (?, ?, ?, ?)",
		targetMinion, itemData, parentMinion, time.Now(),
	)
	return err
}

func (s *Store) DequeueChainData(targetMinion string) (itemData, parentMinion string, ok bool, err error) {
	tx, err := s.db.Begin()
	if err != nil {
		return "", "", false, err
	}
	defer tx.Rollback()

	var id int64
	err = tx.QueryRow(
		"SELECT id, item_data, parent_minion FROM chain_inbox WHERE target_minion = ? ORDER BY id ASC LIMIT 1",
		targetMinion,
	).Scan(&id, &itemData, &parentMinion)
	if err == sql.ErrNoRows {
		return "", "", false, nil
	}
	if err != nil {
		return "", "", false, err
	}

	_, err = tx.Exec("DELETE FROM chain_inbox WHERE id = ?", id)
	if err != nil {
		return "", "", false, err
	}

	if err := tx.Commit(); err != nil {
		return "", "", false, err
	}
	return itemData, parentMinion, true, nil
}

func (s *Store) GetChainDataMinions() ([]string, error) {
	rows, err := s.db.Query("SELECT DISTINCT target_minion FROM chain_inbox ORDER BY MIN(id) ASC")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []string
	for rows.Next() {
		var fn string
		if err := rows.Scan(&fn); err == nil {
			result = append(result, fn)
		}
	}
	return result, nil
}

func (s *Store) GetChainDataCount() (int, error) {
	var count int
	err := s.db.QueryRow("SELECT COUNT(*) FROM chain_inbox").Scan(&count)
	if err != nil {
		return 0, err
	}
	return count, nil
}

func (s *Store) HasChainData(targetMinion string) (bool, error) {
	var count int
	err := s.db.QueryRow("SELECT COUNT(*) FROM chain_inbox WHERE target_minion = ?", targetMinion).Scan(&count)
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

func (s *Store) ClearChainData() error {
	_, err := s.db.Exec("DELETE FROM chain_inbox")
	return err
}

func (s *Store) UpdateCharacterState(filename string, results, errors int) error {
	randStyle := string(character.RandomHairStyle())
	_, err := s.db.Exec(`
		INSERT INTO character_state (minion_filename, total_runs, total_matches, last_results, last_errors, character_type)
		VALUES (?, 1, ?, ?, ?, ?)
		ON CONFLICT(minion_filename) DO UPDATE SET
			total_runs = total_runs + 1,
			total_matches = total_matches + excluded.total_matches,
			last_results = excluded.last_results,
			last_errors = excluded.last_errors,
			character_type = CASE WHEN character_state.character_type = '' THEN excluded.character_type ELSE character_state.character_type END`,
		filename, results, results, errors, randStyle)
	return err
}

func (s *Store) InitCharacterState(filename string) error {
	style := string(character.RandomHairStyle())
	_, err := s.db.Exec(`
		INSERT OR IGNORE INTO character_state
		(minion_filename, total_runs, total_matches, last_results, last_errors, character_type)
		VALUES (?, 0, 0, 0, 0, ?)`,
		filename, style)
	return err
}

func (s *Store) GetCharacterState(filename string) (character.Data, error) {
	var pd character.Data
	var hs string
	err := s.db.QueryRow("SELECT total_runs, total_matches, last_results, last_errors, character_type FROM character_state WHERE minion_filename = ?", filename).Scan(&pd.TotalRuns, &pd.TotalMatches, &pd.LastResults, &pd.LastErrors, &hs)
	if err != nil {
		if err == sql.ErrNoRows {
			return character.Data{}, nil
		}
		return character.Data{}, err
	}
	pd.HairStyle = character.HairStyle(hs)
	return pd, nil
}
