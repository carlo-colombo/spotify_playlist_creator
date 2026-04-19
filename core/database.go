package core

import (
	"database/sql"
	"os"
	"path/filepath"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

// Database wraps sql.DB for caching
type Database struct {
	db *sql.DB
}

// NewDatabase creates a new database connection
func NewDatabase() (*Database, error) {
	cacheDir, err := os.UserCacheDir()
	if err != nil {
		return nil, err
	}
	dbPath := filepath.Join(cacheDir, "spotify_playlist_creator", "spotify_playlist_creator.db")

	if err := os.MkdirAll(filepath.Dir(dbPath), 0755); err != nil {
		return nil, err
	}

	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, err
	}

	// Create tables if they don't exist
	_, err = db.Exec(`
    CREATE TABLE IF NOT EXISTS cache (
        key TEXT PRIMARY KEY,
        value TEXT,
        expiry INTEGER
    );
    `)
	if err != nil {
		return nil, err
	}

	return &Database{db: db}, nil
}

// GetCache retrieves a cached value
func (d *Database) GetCache(key string) (string, bool) {
	var value string
	var expiry int64

	row := d.db.QueryRow("SELECT value, expiry FROM cache WHERE key = ?", key)
	if err := row.Scan(&value, &expiry); err != nil {
		return "", false
	}

	if time.Now().Unix() > expiry {
		d.db.Exec("DELETE FROM cache WHERE key = ?", key)
		return "", false
	}

	return value, true
}

// SetCache stores a value with TTL
func (d *Database) SetCache(key, value string, ttlSeconds int64) error {
	expiry := time.Now().Unix() + ttlSeconds
	_, err := d.db.Exec("INSERT OR REPLACE INTO cache (key, value, expiry) VALUES (?, ?, ?)", key, value, expiry)
	return err
}

// Close closes the database connection
func (d *Database) Close() {
	d.db.Close()
}