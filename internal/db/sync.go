package db

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"tinypulse/internal/config"
)

// SyncConfig compares the provided YAML configuration with the current database state
// and applies necessary creations, updates, and deletions within a single transaction.
func (d *DB) SyncConfig(ctx context.Context, cfg *config.Config) error {
	tx, err := d.conn.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// 1. Process Notifiers
	notifierUIDs := make([]interface{}, 0, len(cfg.Notifiers))
	for uid, nc := range cfg.Notifiers {
		notifierUIDs = append(notifierUIDs, uid)
		
		configJSON, err := json.Marshal(nc.Config)
		if err != nil {
			return fmt.Errorf("failed to marshal notifier config for %q: %w", uid, err)
		}

		_, err = tx.ExecContext(ctx, `
			INSERT INTO notifiers (uid, name, type, config_json, source)
			VALUES (?, ?, ?, ?, 'config')
			ON CONFLICT(uid) DO UPDATE SET
				name = excluded.name,
				type = excluded.type,
				config_json = excluded.config_json,
				source = 'config'`,
			uid, nc.Name, nc.Type, string(configJSON))
		
		if err != nil {
			return fmt.Errorf("failed to upsert notifier %q: %w", uid, err)
		}
	}

	// 2. Process Endpoints
	endpointUIDs := make([]interface{}, 0, len(cfg.Endpoints))
	for uid, ec := range cfg.Endpoints {
		endpointUIDs = append(endpointUIDs, uid)
		
		_, err := tx.ExecContext(ctx, `
			INSERT INTO endpoints (uid, type, name, url, interval_seconds, fail_threshold, paused, source)
			VALUES (?, ?, ?, ?, ?, ?, 0, 'config')
			ON CONFLICT(uid) DO UPDATE SET
				type = excluded.type,
				name = excluded.name,
				url = excluded.url,
				interval_seconds = excluded.interval_seconds,
				fail_threshold = excluded.fail_threshold,
				source = 'config'`,
			uid, ec.Type, ec.Name, ec.URL, ec.IntervalSeconds, ec.FailThreshold)
			
		if err != nil {
			return fmt.Errorf("failed to upsert endpoint %q: %w", uid, err)
		}
	}

	// 3. Sync Endpoint-Notifier Connections
	// We only want to manage connections for endpoints that come from the config.
	for uid, ec := range cfg.Endpoints {
		// Get internal DB IDs for this config endpoint and its notifiers
		var endpointID int64
		if err := tx.QueryRowContext(ctx, `SELECT id FROM endpoints WHERE uid = ? AND source = 'config'`, uid).Scan(&endpointID); err != nil {
			return fmt.Errorf("failed to retrieve endpoint ID for %q: %w", uid, err)
		}

		// Clear old connections
		if _, err := tx.ExecContext(ctx, `DELETE FROM endpoint_notifiers WHERE endpoint_id = ?`, endpointID); err != nil {
			return fmt.Errorf("failed to clear old connections for endpoint %q: %w", uid, err)
		}

		// Insert new connections
		for _, nUID := range ec.Notifiers {
			var notifierID int64
			if err := tx.QueryRowContext(ctx, `SELECT id FROM notifiers WHERE uid = ? AND source = 'config'`, nUID).Scan(&notifierID); err != nil {
				return fmt.Errorf("failed to retrieve notifier ID for %q (referenced by %q): %w", nUID, uid, err)
			}

			if _, err := tx.ExecContext(ctx, `INSERT INTO endpoint_notifiers (endpoint_id, notifier_id) VALUES (?, ?)`, endpointID, notifierID); err != nil {
				return fmt.Errorf("failed to insert connection %q -> %q: %w", uid, nUID, err)
			}
		}
	}

	// 4. Prune Orphans
	
	// Prune Config Endpoints not in YAML
	if len(endpointUIDs) > 0 {
		placeholders := make([]string, len(endpointUIDs))
		for i := range placeholders {
			placeholders[i] = "?"
		}
		
		query := fmt.Sprintf(`DELETE FROM endpoints WHERE source = 'config' AND uid NOT IN (%s)`, joinStrings(placeholders, ","))
		res, err := tx.ExecContext(ctx, query, endpointUIDs...)
		if err != nil {
			return fmt.Errorf("failed to prune orphaned endpoints: %w", err)
		}
		if rows, _ := res.RowsAffected(); rows > 0 {
			slog.Info("pruned orphaned config endpoints", "count", rows)
		}
	} else {
		// If the YAML contains no endpoints, delete all config endpoints
		if _, err := tx.ExecContext(ctx, `DELETE FROM endpoints WHERE source = 'config'`); err != nil {
			return fmt.Errorf("failed to prune all config endpoints: %w", err)
		}
	}

	// Prune Config Notifiers not in YAML
	if len(notifierUIDs) > 0 {
		placeholders := make([]string, len(notifierUIDs))
		for i := range placeholders {
			placeholders[i] = "?"
		}
		query := fmt.Sprintf(`DELETE FROM notifiers WHERE source = 'config' AND uid NOT IN (%s)`, joinStrings(placeholders, ","))
		res, err := tx.ExecContext(ctx, query, notifierUIDs...)
		if err != nil {
			return fmt.Errorf("failed to prune orphaned notifiers: %w", err)
		}
		if rows, _ := res.RowsAffected(); rows > 0 {
			slog.Info("pruned orphaned config notifiers", "count", rows)
		}
	} else {
		// If the YAML contains no notifiers, delete all config notifiers
		if _, err := tx.ExecContext(ctx, `DELETE FROM notifiers WHERE source = 'config'`); err != nil {
			return fmt.Errorf("failed to prune all config notifiers: %w", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}

func joinStrings(s []string, sep string) string {
	if len(s) == 0 {
		return ""
	}
	res := s[0]
	for i := 1; i < len(s); i++ {
		res += sep + s[i]
	}
	return res
}
