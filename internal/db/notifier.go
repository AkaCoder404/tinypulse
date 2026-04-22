package db

import (
	"context"

	"tinypulse/internal/model"
)

func (d *DB) CreateNotifier(ctx context.Context, n *model.Notifier) error {
	res, err := d.conn.ExecContext(ctx,
		`INSERT INTO notifiers (name, type, config_json, source) VALUES (?, ?, ?, 'ui')`,
		n.Name, n.Type, n.ConfigJSON)
	if err != nil {
		return err
	}
	id, _ := res.LastInsertId()
	n.ID = id
	return nil
}

func (d *DB) GetNotifier(ctx context.Context, id int64) (*model.Notifier, error) {
	row := d.conn.QueryRowContext(ctx,
		`SELECT id, uid, source, name, type, config_json, created_at FROM notifiers WHERE id = ?`, id)
	n := &model.Notifier{}
	if err := row.Scan(&n.ID, &n.UID, &n.Source, &n.Name, &n.Type, &n.ConfigJSON, &n.CreatedAt); err != nil {
		return nil, err
	}
	return n, nil
}

func (d *DB) UpdateNotifier(ctx context.Context, n *model.Notifier) error {
	_, err := d.conn.ExecContext(ctx,
		`UPDATE notifiers SET name = ?, type = ?, config_json = ? WHERE id = ?`,
		n.Name, n.Type, n.ConfigJSON, n.ID)
	return err
}

func (d *DB) DeleteNotifier(ctx context.Context, id int64) error {
	_, err := d.conn.ExecContext(ctx, `DELETE FROM notifiers WHERE id = ?`, id)
	return err
}

func (d *DB) ListNotifiers(ctx context.Context) ([]model.Notifier, error) {
	rows, err := d.conn.QueryContext(ctx, `SELECT id, uid, source, name, type, config_json, created_at FROM notifiers ORDER BY id DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var notifiers []model.Notifier
	for rows.Next() {
		var n model.Notifier
		if err := rows.Scan(&n.ID, &n.UID, &n.Source, &n.Name, &n.Type, &n.ConfigJSON, &n.CreatedAt); err != nil {
			return nil, err
		}
		notifiers = append(notifiers, n)
	}
	return notifiers, rows.Err()
}

func (d *DB) GetNotifiersForEndpoint(ctx context.Context, endpointID int64) ([]model.Notifier, error) {
	rows, err := d.conn.QueryContext(ctx, `
		SELECT n.id, n.uid, n.source, n.name, n.type, n.config_json, n.created_at 
		FROM notifiers n
		JOIN endpoint_notifiers en ON n.id = en.notifier_id
		WHERE en.endpoint_id = ?`, endpointID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var notifiers []model.Notifier
	for rows.Next() {
		var n model.Notifier
		if err := rows.Scan(&n.ID, &n.UID, &n.Source, &n.Name, &n.Type, &n.ConfigJSON, &n.CreatedAt); err != nil {
			return nil, err
		}
		notifiers = append(notifiers, n)
	}
	return notifiers, rows.Err()
}

// SetEndpointNotifiers replaces all linked notifiers for an endpoint within a transaction.
func (d *DB) SetEndpointNotifiers(ctx context.Context, endpointID int64, notifierIDs []int64) error {
	tx, err := d.conn.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(ctx, `DELETE FROM endpoint_notifiers WHERE endpoint_id = ?`, endpointID); err != nil {
		return err
	}

	for _, nid := range notifierIDs {
		if _, err := tx.ExecContext(ctx, `INSERT INTO endpoint_notifiers (endpoint_id, notifier_id) VALUES (?, ?)`, endpointID, nid); err != nil {
			return err
		}
	}

	return tx.Commit()
}

// GetEndpointNotifierIDs returns a map of endpoint_id -> []notifier_id
func (d *DB) GetEndpointNotifierIDs(ctx context.Context) (map[int64][]int64, error) {
	rows, err := d.conn.QueryContext(ctx, `SELECT endpoint_id, notifier_id FROM endpoint_notifiers`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	m := make(map[int64][]int64)
	for rows.Next() {
		var eid, nid int64
		if err := rows.Scan(&eid, &nid); err != nil {
			return nil, err
		}
		m[eid] = append(m[eid], nid)
	}
	return m, rows.Err()
}
