package main

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"time"

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
	_, err = db.Exec(`CREATE TABLE IF NOT EXISTS word_counts (
		date TEXT PRIMARY KEY,
		count INTEGER NOT NULL
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

// recordWordDelta accumulates a net change in words (typed or deleted) into the
// daily word count. Deltas can be negative (deletions); the running total is
// clamped at 0. This keeps the count persistent across app restarts.
func recordWordDelta(db *sql.DB, delta int) error {
	if delta == 0 {
		return nil
	}
	date := time.Now().Format("2006-01-02")
	_, err := db.Exec(`INSERT INTO word_counts (date, count) VALUES (?, MAX(0, ?))
		ON CONFLICT(date) DO UPDATE SET count = MAX(0, count + ?)`, date, delta, delta)
	return err
}

// getTodayWordCount returns the persistent word count accumulated for today.
func getTodayWordCount(db *sql.DB) (int, error) {
	date := time.Now().Format("2006-01-02")
	var count int
	err := db.QueryRow(`SELECT count FROM word_counts WHERE date = ?`, date).Scan(&count)
	if err == sql.ErrNoRows {
		return 0, nil
	}
	if err != nil {
		return 0, err
	}
	return count, nil
}

func getWordCounts(db *sql.DB, year int, month int) ([]struct {
	Date  string
	Count int
}, error) {
	query := `SELECT date, count FROM word_counts WHERE strftime('%Y', date) = ? AND strftime('%m', date) = ?`
	rows, err := db.Query(query, fmt.Sprintf("%d", year), fmt.Sprintf("%02d", month))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []struct {
		Date  string
		Count int
	}
	for rows.Next() {
		var date string
		var count int
		if err := rows.Scan(&date, &count); err != nil {
			return nil, err
		}
		results = append(results, struct {
			Date  string
			Count int
		}{date, count})
	}
	return results, nil
}

func getStreak(db *sql.DB, goal int) (int, error) {
	rows, err := db.Query(`SELECT DISTINCT date FROM word_counts WHERE count >= ?`, goal)
	if err != nil {
		return 0, err
	}
	defer rows.Close()

	met := make(map[string]bool)
	for rows.Next() {
		var date string
		if err := rows.Scan(&date); err != nil {
			return 0, err
		}
		met[date] = true
	}

	// Count consecutive days ending today, or yesterday if today not met yet.
	today := time.Now()
	cur := today
	if !met[cur.Format("2006-01-02")] {
		cur = cur.AddDate(0, 0, -1)
	}
	streak := 0
	for met[cur.Format("2006-01-02")] {
		streak++
		cur = cur.AddDate(0, 0, -1)
	}
	return streak, nil
}
