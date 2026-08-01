package main

import (
	"database/sql"
	"os"
	"path/filepath"

	_ "modernc.org/sqlite"
)

func dbPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(home, ".local", "share", "writr")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", err
	}
	return filepath.Join(dir, "writr.db"), nil
}

func openDB() (*sql.DB, error) {
	path, err := dbPath()
	if err != nil {
		return nil, err
	}
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	_, err = db.Exec(`CREATE TABLE IF NOT EXISTS settings (
		key TEXT PRIMARY KEY,
		value TEXT NOT NULL
	)`)
	if err != nil {
		return nil, err
	}
	_, err = db.Exec(`INSERT OR IGNORE INTO settings (key, value) VALUES ('word_goal', '500')`)
	if err != nil {
		return nil, err
	}
	return db, nil
}

func getGoal(db *sql.DB) (string, error) {
	var value string
	err := db.QueryRow(`SELECT value FROM settings WHERE key = 'word_goal'`).Scan(&value)
	if err != nil {
		return "500", nil
	}
	return value, nil
}

func setGoal(db *sql.DB, goal string) error {
	_, err := db.Exec(`INSERT OR REPLACE INTO settings (key, value) VALUES ('word_goal', ?)`, goal)
	return err
}