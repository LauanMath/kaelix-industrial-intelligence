package store

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	c "kaelix/contracts"
)

func (s *Store) Ingest(ctx context.Context, b c.ReadingBatch) (c.IngestResult, error) {
	out := c.IngestResult{SchemaVersion: 1, GatewayID: b.GatewayID, GatewaySessionID: b.GatewaySessionID, Acknowledged: []c.Acknowledgement{}}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return out, err
	}
	defer tx.Rollback(ctx)
	now := time.Now().UTC()
	var lockedGateway string
	if err = tx.QueryRow(ctx, `SELECT id FROM gateways WHERE id=$1 FOR UPDATE`, b.GatewayID).Scan(&lockedGateway); err != nil {
		return out, dbError(err)
	}
	out.ServerReceivedAt = now
	if _, err = tx.Exec(ctx, `INSERT INTO gateway_sessions(gateway_id,session_id,first_seen_at) VALUES($1,$2,$3) ON CONFLICT DO NOTHING`, b.GatewayID, b.GatewaySessionID, now); err != nil {
		return out, err
	}
	for _, r := range b.Readings {
		if _, err = tx.Exec(ctx, `INSERT INTO devices(id) VALUES($1) ON CONFLICT DO NOTHING`, r.DeviceID); err != nil {
			return out, err
		}
		payload, err := json.Marshal(r)
		if err != nil {
			return out, err
		}
		delay := b.UploadUptimeMS - r.GatewayRxUptimeMS
		received := estimatedReception(now, delay, r.GatewayReceivedAt)
		status := "stored"
		var id int64
		err = tx.QueryRow(ctx, `INSERT INTO readings(gateway_id,gateway_session_id,event_seq,device_id,ingested_at,estimated_received_at,queue_delay_ms,payload) VALUES($1,$2,$3,$4,$5,$6,$7,$8) ON CONFLICT(gateway_id,gateway_session_id,event_seq) DO NOTHING RETURNING id`, b.GatewayID, b.GatewaySessionID, r.EventSeq, r.DeviceID, now, received, delay, payload).Scan(&id)
		if errors.Is(err, pgx.ErrNoRows) {
			var same bool
			if err = tx.QueryRow(ctx, `SELECT payload=$4::jsonb FROM readings WHERE gateway_id=$1 AND gateway_session_id=$2 AND event_seq=$3`, b.GatewayID, b.GatewaySessionID, r.EventSeq, payload).Scan(&same); err != nil {
				return out, err
			}
			if !same {
				return out, ErrConflict
			}
			status = "duplicate"
		} else if err != nil {
			return out, err
		}
		out.Acknowledged = append(out.Acknowledged, c.Acknowledgement{EventSeq: r.EventSeq, Status: status})
	}
	if _, err = tx.Exec(ctx, `UPDATE gateways SET last_ingested_at=$2 WHERE id=$1`, b.GatewayID, now); err != nil {
		return out, err
	}
	if err = tx.Commit(ctx); err != nil {
		return out, err
	}
	return out, nil
}

// Safe integer uptimes can exceed time.Duration's range. Keep the delay even if
// its estimated timestamp cannot be represented as a RFC3339/PostgreSQL date.
func estimatedReception(now time.Time, delay int64, known *time.Time) *time.Time {
	if known != nil {
		t := known.UTC()
		return &t
	}
	t := time.Unix(now.Unix()-delay/1000, int64(now.Nanosecond())-(delay%1000)*int64(time.Millisecond)).UTC()
	if t.Year() < 1 || t.Year() > 9999 {
		return nil
	}
	return &t
}

type Cursor struct {
	DeviceID string    `json:"device"`
	Time     time.Time `json:"time"`
	ID       int64     `json:"id"`
}

func ParseCursor(value string, device string) (*Cursor, error) {
	if value == "" {
		return nil, nil
	}
	if len(value) > 512 {
		return nil, fmt.Errorf("invalid cursor")
	}
	raw, err := base64.RawURLEncoding.DecodeString(value)
	if err != nil {
		return nil, fmt.Errorf("invalid cursor")
	}
	var c Cursor
	if json.Unmarshal(raw, &c) != nil || c.DeviceID != device || (c.Time.Year() < 1 || c.Time.Year() > 9999) || c.ID < 1 {
		return nil, fmt.Errorf("invalid cursor")
	}
	return &c, nil
}

type HistoryFilter struct {
	From   *time.Time
	To     *time.Time
	Cursor *Cursor
	Limit  int
}

func (s *Store) MotorDevice(ctx context.Context, id string) (*string, error) {
	var device *string
	err := s.pool.QueryRow(ctx, `SELECT device_id FROM motors WHERE id=$1`, id).Scan(&device)
	return device, dbError(err)
}
func (s *Store) DeviceExists(ctx context.Context, id string) error {
	var ok bool
	if err := s.pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM devices WHERE id=$1)`, id).Scan(&ok); err != nil {
		return err
	}
	if !ok {
		return ErrNotFound
	}
	return nil
}
func (s *Store) History(ctx context.Context, device string, f HistoryFilter) (c.HistoryPage, error) {
	var cursorTime *time.Time
	var cursorID int64
	if f.Cursor != nil {
		cursorTime = &f.Cursor.Time
		cursorID = f.Cursor.ID
	}
	rows, err := s.pool.Query(ctx, `SELECT id,gateway_id,gateway_session_id,payload,ingested_at,estimated_received_at,queue_delay_ms FROM readings WHERE device_id=$1 AND ($2::timestamptz IS NULL OR estimated_received_at >= $2) AND ($3::timestamptz IS NULL OR estimated_received_at < $3) AND ($4::timestamptz IS NULL OR (COALESCE(estimated_received_at,'0001-01-01T00:00:00Z'::timestamptz),id) < ($4,$5::bigint)) ORDER BY COALESCE(estimated_received_at,'0001-01-01T00:00:00Z'::timestamptz) DESC,id DESC LIMIT $6`, device, f.From, f.To, cursorTime, cursorID, f.Limit+1)
	page := c.HistoryPage{Items: []c.Measurement{}}
	if err != nil {
		return page, err
	}
	defer rows.Close()
	for rows.Next() {
		var m c.Measurement
		var raw []byte
		if err = rows.Scan(&m.ID, &m.GatewayID, &m.GatewaySessionID, &raw, &m.IngestedAt, &m.EstimatedReceivedAt, &m.QueueDelayMS); err != nil {
			return page, err
		}
		if err = json.Unmarshal(raw, &m.Reading); err != nil {
			return page, err
		}
		page.Items = append(page.Items, m)
	}
	if err = rows.Err(); err != nil {
		return page, err
	}
	if len(page.Items) > f.Limit {
		page.Items = page.Items[:f.Limit]
		last := page.Items[len(page.Items)-1]
		position := time.Date(1, 1, 1, 0, 0, 0, 0, time.UTC)
		if last.EstimatedReceivedAt != nil {
			position = *last.EstimatedReceivedAt
		}
		raw, _ := json.Marshal(Cursor{DeviceID: device, Time: position, ID: last.ID})
		page.NextCursor = base64.RawURLEncoding.EncodeToString(raw)
	}
	return page, nil
}

func (s *Store) RecentReadings(ctx context.Context)([]c.Measurement,error){
 rows,err:=s.pool.Query(ctx,`SELECT id,gateway_id,gateway_session_id,payload,ingested_at,estimated_received_at,queue_delay_ms FROM readings ORDER BY ingested_at DESC,id DESC LIMIT 50`);if err!=nil{return nil,err};defer rows.Close()
 out:=[]c.Measurement{}
 for rows.Next(){var m c.Measurement;var raw []byte;if err=rows.Scan(&m.ID,&m.GatewayID,&m.GatewaySessionID,&raw,&m.IngestedAt,&m.EstimatedReceivedAt,&m.QueueDelayMS);err!=nil{return nil,err};if err=json.Unmarshal(raw,&m.Reading);err!=nil{return nil,err};out=append(out,m)}
 return out,rows.Err()
}
