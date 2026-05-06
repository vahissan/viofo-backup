package tracker

import (
	"database/sql"
	"fmt"
	"time"

	_ "modernc.org/sqlite"
)

const schema = `
CREATE TABLE IF NOT EXISTS downloads (
    name          TEXT PRIMARY KEY,
    fpath         TEXT NOT NULL,
    category      TEXT NOT NULL,
    local_path    TEXT NOT NULL,
    size          INTEGER NOT NULL,
    file_time     DATETIME NOT NULL,
    downloaded_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX IF NOT EXISTS idx_file_time ON downloads (file_time);
`

// Tracker persists a record of every successfully downloaded file.
type Tracker struct {
	db *sql.DB
}

func Open(path string) (*Tracker, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("open db: %w", err)
	}
	if _, err := db.Exec(schema); err != nil {
		db.Close()
		return nil, fmt.Errorf("init schema: %w", err)
	}
	return &Tracker{db: db}, nil
}

func (t *Tracker) Close() error { return t.db.Close() }

// Entry is a row in the downloads table.
type Entry struct {
	Name         string
	FPATH        string
	Category     string
	LocalPath    string
	Size         int64
	FileTime     time.Time
	DownloadedAt time.Time
}

// Has returns true if a file with the given name is already tracked.
func (t *Tracker) Has(name string) (bool, error) {
	var n int
	err := t.db.QueryRow("SELECT COUNT(*) FROM downloads WHERE name = ?", name).Scan(&n)
	return n > 0, err
}

// Record inserts or replaces a download entry.
func (t *Tracker) Record(e Entry) error {
	_, err := t.db.Exec(
		`INSERT OR REPLACE INTO downloads
		 (name, fpath, category, local_path, size, file_time, downloaded_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?)`,
		e.Name, e.FPATH, e.Category, e.LocalPath, e.Size,
		e.FileTime.UTC(), time.Now().UTC(),
	)
	return err
}

// AddIfMissing inserts an entry only if it does not already exist.
// Used during startup reconciliation with local files.
func (t *Tracker) AddIfMissing(e Entry) error {
	_, err := t.db.Exec(
		`INSERT OR IGNORE INTO downloads
		 (name, fpath, category, local_path, size, file_time, downloaded_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?)`,
		e.Name, e.FPATH, e.Category, e.LocalPath, e.Size,
		e.FileTime.UTC(), e.FileTime.UTC(),
	)
	return err
}

// OldestByAge returns entries whose file_time is before cutoff, ordered oldest first.
func (t *Tracker) OldestByAge(cutoff time.Time) ([]Entry, error) {
	rows, err := t.db.Query(
		`SELECT name, fpath, category, local_path, size, file_time, downloaded_at
		 FROM downloads WHERE file_time < ? ORDER BY file_time ASC`,
		cutoff.UTC(),
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanEntries(rows)
}

// TotalSize returns the sum of all tracked file sizes.
func (t *Tracker) TotalSize() (int64, error) {
	var total int64
	err := t.db.QueryRow("SELECT COALESCE(SUM(size), 0) FROM downloads").Scan(&total)
	return total, err
}

// OldestFirst returns up to limit entries ordered by file_time ascending.
func (t *Tracker) OldestFirst(limit int) ([]Entry, error) {
	rows, err := t.db.Query(
		`SELECT name, fpath, category, local_path, size, file_time, downloaded_at
		 FROM downloads ORDER BY file_time ASC LIMIT ?`,
		limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanEntries(rows)
}

// Delete removes a tracked entry by filename.
func (t *Tracker) Delete(name string) error {
	_, err := t.db.Exec("DELETE FROM downloads WHERE name = ?", name)
	return err
}

func scanEntries(rows *sql.Rows) ([]Entry, error) {
	var entries []Entry
	for rows.Next() {
		var e Entry
		if err := rows.Scan(
			&e.Name, &e.FPATH, &e.Category, &e.LocalPath,
			&e.Size, &e.FileTime, &e.DownloadedAt,
		); err != nil {
			return nil, err
		}
		entries = append(entries, e)
	}
	return entries, rows.Err()
}
