package db

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	"tinypulse/internal/config"
)

// ExportConfig reads all endpoints and notifiers from the database and returns a populated Config struct.
// For items created via the UI (source = 'ui'), it generates a slug-safe UID to use as the YAML key.
func (d *DB) ExportConfig(ctx context.Context) (*config.Config, error) {
	cfg := &config.Config{
		Notifiers: make(map[string]config.NotifierConfig),
		Endpoints: make(map[string]config.EndpointConfig),
	}

	// 1. Export Notifiers
	notifierUIDs := make(map[int64]string) // Map DB ID to generated/existing UID

	nRows, err := d.conn.QueryContext(ctx, `SELECT id, uid, name, type, config_json FROM notifiers ORDER BY id ASC`)
	if err != nil {
		return nil, fmt.Errorf("failed to query notifiers: %w", err)
	}
	defer nRows.Close()

	for nRows.Next() {
		var id int64
		var uid *string
		var name, typ, configJSON string
		if err := nRows.Scan(&id, &uid, &name, &typ, &configJSON); err != nil {
			return nil, fmt.Errorf("failed to scan notifier: %w", err)
		}

		finalUID := generateUID(uid, name, id)
		notifierUIDs[id] = finalUID

		var parsedConfig map[string]interface{}
		if err := json.Unmarshal([]byte(configJSON), &parsedConfig); err != nil {
			return nil, fmt.Errorf("failed to unmarshal notifier config json for %q: %w", name, err)
		}

		cfg.Notifiers[finalUID] = config.NotifierConfig{
			Name:   name,
			Type:   typ,
			Config: parsedConfig,
		}
	}
	if err := nRows.Err(); err != nil {
		return nil, err
	}

	// 2. Export Endpoints
	eRows, err := d.conn.QueryContext(ctx, `SELECT id, uid, name, type, url, interval_seconds, fail_threshold FROM endpoints ORDER BY id ASC`)
	if err != nil {
		return nil, fmt.Errorf("failed to query endpoints: %w", err)
	}
	defer eRows.Close()

	for eRows.Next() {
		var id int64
		var uid *string
		var name, typ, url string
		var interval, failThreshold int
		if err := eRows.Scan(&id, &uid, &name, &typ, &url, &interval, &failThreshold); err != nil {
			return nil, fmt.Errorf("failed to scan endpoint: %w", err)
		}

		finalUID := generateUID(uid, name, id)

		// Fetch connected notifiers
		var connectedNotifiers []string
		enRows, err := d.conn.QueryContext(ctx, `SELECT notifier_id FROM endpoint_notifiers WHERE endpoint_id = ? ORDER BY notifier_id ASC`, id)
		if err != nil {
			return nil, fmt.Errorf("failed to query endpoint notifiers for %q: %w", name, err)
		}
		for enRows.Next() {
			var nid int64
			if err := enRows.Scan(&nid); err != nil {
				enRows.Close()
				return nil, fmt.Errorf("failed to scan endpoint notifier: %w", err)
			}
			if nUID, ok := notifierUIDs[nid]; ok {
				connectedNotifiers = append(connectedNotifiers, nUID)
			}
		}
		enRows.Close()
		if err := enRows.Err(); err != nil {
			return nil, err
		}

		cfg.Endpoints[finalUID] = config.EndpointConfig{
			Name:            name,
			Type:            typ,
			URL:             url,
			IntervalSeconds: interval,
			FailThreshold:   failThreshold,
			Notifiers:       connectedNotifiers,
		}
	}
	if err := eRows.Err(); err != nil {
		return nil, err
	}

	return cfg, nil
}

// generateUID generates a slug-safe UID for YAML keys.
// If an existing UID is provided (from the database), it returns it.
// Otherwise, it creates one from the name, falling back to an ID string if empty.
func generateUID(existingUID *string, name string, id int64) string {
	if existingUID != nil && *existingUID != "" {
		return *existingUID
	}

	if name == "" {
		return fmt.Sprintf("item_%d", id)
	}

	// Convert to lowercase
	slug := strings.ToLower(name)
	// Replace non-alphanumeric characters with underscores
	reg := regexp.MustCompile(`[^a-z0-9]+`)
	slug = reg.ReplaceAllString(slug, "_")
	// Trim trailing underscores
	slug = strings.Trim(slug, "_")

	if slug == "" {
		return fmt.Sprintf("item_%d", id)
	}

	// For safety, append ID to guarantee uniqueness
	return fmt.Sprintf("%s_%d", slug, id)
}
