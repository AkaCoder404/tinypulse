package db

import (
	"context"
	"fmt"
	"strings"
	"time"

	"tinypulse/internal/model"
)

func (d *DB) RecordCheck(ctx context.Context, c *model.Check) error {
	res, err := d.conn.ExecContext(ctx,
		`INSERT INTO checks (endpoint_id, status_code, response_time_ms, is_up, checked_at) VALUES (?, ?, ?, ?, ?)`,
		c.EndpointID, c.StatusCode, c.ResponseTimeMs, c.IsUp, c.CheckedAt)
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

func (d *DB) CalculateUptime(ctx context.Context, endpointID int64, hours int) (*float64, error) {
	query := fmt.Sprintf(`SELECT ROUND(AVG(is_up) * 100, 2) FROM checks WHERE endpoint_id = ? AND checked_at >= datetime('now', '-%d hours')`, hours)
	var uptime *float64
	err := d.conn.QueryRowContext(ctx, query, endpointID).Scan(&uptime)
	if err != nil {
		return nil, err
	}
	return uptime, nil
}

func (d *DB) RecordChecksBatch(ctx context.Context, checks []*model.Check) error {
	tx, err := d.conn.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	batchSize := 100
	for i := 0; i < len(checks); i += batchSize {
		end := i + batchSize
		if end > len(checks) {
			end = len(checks)
		}
		chunk := checks[i:end]

		placeholders := make([]string, len(chunk))
		values := make([]interface{}, len(chunk)*5)
		for j, c := range chunk {
			placeholders[j] = "(?, ?, ?, ?, ?)"
			values[j*5] = c.EndpointID
			values[j*5+1] = c.StatusCode
			values[j*5+2] = c.ResponseTimeMs
			values[j*5+3] = c.IsUp
			values[j*5+4] = c.CheckedAt
		}

		query := fmt.Sprintf("INSERT INTO checks (endpoint_id, status_code, response_time_ms, is_up, checked_at) VALUES %s", strings.Join(placeholders, ","))
		if _, err := tx.ExecContext(ctx, query, values...); err != nil {
			return err
		}
	}
	return tx.Commit()
}
