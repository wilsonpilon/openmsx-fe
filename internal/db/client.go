package db

import (
	"database/sql"
	"fmt"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

type DB struct {
	conn *sql.DB
}

type HistoryEntry struct {
	ID        int64
	Command   string
	Response  string
	CreatedAt time.Time
}

type Favorite struct {
	ID        int64
	Name      string
	FileName  string
	FileType  string
	URL       string
	MsxGen    string
	CreatedAt time.Time
}

type RecentFile struct {
	ID       int64
	FileName string
	FilePath string
	FileType string
	LoadedAt time.Time
}

func New(path string) (*DB, error) {
	conn, err := sql.Open("sqlite3", path+"?_journal_mode=WAL&_foreign_keys=on")
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}
	if err := conn.Ping(); err != nil {
		return nil, fmt.Errorf("ping sqlite: %w", err)
	}
	return &DB{conn: conn}, nil
}

func (d *DB) Close() error {
	return d.conn.Close()
}

func (d *DB) Migrate() error {
	schema := `
	CREATE TABLE IF NOT EXISTS command_history (
		id         INTEGER PRIMARY KEY AUTOINCREMENT,
		command    TEXT NOT NULL,
		response   TEXT,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);

	CREATE TABLE IF NOT EXISTS favorites (
		id         INTEGER PRIMARY KEY AUTOINCREMENT,
		name       TEXT NOT NULL,
		file_name  TEXT NOT NULL,
		file_type  TEXT NOT NULL,
		url        TEXT,
		msx_gen    TEXT,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);

	CREATE TABLE IF NOT EXISTS recent_files (
		id         INTEGER PRIMARY KEY AUTOINCREMENT,
		file_name  TEXT NOT NULL,
		file_path  TEXT NOT NULL,
		file_type  TEXT NOT NULL,
		loaded_at  DATETIME DEFAULT CURRENT_TIMESTAMP,
		UNIQUE(file_path)
	);

	CREATE INDEX IF NOT EXISTS idx_cmd_history_created ON command_history(created_at DESC);
	CREATE INDEX IF NOT EXISTS idx_favorites_type ON favorites(file_type);
	`
	_, err := d.conn.Exec(schema)
	return err
}

// ── Command History ──────────────────────────────────────────────────────────

func (d *DB) AddCommandHistory(cmd, response string) error {
	_, err := d.conn.Exec(
		`INSERT INTO command_history (command, response) VALUES (?, ?)`,
		cmd, response,
	)
	return err
}

func (d *DB) GetCommandHistory(limit int) ([]HistoryEntry, error) {
	rows, err := d.conn.Query(
		`SELECT id, command, response, created_at FROM command_history ORDER BY created_at DESC LIMIT ?`,
		limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var entries []HistoryEntry
	for rows.Next() {
		var e HistoryEntry
		if err := rows.Scan(&e.ID, &e.Command, &e.Response, &e.CreatedAt); err != nil {
			return nil, err
		}
		entries = append(entries, e)
	}
	return entries, rows.Err()
}

func (d *DB) ClearCommandHistory() error {
	_, err := d.conn.Exec(`DELETE FROM command_history`)
	return err
}

// ── Favorites ────────────────────────────────────────────────────────────────

func (d *DB) AddFavorite(name, fileName, fileType, url, msxGen string) error {
	_, err := d.conn.Exec(
		`INSERT INTO favorites (name, file_name, file_type, url, msx_gen) VALUES (?, ?, ?, ?, ?)`,
		name, fileName, fileType, url, msxGen,
	)
	return err
}

func (d *DB) GetFavorites() ([]Favorite, error) {
	rows, err := d.conn.Query(
		`SELECT id, name, file_name, file_type, url, msx_gen, created_at FROM favorites ORDER BY created_at DESC`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var favs []Favorite
	for rows.Next() {
		var f Favorite
		if err := rows.Scan(&f.ID, &f.Name, &f.FileName, &f.FileType, &f.URL, &f.MsxGen, &f.CreatedAt); err != nil {
			return nil, err
		}
		favs = append(favs, f)
	}
	return favs, rows.Err()
}

func (d *DB) DeleteFavorite(id int64) error {
	_, err := d.conn.Exec(`DELETE FROM favorites WHERE id = ?`, id)
	return err
}

// ── Recent Files ─────────────────────────────────────────────────────────────

func (d *DB) UpsertRecentFile(fileName, filePath, fileType string) error {
	_, err := d.conn.Exec(`
		INSERT INTO recent_files (file_name, file_path, file_type, loaded_at)
		VALUES (?, ?, ?, CURRENT_TIMESTAMP)
		ON CONFLICT(file_path) DO UPDATE SET loaded_at = CURRENT_TIMESTAMP
	`, fileName, filePath, fileType)
	return err
}

func (d *DB) GetRecentFiles(limit int) ([]RecentFile, error) {
	rows, err := d.conn.Query(
		`SELECT id, file_name, file_path, file_type, loaded_at FROM recent_files ORDER BY loaded_at DESC LIMIT ?`,
		limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var files []RecentFile
	for rows.Next() {
		var f RecentFile
		if err := rows.Scan(&f.ID, &f.FileName, &f.FilePath, &f.FileType, &f.LoadedAt); err != nil {
			return nil, err
		}
		files = append(files, f)
	}
	return files, rows.Err()
}
