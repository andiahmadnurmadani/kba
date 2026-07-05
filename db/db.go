package db

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"time"

	_ "modernc.org/sqlite"
)

var conn *sql.DB

type BackupRecord struct {
	ID       int
	Path     string
	Date     string
	Size     int64
	Services string
	Hostname string
	Status   string
}

// Init opens/creates the database
func Init() error {
	home, _ := os.UserHomeDir()
	dbDir := filepath.Join(home, ".kroombox")
	os.MkdirAll(dbDir, 0755)
	dbPath := filepath.Join(dbDir, "backup-agent.db")

	var err error
	conn, err = sql.Open("sqlite", dbPath)
	if err != nil {
		return fmt.Errorf("open db: %w", err)
	}

	// Create tables
	schema := `
	CREATE TABLE IF NOT EXISTS config (
		key TEXT PRIMARY KEY,
		value TEXT NOT NULL
	);
	CREATE TABLE IF NOT EXISTS backups (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		path TEXT NOT NULL,
		date TEXT NOT NULL,
		size INTEGER DEFAULT 0,
		services TEXT DEFAULT '',
		hostname TEXT DEFAULT '',
		status TEXT DEFAULT 'ok',
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);`
	if _, err := conn.Exec(schema); err != nil {
		return fmt.Errorf("create tables: %w", err)
	}
	return nil
}

// Close closes the database
func Close() {
	if conn != nil { conn.Close() }
}

// SaveBackup records a backup run
func SaveBackup(path, date string, size int64, services, hostname, status string) error {
	if conn == nil { return fmt.Errorf("db not initialized") }
	_, err := conn.Exec(
		"INSERT INTO backups (path, date, size, services, hostname, status) VALUES (?, ?, ?, ?, ?, ?)",
		path, date, size, services, hostname, status,
	)
	return err
}

// GetOldBackups returns backups older than keepDays
func GetOldBackups(keepDays int) ([]BackupRecord, error) {
	if conn == nil { return nil, fmt.Errorf("db not initialized") }
	cutoff := time.Now().AddDate(0, 0, -keepDays).Format("2006-01-02")
	rows, err := conn.Query(
		"SELECT id, path, date, size, services, hostname, status FROM backups WHERE date < ? ORDER BY date",
		cutoff,
	)
	if err != nil { return nil, err }
	defer rows.Close()

	var records []BackupRecord
	for rows.Next() {
		var r BackupRecord
		if err := rows.Scan(&r.ID, &r.Path, &r.Date, &r.Size, &r.Services, &r.Hostname, &r.Status); err != nil {
			continue
		}
		records = append(records, r)
	}
	return records, nil
}

// GetAllBackups returns all backups ordered by date desc
func GetAllBackups() ([]BackupRecord, error) {
	if conn == nil { return nil, fmt.Errorf("db not initialized") }
	rows, err := conn.Query(
		"SELECT id, path, date, size, services, hostname, status FROM backups ORDER BY date DESC LIMIT 30",
	)
	if err != nil { return nil, err }
	defer rows.Close()

	var records []BackupRecord
	for rows.Next() {
		var r BackupRecord
		if err := rows.Scan(&r.ID, &r.Path, &r.Date, &r.Size, &r.Services, &r.Hostname, &r.Status); err != nil {
			continue
		}
		records = append(records, r)
	}
	return records, nil
}

// DeleteBackup deletes a backup record by ID
func DeleteBackup(id int) error {
	if conn == nil { return fmt.Errorf("db not initialized") }
	_, err := conn.Exec("DELETE FROM backups WHERE id = ?", id)
	return err
}

// SetConfig stores a config value
func SetConfig(key, value string) error {
	if conn == nil { return fmt.Errorf("db not initialized") }
	_, err := conn.Exec(
		"INSERT INTO config (key, value) VALUES (?, ?) ON CONFLICT(key) DO UPDATE SET value = ?",
		key, value, value,
	)
	return err
}

// GetConfig retrieves a config value
func GetConfig(key string) (string, error) {
	if conn == nil { return "", fmt.Errorf("db not initialized") }
	var value string
	err := conn.QueryRow("SELECT value FROM config WHERE key = ?", key).Scan(&value)
	if err == sql.ErrNoRows { return "", nil }
	return value, err
}

// GetLastBackup returns the most recent backup
func GetLastBackup() (*BackupRecord, error) {
	if conn == nil { return nil, fmt.Errorf("db not initialized") }
	var r BackupRecord
	err := conn.QueryRow(
		"SELECT id, path, date, size, services, hostname, status FROM backups ORDER BY date DESC LIMIT 1",
	).Scan(&r.ID, &r.Path, &r.Date, &r.Size, &r.Services, &r.Hostname, &r.Status)
	if err == sql.ErrNoRows { return nil, nil }
	if err != nil { return nil, err }
	return &r, nil
}

// GetTotalSize returns total size of all backups
func GetTotalSize() (int64, error) {
	if conn == nil { return 0, fmt.Errorf("db not initialized") }
	var total int64
	err := conn.QueryRow("SELECT COALESCE(SUM(size), 0) FROM backups").Scan(&total)
	return total, err
}
