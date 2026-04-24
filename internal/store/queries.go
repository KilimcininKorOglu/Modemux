package store

import (
	"context"
	"fmt"
	"time"
)

type IPRotation struct {
	ID         int64     `json:"id"`
	ModemID    string    `json:"modemId"`
	OldIP      string    `json:"oldIp"`
	NewIP      string    `json:"newIp"`
	DurationMs int64     `json:"durationMs"`
	RotatedAt  time.Time `json:"rotatedAt"`
}

type ModemEvent struct {
	ID        int64     `json:"id"`
	ModemID   string    `json:"modemId"`
	Event     string    `json:"event"`
	Detail    string    `json:"detail"`
	Timestamp time.Time `json:"timestamp"`
}

func (s *Store) InsertRotation(ctx context.Context, modemID, oldIP, newIP string, durationMs int64) (int64, error) {
	result, err := s.db.ExecContext(ctx,
		"INSERT INTO ip_rotations (modem_id, old_ip, new_ip, duration_ms) VALUES (?, ?, ?, ?)",
		modemID, oldIP, newIP, durationMs,
	)
	if err != nil {
		return 0, err
	}
	return result.LastInsertId()
}

func (s *Store) InsertEvent(ctx context.Context, modemID, event, detail string) error {
	_, err := s.db.ExecContext(ctx,
		"INSERT INTO modem_events (modem_id, event, detail) VALUES (?, ?, ?)",
		modemID, event, detail,
	)
	return err
}

func (s *Store) GetRotations(ctx context.Context, modemID string, limit, offset int) ([]IPRotation, error) {
	query := "SELECT id, modem_id, old_ip, new_ip, duration_ms, rotated_at FROM ip_rotations"
	args := []interface{}{}

	if modemID != "" {
		query += " WHERE modem_id = ?"
		args = append(args, modemID)
	}

	query += " ORDER BY rotated_at DESC LIMIT ? OFFSET ?"
	args = append(args, limit, offset)

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var rotations []IPRotation
	for rows.Next() {
		var r IPRotation
		if err := rows.Scan(&r.ID, &r.ModemID, &r.OldIP, &r.NewIP, &r.DurationMs, &r.RotatedAt); err != nil {
			return nil, err
		}
		rotations = append(rotations, r)
	}

	return rotations, rows.Err()
}

func (s *Store) GetRotationCount(ctx context.Context, modemID string) (int, error) {
	query := "SELECT COUNT(*) FROM ip_rotations"
	args := []interface{}{}

	if modemID != "" {
		query += " WHERE modem_id = ?"
		args = append(args, modemID)
	}

	var count int
	err := s.db.QueryRowContext(ctx, query, args...).Scan(&count)
	return count, err
}

func (s *Store) GetUniqueIPCount(ctx context.Context, modemID string) (int, error) {
	query := "SELECT COUNT(DISTINCT new_ip) FROM ip_rotations"
	args := []interface{}{}

	if modemID != "" {
		query += " WHERE modem_id = ?"
		args = append(args, modemID)
	}

	var count int
	err := s.db.QueryRowContext(ctx, query, args...).Scan(&count)
	return count, err
}

func (s *Store) GetEvents(ctx context.Context, modemID string, limit int) ([]ModemEvent, error) {
	query := "SELECT id, modem_id, event, detail, timestamp FROM modem_events"
	args := []interface{}{}

	if modemID != "" {
		query += " WHERE modem_id = ?"
		args = append(args, modemID)
	}

	query += " ORDER BY timestamp DESC LIMIT ?"
	args = append(args, limit)

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var events []ModemEvent
	for rows.Next() {
		var e ModemEvent
		if err := rows.Scan(&e.ID, &e.ModemID, &e.Event, &e.Detail, &e.Timestamp); err != nil {
			return nil, err
		}
		events = append(events, e)
	}

	return events, rows.Err()
}

func (s *Store) DeleteOldRotations(ctx context.Context, retentionDays int) (int64, error) {
	result, err := s.db.ExecContext(ctx,
		"DELETE FROM ip_rotations WHERE rotated_at < datetime('now', ?)",
		fmt.Sprintf("-%d days", retentionDays),
	)
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}

func (s *Store) DeleteOldEvents(ctx context.Context, retentionDays int) (int64, error) {
	result, err := s.db.ExecContext(ctx,
		"DELETE FROM modem_events WHERE timestamp < datetime('now', ?)",
		fmt.Sprintf("-%d days", retentionDays),
	)
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}
