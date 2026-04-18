package db

import (
	"context"
	"fmt"
	"time"

	"tinypulse/internal/model"
)

func (d *DB) RecordCheck(ctx context.Context, c *model.Check) error {
	res, err := d.conn.ExecContext(ctx,
		`INSERT INTO checks (endpoint_id, status_code, response_time_ms, is_up) VALUES (?, ?, ?, ?)`,
		c.EndpointID, c.StatusCode, c.ResponseTimeMs, c.IsUp)
	if err != nil {
		return err
	}
	c.ID, _ = res.LastInsertId()
	return nil
}

func (d *DB) CleanupOldChecks(ctx context.Context, days int) error {
	modifier := fmt.Sprintf("-%d days", days)

	for {
		// Delete up to 1000 rows at a time
		res, err := d.conn.ExecContext(ctx,
			`DELETE FROM checks WHERE id IN (
				SELECT id FROM checks WHERE checked_at < datetime('now', ?) LIMIT 1000
			)`, modifier)
		if err != nil {
			return err
		}

		affected, err := res.RowsAffected()
		if err != nil {
			return err
		}

		if affected == 0 {
			// No more rows to delete, we are done
			break
		}

		// Yield to other writers (like our new channel writer)
		// This ensures we don't lock the DB for seconds at a time
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(100 * time.Millisecond):
		}
	}

	return nil
}

func (d *DB) GetChecksByEndpoint(ctx context.Context, endpointID int64, limit int) ([]model.Check, error) {
	rows, err := d.conn.QueryContext(ctx,
		`SELECT id, endpoint_id, status_code, response_time_ms, is_up, checked_at 
         FROM checks WHERE endpoint_id = ? ORDER BY checked_at DESC, id DESC LIMIT ?`,
		endpointID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var checks []model.Check
	for rows.Next() {
		var c model.Check
		if err := rows.Scan(&c.ID, &c.EndpointID, &c.StatusCode, &c.ResponseTimeMs, &c.IsUp, &c.CheckedAt); err != nil {
			return nil, err
		}
		checks = append(checks, c)
	}
	return checks, rows.Err()
}
