package db

import (
	"context"
	"encoding/json"
	"fmt"

	"tinypulse/internal/config"
)

// PrintDryRunDiff prints a comparison between the DB state and the provided config.
func (d *DB) PrintDryRunDiff(ctx context.Context, cfg *config.Config) error {
	fmt.Println("[INFO] Performing dry run")
	fmt.Println()
	
	if err := d.diffNotifiers(ctx, cfg); err != nil {
		return err
	}

	if err := d.diffEndpoints(ctx, cfg); err != nil {
		return err
	}

	fmt.Println()
	fmt.Println("[INFO] Dry run complete. No changes were applied to the database.")
	return nil
}

func (d *DB) diffNotifiers(ctx context.Context, cfg *config.Config) error {
	fmt.Println("Notifiers:")
	
	// Get existing config notifiers from DB
	rows, err := d.conn.QueryContext(ctx, `SELECT uid, name, type, config_json FROM notifiers WHERE source = 'config'`)
	if err != nil {
		return err
	}
	defer rows.Close()

	existing := make(map[string]struct {
		name       string
		typ        string
		configJSON string
	})
	
	for rows.Next() {
		var uid, name, typ, configJSON string
		if err := rows.Scan(&uid, &name, &typ, &configJSON); err != nil {
			return err
		}
		existing[uid] = struct {
			name       string
			typ        string
			configJSON string
		}{name, typ, configJSON}
	}

	// Compare
	for uid, nc := range cfg.Notifiers {
		configJSON, _ := json.Marshal(nc.Config)
		newConfig := string(configJSON)

		if ex, found := existing[uid]; found {
			if ex.name != nc.Name || ex.typ != nc.Type || ex.configJSON != newConfig {
				fmt.Printf("  ~ [UPDATE] %s (updated)\n", uid)
			} else {
				fmt.Printf("  = [KEEP]   %s (unchanged)\n", uid)
			}
			delete(existing, uid)
		} else {
			fmt.Printf("  + [CREATE] %s (type: %s)\n", uid, nc.Type)
		}
	}

	// Any left in 'existing' are deleted
	for uid := range existing {
		fmt.Printf("  - [DELETE] %s\n", uid)
	}

	return nil
}

func (d *DB) diffEndpoints(ctx context.Context, cfg *config.Config) error {
	fmt.Println("\nEndpoints:")
	
	rows, err := d.conn.QueryContext(ctx, `SELECT uid, name, type, url, interval_seconds, fail_threshold FROM endpoints WHERE source = 'config'`)
	if err != nil {
		return err
	}
	defer rows.Close()

	existing := make(map[string]struct {
		name            string
		typ             string
		url             string
		intervalSeconds int
		failThreshold   int
	})
	
	for rows.Next() {
		var uid, name, typ, url string
		var interval, fail int
		if err := rows.Scan(&uid, &name, &typ, &url, &interval, &fail); err != nil {
			return err
		}
		existing[uid] = struct {
			name            string
			typ             string
			url             string
			intervalSeconds int
			failThreshold   int
		}{name, typ, url, interval, fail}
	}

	for uid, ec := range cfg.Endpoints {
		if ex, found := existing[uid]; found {
			if ex.name != ec.Name || ex.typ != ec.Type || ex.url != ec.URL || ex.intervalSeconds != ec.IntervalSeconds || ex.failThreshold != ec.FailThreshold {
				fmt.Printf("  ~ [UPDATE] %s (updated)\n", uid)
			} else {
				fmt.Printf("  = [KEEP]   %s (unchanged)\n", uid)
			}
			delete(existing, uid)
		} else {
			fmt.Printf("  + [CREATE] %s (url: %s)\n", uid, ec.URL)
		}
	}

	for uid := range existing {
		fmt.Printf("  - [DELETE] %s\n", uid)
	}

	// We could also do diffEndpointConnections here, but just logging it as links helps.
	// Since connections are just re-created on each sync, we don't strictly need to diff them perfectly.
	fmt.Println("\nEndpoint Notifier Connections:")
	for uid, ec := range cfg.Endpoints {
		for _, nUID := range ec.Notifiers {
			fmt.Printf("  * [LINK]   %s -> %s\n", uid, nUID)
		}
	}

	return nil
}

// PrintAllAsCreates is used when there's no DB file at all for a pure dry-run
func PrintAllAsCreates(cfg *config.Config) error {
	fmt.Println("[INFO] Database does not exist. Pure dry-run mode.")
	fmt.Println("\nNotifiers:")
	for uid, nc := range cfg.Notifiers {
		fmt.Printf("  + [CREATE] %s (type: %s)\n", uid, nc.Type)
	}
	fmt.Println("\nEndpoints:")
	for uid, ec := range cfg.Endpoints {
		fmt.Printf("  + [CREATE] %s (url: %s)\n", uid, ec.URL)
	}
	fmt.Println("\nEndpoint Notifier Connections:")
	for uid, ec := range cfg.Endpoints {
		for _, nUID := range ec.Notifiers {
			fmt.Printf("  * [LINK]   %s -> %s\n", uid, nUID)
		}
	}
	fmt.Println("\n[INFO] Dry run complete. No changes were applied.")
	return nil
}
