package db

import (
	"context"

	"tinypulse/internal/model"
)

func (d *DB) CreateEndpoint(ctx context.Context, ep *model.Endpoint) error {
	if ep.Type == "" {
		ep.Type = "http"
	}
	res, err := d.conn.ExecContext(ctx,
		`INSERT INTO endpoints (type, name, url, interval_seconds, fail_threshold, paused) VALUES (?, ?, ?, ?, ?, ?)`,
		ep.Type, ep.Name, ep.URL, ep.IntervalSeconds, ep.FailThreshold, ep.Paused)
	if err != nil {
		return err
	}
	id, _ := res.LastInsertId()
	created, err := d.GetEndpoint(ctx, id)
	if err != nil {
		return err
	}
	notifierIDs := ep.NotifierIDs
	*ep = *created
	ep.NotifierIDs = notifierIDs
	return nil
}

func (d *DB) GetEndpoint(ctx context.Context, id int64) (*model.Endpoint, error) {
	row := d.conn.QueryRowContext(ctx,
		`SELECT id, type, name, url, interval_seconds, fail_threshold, paused, created_at FROM endpoints WHERE id = ?`, id)
	ep := &model.Endpoint{}
	err := row.Scan(&ep.ID, &ep.Type, &ep.Name, &ep.URL, &ep.IntervalSeconds, &ep.FailThreshold, &ep.Paused, &ep.CreatedAt)
	if err != nil {
		return nil, err
	}
	return ep, nil
}

func (d *DB) UpdateEndpoint(ctx context.Context, ep *model.Endpoint) error {
	if ep.Type == "" {
		ep.Type = "http"
	}
	_, err := d.conn.ExecContext(ctx,
		`UPDATE endpoints SET type = ?, name = ?, url = ?, interval_seconds = ?, fail_threshold = ? WHERE id = ?`,
		ep.Type, ep.Name, ep.URL, ep.IntervalSeconds, ep.FailThreshold, ep.ID)
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
		`SELECT id, type, name, url, interval_seconds, fail_threshold, paused, created_at FROM endpoints WHERE paused = 0`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var endpoints []model.Endpoint
	for rows.Next() {
		var ep model.Endpoint
		if err := rows.Scan(&ep.ID, &ep.Type, &ep.Name, &ep.URL, &ep.IntervalSeconds, &ep.FailThreshold, &ep.Paused, &ep.CreatedAt); err != nil {
			return nil, err
		}
		endpoints = append(endpoints, ep)
	}
	return endpoints, rows.Err()
}


func (d *DB) ListEndpoints(ctx context.Context) ([]model.Endpoint, error) {
	rows, err := d.conn.QueryContext(ctx,
		`SELECT id, type, name, url, interval_seconds, fail_threshold, paused, created_at FROM endpoints ORDER BY created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var endpoints []model.Endpoint
	for rows.Next() {
		var ep model.Endpoint
		if err := rows.Scan(&ep.ID, &ep.Type, &ep.Name, &ep.URL, &ep.IntervalSeconds, &ep.FailThreshold, &ep.Paused, &ep.CreatedAt); err != nil {
			return nil, err
		}
		endpoints = append(endpoints, ep)
	}
	return endpoints, rows.Err()
}
