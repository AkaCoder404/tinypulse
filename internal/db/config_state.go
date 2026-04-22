package db

import (
	"context"
)

// CountConfigItems returns the total number of endpoints and notifiers
// in the database that are currently marked as managed by configuration.
func (d *DB) CountConfigItems(ctx context.Context) (int, error) {
	var endpointCount, notifierCount int

	if err := d.conn.QueryRowContext(ctx, `SELECT COUNT(*) FROM endpoints WHERE source = 'config'`).Scan(&endpointCount); err != nil {
		return 0, err
	}

	if err := d.conn.QueryRowContext(ctx, `SELECT COUNT(*) FROM notifiers WHERE source = 'config'`).Scan(&notifierCount); err != nil {
		return 0, err
	}

	return endpointCount + notifierCount, nil
}

// EjectConfigItems demotes all config-managed items to UI-managed items.
// This allows users to safely resume manual control over items that were previously
// provisioned via a config file, without losing their historical check data.
func (d *DB) EjectConfigItems(ctx context.Context) error {
	tx, err := d.conn.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(ctx, `UPDATE endpoints SET source = 'ui', uid = NULL WHERE source = 'config'`); err != nil {
		return err
	}

	if _, err := tx.ExecContext(ctx, `UPDATE notifiers SET source = 'ui', uid = NULL WHERE source = 'config'`); err != nil {
		return err
	}

	return tx.Commit()
}
