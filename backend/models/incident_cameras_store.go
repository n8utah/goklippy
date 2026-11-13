package models

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"time"

	"goklippy/backend/logic/sorter"

	"github.com/jmoiron/sqlx"
)

// Every channel's picture and state when an incident opened (sorter.IncidentCameraStore).

func (s *RecordsStore) AddIncidentCamera(ctx context.Context, incidentID int64, cam sorter.IncidentCamera, path string) error {
	if s.db == nil {
		return ErrDBNotConfigured
	}
	state, err := json.Marshal(cam.State)
	if err != nil {
		return err
	}
	const query = `
INSERT INTO incident_cameras (incident_id, channel, path, taken_at, state) VALUES (?, ?, ?, ?, ?)
ON CONFLICT (incident_id, channel) DO UPDATE SET path = excluded.path, taken_at = excluded.taken_at, state = excluded.state
`
	_, err = s.db.ExecContext(ctx, query, incidentID, string(cam.Channel), path, cam.TakenAt.UTC(), string(state))
	return err
}

func (s *RecordsStore) IncidentCameraPath(ctx context.Context, incidentID int64, ch sorter.Channel) (string, error) {
	if s.db == nil {
		return "", ErrDBNotConfigured
	}
	var path string
	err := s.db.GetContext(ctx, &path, `SELECT path FROM incident_cameras WHERE incident_id = ? AND channel = ?`, incidentID, string(ch))
	if errors.Is(err, sql.ErrNoRows) || (err == nil && path == "") {
		return "", sorter.ErrIncidentImageNotFound
	}
	return path, err
}

func (s *RecordsStore) ClearIncidentCameras(ctx context.Context) (int, error) {
	if s.db == nil {
		return 0, ErrDBNotConfigured
	}
	res, err := s.db.ExecContext(ctx, `DELETE FROM incident_cameras`)
	if err != nil {
		return 0, err
	}
	n, err := res.RowsAffected()
	return int(n), err
}

func (s *RecordsStore) DeleteIncidentsExcept(ctx context.Context, keep int64) ([]int64, error) {
	if s.db == nil {
		return nil, ErrDBNotConfigured
	}
	tx, err := s.db.BeginTxx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()
	var ids []int64
	if err := tx.SelectContext(ctx, &ids, `SELECT id FROM incidents WHERE id <> ? ORDER BY id`, keep); err != nil {
		return nil, err
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM incident_cameras WHERE incident_id <> ?`, keep); err != nil {
		return nil, err
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM incidents WHERE id <> ?`, keep); err != nil {
		return nil, err
	}
	return ids, tx.Commit()
}

// attachIncidentCameras gives each listed incident its kept cameras, C1 to C4.
func (s *RecordsStore) attachIncidentCameras(ctx context.Context, incs []sorter.Incident) error {
	if len(incs) == 0 {
		return nil
	}
	ids := make([]any, len(incs))
	index := make(map[int64]int, len(incs))
	for i, inc := range incs {
		ids[i] = inc.ID
		index[inc.ID] = i
	}
	query, args, err := sqlx.In(`SELECT incident_id, channel, path, taken_at, state FROM incident_cameras
WHERE incident_id IN (?) ORDER BY incident_id, channel`, ids)
	if err != nil {
		return err
	}
	var rows []struct {
		IncidentID int64     `db:"incident_id"`
		Channel    string    `db:"channel"`
		Path       string    `db:"path"`
		TakenAt    time.Time `db:"taken_at"`
		State      string    `db:"state"`
	}
	if err := s.db.SelectContext(ctx, &rows, s.db.Rebind(query), args...); err != nil {
		return err
	}
	for _, r := range rows {
		cam := sorter.IncidentCamera{Channel: sorter.Channel(r.Channel), Image: r.Path != "", TakenAt: r.TakenAt}
		_ = json.Unmarshal([]byte(r.State), &cam.State)
		inc := &incs[index[r.IncidentID]]
		inc.Cameras = append(inc.Cameras, cam)
	}
	return nil
}
