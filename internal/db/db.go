package db

import (
	"database/sql"
	"fmt"
	"log/slog"

	_ "modernc.org/sqlite"
)

type DB struct {
	conn *sql.DB
}

func New(path string) (*DB, error) {
	conn, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	if err := conn.Ping(); err != nil {
		return nil, err
	}

	db := &DB{conn: conn}
	if err := db.migrate(); err != nil {
		return nil, fmt.Errorf("migrate: %w", err)
	}
	return db, nil
}

func (d *DB) migrate() error {
	if _, err := d.conn.Exec(`PRAGMA journal_mode = WAL;`); err != nil {
		return fmt.Errorf("set wal mode: %w", err)
	}
	if _, err := d.conn.Exec(`PRAGMA foreign_keys = ON;`); err != nil {
		return fmt.Errorf("enable foreign keys: %w", err)
	}

	// 1. Ensure versioning table exists
	if _, err := d.conn.Exec(`CREATE TABLE IF NOT EXISTS schema_version (version INTEGER)`); err != nil {
		return err
	}

	// 2. Get current version (defaults to 0 if table is empty)
	var currentVersion int
	_ = d.conn.QueryRow(`SELECT MAX(version) FROM schema_version`).Scan(&currentVersion)

	targetVersion := 2

	// Special case: If currentVersion is 0, but the endpoints table already exists,
	// this is an existing v0.1.0-beta.1 database updating to the new versioning system.
	// We need to set it to version 1 so it triggers a backup before applying the v2 migration.
	if currentVersion == 0 {
		var hasEndpoints bool
		_ = d.conn.QueryRow(`SELECT 1 FROM sqlite_master WHERE type='table' AND name='endpoints'`).Scan(&hasEndpoints)
		if hasEndpoints {
			currentVersion = 1
			slog.Info("detected unversioned database, initializing to schema version 1")
		}
	}

	// 3. If a migration is needed, make a backup FIRST
	if currentVersion > 0 && currentVersion < targetVersion {
		backupPath := fmt.Sprintf("uptime_backup_v%d.db", currentVersion)
		slog.Info("database migration required, creating backup...", "backup_file", backupPath)

		// VACUUM INTO safely copies the entire DB (even in WAL mode) to a new file
		if _, err := d.conn.Exec(fmt.Sprintf(`VACUUM INTO '%s'`, backupPath)); err != nil {
			return fmt.Errorf("failed to create pre-migration backup: %w", err)
		}
	}

	// 4. Apply V1 (Base Schema - pre-TCP support)
	if currentVersion < 1 {
		schemaV1 := `
		CREATE TABLE IF NOT EXISTS endpoints (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			name TEXT NOT NULL,
			url TEXT NOT NULL,
			interval_seconds INTEGER NOT NULL DEFAULT 60,
			fail_threshold INTEGER NOT NULL DEFAULT 3,
			paused INTEGER NOT NULL DEFAULT 0,
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		);

		CREATE TABLE IF NOT EXISTS checks (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			endpoint_id INTEGER NOT NULL REFERENCES endpoints(id) ON DELETE CASCADE,
			status_code INTEGER,
			response_time_ms INTEGER NOT NULL,
			is_up INTEGER NOT NULL,
			checked_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		);

		CREATE INDEX IF NOT EXISTS idx_checks_endpoint_id ON checks(endpoint_id);
		CREATE INDEX IF NOT EXISTS idx_checks_checked_at ON checks(checked_at);

		CREATE TABLE IF NOT EXISTS notifiers (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			name TEXT NOT NULL,
			type TEXT NOT NULL,
			config_json TEXT NOT NULL,
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		);

		CREATE TABLE IF NOT EXISTS endpoint_notifiers (
			endpoint_id INTEGER NOT NULL REFERENCES endpoints(id) ON DELETE CASCADE,
			notifier_id INTEGER NOT NULL REFERENCES notifiers(id) ON DELETE CASCADE,
			PRIMARY KEY (endpoint_id, notifier_id)
		);
		`
		if _, err := d.conn.Exec(schemaV1); err != nil {
			return fmt.Errorf("v1 migration failed: %w", err)
		}
		currentVersion = 1
	}

	// 5. Apply V2 (Add 'type' column for HTTP/TCP support)
	if currentVersion < 2 {
		slog.Info("applying v2 schema migration (adding 'type' column)")
		if _, err := d.conn.Exec(`ALTER TABLE endpoints ADD COLUMN type TEXT NOT NULL DEFAULT 'http';`); err != nil {
			return fmt.Errorf("v2 migration failed: %w", err)
		}
		currentVersion = 2
	}

	// 6. Save the new version state
	if currentVersion == targetVersion {
		d.conn.Exec(`DELETE FROM schema_version`)
		d.conn.Exec(`INSERT INTO schema_version (version) VALUES (?)`, targetVersion)
	}

	return nil
}

func (d *DB) Close() error {
	return d.conn.Close()
}
