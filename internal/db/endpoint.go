package db

import (
	"context"
	"database/sql"

	"tinypulse/internal/model"
)

func (d *DB) CreateEndpoint(ctx context.Context, ep *model.Endpoint) error {
	res, err := d.conn.ExecContext(ctx,
		`INSERT INTO endpoints (name, url, interval_seconds, fail_threshold, paused) VALUES (?, ?, ?, ?, ?)`,
		ep.Name, ep.URL, ep.IntervalSeconds, ep.FailThreshold, ep.Paused)
	if err != nil {
		return err
	}
	id, _ := res.LastInsertId()
	created, err := d.GetEndpoint(ctx, id)
	if err != nil {
		return err
	}
	*ep = *created
	return nil
}

func (d *DB) GetEndpoint(ctx context.Context, id int64) (*model.Endpoint, error) {
	row := d.conn.QueryRowContext(ctx,
		`SELECT id, name, url, interval_seconds, fail_threshold, paused, created_at FROM endpoints WHERE id = ?`, id)
	ep := &model.Endpoint{}
	err := row.Scan(&ep.ID, &ep.Name, &ep.URL, &ep.IntervalSeconds, &ep.FailThreshold, &ep.Paused, &ep.CreatedAt)
	if err != nil {
		return nil, err
	}
	return ep, nil
}

func (d *DB) UpdateEndpoint(ctx context.Context, ep *model.Endpoint) error {
	_, err := d.conn.ExecContext(ctx,
		`UPDATE endpoints SET name = ?, url = ?, interval_seconds = ?, fail_threshold = ? WHERE id = ?`,
		ep.Name, ep.URL, ep.IntervalSeconds, ep.FailThreshold, ep.ID)
	return err
}

func (d *DB) DeleteEndpoint(ctx context.Context, id int64) error {
	_, err := d.conn.ExecContext(ctx, `DELETE FROM endpoints WHERE id = ?`, id)
	return err
}

func (d *DB) TogglePause(ctx context.Context, id int64) (bool, error) {
	var paused bool
	if err := d.conn.QueryRowContext(ctx, `SELECT paused FROM endpoints WHERE id = ?`, id).Scan(&paused); err != nil {
		return false, err
	}
	newPaused := !paused
	_, err := d.conn.ExecContext(ctx, `UPDATE endpoints SET paused = ? WHERE id = ?`, newPaused, id)
	return newPaused, err
}

func (d *DB) ListActiveEndpoints(ctx context.Context) ([]model.Endpoint, error) {
	rows, err := d.conn.QueryContext(ctx,
		`SELECT id, name, url, interval_seconds, fail_threshold, paused, created_at FROM endpoints WHERE paused = 0`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var endpoints []model.Endpoint
	for rows.Next() {
		var ep model.Endpoint
		if err := rows.Scan(&ep.ID, &ep.Name, &ep.URL, &ep.IntervalSeconds, &ep.FailThreshold, &ep.Paused, &ep.CreatedAt); err != nil {
			return nil, err
		}
		endpoints = append(endpoints, ep)
	}
	return endpoints, rows.Err()
}

func (d *DB) ListEndpointsWithStats(ctx context.Context) ([]model.EndpointWithStats, error) {
	query := `
SELECT 
    e.id, e.name, e.url, e.interval_seconds, e.fail_threshold, e.paused, e.created_at,
    c.status_code, c.response_time_ms, c.is_up, c.checked_at,
    u.uptime_pct
FROM endpoints e
LEFT JOIN (
    SELECT endpoint_id, status_code, response_time_ms, is_up, checked_at
    FROM checks
    WHERE (endpoint_id, checked_at) IN (
        SELECT endpoint_id, MAX(checked_at) FROM checks GROUP BY endpoint_id
    )
) c ON c.endpoint_id = e.id
LEFT JOIN (
    SELECT endpoint_id, ROUND(AVG(is_up) * 100, 2) as uptime_pct
    FROM checks 
    WHERE checked_at >= datetime('now', '-30 days') 
    GROUP BY endpoint_id
) u ON u.endpoint_id = e.id
ORDER BY e.created_at DESC
`
	rows, err := d.conn.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []model.EndpointWithStats
	for rows.Next() {
		var s model.EndpointWithStats
		var statusCode sql.NullInt64
		var responseTimeMs sql.NullInt64
		var isUp sql.NullBool
		var checkedAt sql.NullTime
		var uptime30d sql.NullFloat64

		err := rows.Scan(
			&s.ID, &s.Name, &s.URL, &s.IntervalSeconds, &s.FailThreshold, &s.Paused, &s.CreatedAt,
			&statusCode, &responseTimeMs, &isUp, &checkedAt,
			&uptime30d,
		)
		if err != nil {
			return nil, err
		}
		if statusCode.Valid {
			v := int(statusCode.Int64)
			s.StatusCode = &v
		}
		if responseTimeMs.Valid {
			v := int(responseTimeMs.Int64)
			s.ResponseTimeMs = &v
		}
		if isUp.Valid {
			v := isUp.Bool
			s.IsUp = &v
		}
		if checkedAt.Valid {
			s.CheckedAt = &checkedAt.Time
		}
		if uptime30d.Valid {
			v := uptime30d.Float64
			s.Uptime30d = &v
		}
		results = append(results, s)
	}
	return results, rows.Err()
}
