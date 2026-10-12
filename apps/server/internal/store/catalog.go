package store

import (
	"context"
	"encoding/json"
	"time"

	c "kaelix/contracts"
)

const latestJSON = `r.payload || jsonb_build_object('id',r.id,'gateway_id',r.gateway_id,'gateway_session_id',r.gateway_session_id,'ingested_at',r.ingested_at,'estimated_received_at',r.estimated_received_at,'queue_delay_ms',r.queue_delay_ms)`

func decodeLatest(raw []byte) (*c.Measurement, error) {
	if len(raw) == 0 {
		return nil, nil
	}
	var m c.Measurement
	err := json.Unmarshal(raw, &m)
	return &m, err
}
func (s *Store) Devices(ctx context.Context) ([]c.Device, error) {
	rows, err := s.pool.Query(ctx, `SELECT d.id,d.profile,d.notes,m.id,d.created_at,l.latest FROM devices d LEFT JOIN motors m ON m.device_id=d.id LEFT JOIN LATERAL (SELECT `+latestJSON+` AS latest FROM readings r WHERE r.device_id=d.id ORDER BY COALESCE(r.estimated_received_at,'0001-01-01T00:00:00Z'::timestamptz) DESC,r.id DESC LIMIT 1) l ON true ORDER BY d.id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []c.Device{}
	for rows.Next() {
		var d c.Device
		var raw []byte
		if err = rows.Scan(&d.ID, &d.Profile, &d.Notes, &d.MotorID, &d.CreatedAt, &raw); err != nil {
			return nil, err
		}
		if d.Latest, err = decodeLatest(raw); err != nil {
			return nil, err
		}
		out = append(out, d)
	}
	return out, rows.Err()
}
func (s *Store) CreateDevice(ctx context.Context, d c.DeviceInput) (c.Device, error) {
	var out c.Device
	err := s.pool.QueryRow(ctx, `INSERT INTO devices(id,profile,notes) VALUES($1,$2,$3) RETURNING id,profile,notes,created_at`, d.ID, d.Profile, d.Notes).Scan(&out.ID, &out.Profile, &out.Notes, &out.CreatedAt)
	return out, dbError(err)
}
func (s *Store) UpdateDevice(ctx context.Context, d c.DeviceInput) (c.Device, error) {
	var out c.Device
	err := s.pool.QueryRow(ctx, `UPDATE devices SET profile=$2,notes=$3 WHERE id=$1 RETURNING id,profile,notes,created_at,(SELECT id FROM motors WHERE device_id=$1)`, d.ID, d.Profile, d.Notes).Scan(&out.ID, &out.Profile, &out.Notes, &out.CreatedAt, &out.MotorID)
	return out, dbError(err)
}
func (s *Store) Motors(ctx context.Context) ([]c.Motor, error) {
	rows, err := s.pool.Query(ctx, `SELECT m.id,m.name,m.location,m.device_id,m.expected_interval_seconds,m.created_at,l.latest FROM motors m LEFT JOIN LATERAL (SELECT `+latestJSON+` AS latest FROM readings r WHERE r.device_id=m.device_id ORDER BY COALESCE(r.estimated_received_at,'0001-01-01T00:00:00Z'::timestamptz) DESC,r.id DESC LIMIT 1) l ON true ORDER BY m.id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []c.Motor{}
	for rows.Next() {
		var m c.Motor
		var raw []byte
		if err = rows.Scan(&m.ID, &m.Name, &m.Location, &m.DeviceID, &m.ExpectedIntervalSeconds, &m.CreatedAt, &raw); err != nil {
			return nil, err
		}
		if m.Latest, err = decodeLatest(raw); err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}
func (s *Store) SaveMotor(ctx context.Context, m c.MotorInput, update bool) (c.Motor, error) {
	var out c.Motor
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return out, err
	}
	defer tx.Rollback(ctx)
	if m.DeviceID != nil {
		if _, err = tx.Exec(ctx, `INSERT INTO devices(id) VALUES($1) ON CONFLICT DO NOTHING`, *m.DeviceID); err != nil {
			return out, dbError(err)
		}
	}
	sql := `INSERT INTO motors(id,name,location,device_id,expected_interval_seconds) VALUES($1,$2,$3,$4,$5) RETURNING id,name,location,device_id,expected_interval_seconds,created_at`
	if update {
		sql = `UPDATE motors SET name=$2,location=$3,device_id=$4,expected_interval_seconds=$5 WHERE id=$1 RETURNING id,name,location,device_id,expected_interval_seconds,created_at`
	}
	err = tx.QueryRow(ctx, sql, m.ID, m.Name, m.Location, m.DeviceID, m.ExpectedIntervalSeconds).Scan(&out.ID, &out.Name, &out.Location, &out.DeviceID, &out.ExpectedIntervalSeconds, &out.CreatedAt)
	if err != nil {
		return out, dbError(err)
	}
	return out, tx.Commit(ctx)
}
func (s *Store) CreateGateway(ctx context.Context, g c.GatewayInput) (c.Gateway, string, error) {
	if g.ID == "" {
		g.ID = NewUUID()
	}
	token := NewToken()
	var out c.Gateway
	out.GatewayInput = g
	err := s.pool.QueryRow(ctx, `INSERT INTO gateways(id,name,location,token_hash) VALUES($1,$2,$3,$4) RETURNING created_at`, g.ID, g.Name, g.Location, tokenHash(token)).Scan(&out.CreatedAt)
	if err != nil {
		return out, "", dbError(err)
	}
	return out, token, nil
}
func (s *Store) RotateGatewayToken(ctx context.Context, id string) (string, error) {
	token := NewToken()
	tag, err := s.pool.Exec(ctx, `UPDATE gateways SET token_hash=$2 WHERE id=$1`, id, tokenHash(token))
	if err != nil {
		return "", err
	}
	if tag.RowsAffected() == 0 {
		return "", ErrNotFound
	}
	return token, nil
}
func (s *Store) Gateways(ctx context.Context) ([]c.Gateway, error) {
	rows, err := s.pool.Query(ctx, `SELECT id,name,location,created_at,last_ingested_at,last_heartbeat_at,heartbeat,active_session_id FROM gateways ORDER BY id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []c.Gateway{}
	for rows.Next() {
		var g c.Gateway
		var raw []byte
		if err = rows.Scan(&g.ID, &g.Name, &g.Location, &g.CreatedAt, &g.LastIngestedAt, &g.LastHeartbeatAt, &raw, &g.ActiveSessionID); err != nil {
			return nil, err
		}
		if len(raw) > 0 {
			g.Heartbeat = &c.Heartbeat{}
			if err = json.Unmarshal(raw, g.Heartbeat); err != nil {
				return nil, err
			}
		}
		g.Activity = "UNKNOWN"
		if g.LastHeartbeatAt != nil {
			g.Activity = "RECENT"
			if time.Since(*g.LastHeartbeatAt) >= 90*time.Second {
				g.Activity = "STALE"
			}
		}
		out = append(out, g)
	}
	return out, rows.Err()
}
