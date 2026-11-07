package store

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	c "kaelix/contracts"
)

// receptionFor applies the timing rules shared by readings and windows. A previous boot's
// uptime is not comparable with the current clock, so there is no queue delay to measure
// and, without a gateway timestamp, no time to estimate.
func receptionFor(now time.Time, previous bool, upload, rx int64, known *time.Time) (*time.Time, int64) {
	if previous {
		if known == nil {
			return nil, 0
		}
		t := known.UTC()
		return &t, 0
	}
	delay := upload - rx
	return estimatedReception(now, delay, known), delay
}

// IngestWaveform stores a window once per (gateway, session, event). Retries are
// acknowledged as duplicates; the same identity with other content is a conflict.
func (s *Store) IngestWaveform(ctx context.Context, w c.WaveformUpload, samples []byte) (c.IngestResult, error) {
	out := c.IngestResult{SchemaVersion: 1, GatewayID: w.GatewayID, GatewaySessionID: w.GatewaySessionID, Acknowledged: []c.Acknowledgement{}}
	sum := sha256.Sum256(samples)
	missing := w.FragmentsMissing
	if missing == nil {
		missing = []int{}
	}
	metadata, err := json.Marshal(c.WaveformMetadata{GatewayRxUptimeMS: w.GatewayRxUptimeMS, GatewayReceivedAt: w.GatewayReceivedAt, DeviceSequence: *w.DeviceSequence, BootCount: *w.BootCount, Channels: w.Channels, WindowSamples: w.WindowSamples, SampleRateHz: *w.SampleRateHz, AccelRange: w.AccelRange, GyroRange: w.GyroRange, DLPF: w.DLPF, BurstProfile: w.BurstProfile, FragmentTotal: w.FragmentTotal, FragmentsMissing: missing, SamplesSHA256: hex.EncodeToString(sum[:])})
	if err != nil {
		return out, err
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return out, err
	}
	defer tx.Rollback(ctx)
	now := time.Now().UTC()
	out.ServerReceivedAt = now
	var locked string
	if err = tx.QueryRow(ctx, `SELECT id FROM gateways WHERE id=$1 FOR UPDATE`, w.GatewayID).Scan(&locked); err != nil {
		return out, dbError(err)
	}
	if _, err = tx.Exec(ctx, `INSERT INTO gateway_sessions(gateway_id,session_id,first_seen_at) VALUES($1,$2,$3) ON CONFLICT DO NOTHING`, w.GatewayID, w.GatewaySessionID, now); err != nil {
		return out, err
	}
	if _, err = tx.Exec(ctx, `INSERT INTO devices(id) VALUES($1) ON CONFLICT DO NOTHING`, w.DeviceID); err != nil {
		return out, err
	}
	received, delay := receptionFor(now, w.PreviousSession, w.UploadUptimeMS, w.GatewayRxUptimeMS, w.GatewayReceivedAt)
	status := "stored"
	var id int64
	err = tx.QueryRow(ctx, `INSERT INTO waveforms(gateway_id,gateway_session_id,event_seq,device_id,ingested_at,estimated_received_at,queue_delay_ms,metadata,samples) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9) ON CONFLICT(gateway_id,gateway_session_id,event_seq) DO NOTHING RETURNING id`, w.GatewayID, w.GatewaySessionID, w.EventSeq, w.DeviceID, now, received, delay, metadata, samples).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		var same bool
		if err = tx.QueryRow(ctx, `SELECT metadata=$4::jsonb AND device_id=$5 FROM waveforms WHERE gateway_id=$1 AND gateway_session_id=$2 AND event_seq=$3`, w.GatewayID, w.GatewaySessionID, w.EventSeq, metadata, w.DeviceID).Scan(&same); err != nil {
			return out, err
		}
		if !same {
			return out, ErrConflict
		}
		status = "duplicate"
	} else if err != nil {
		return out, err
	}
	out.Acknowledged = append(out.Acknowledged, c.Acknowledgement{EventSeq: w.EventSeq, Status: status})
	if _, err = tx.Exec(ctx, `UPDATE gateways SET last_ingested_at=$2 WHERE id=$1`, w.GatewayID, now); err != nil {
		return out, err
	}
	return out, tx.Commit(ctx)
}

func scanWaveform(scan func(...any) error, extra ...any) (c.Waveform, error) {
	var w c.Waveform
	var raw []byte
	targets := append([]any{&w.ID, &w.GatewayID, &w.GatewaySessionID, &w.EventSeq, &w.DeviceID, &raw, &w.IngestedAt, &w.EstimatedReceivedAt, &w.QueueDelayMS}, extra...)
	if err := scan(targets...); err != nil {
		return w, err
	}
	return w, json.Unmarshal(raw, &w.WaveformMetadata)
}

const waveformColumns = `id,gateway_id,gateway_session_id,event_seq,device_id,metadata,ingested_at,estimated_received_at,queue_delay_ms`

// Waveforms lists windows without samples, newest first, in the same order and with the
// same cursor as the reading history.
func (s *Store) Waveforms(ctx context.Context, device string, f HistoryFilter) (c.WaveformPage, error) {
	page := c.WaveformPage{Items: []c.Waveform{}}
	var cursorTime *time.Time
	var cursorID int64
	if f.Cursor != nil {
		cursorTime = &f.Cursor.Time
		cursorID = f.Cursor.ID
	}
	rows, err := s.pool.Query(ctx, `SELECT `+waveformColumns+` FROM waveforms WHERE device_id=$1 AND ($2::timestamptz IS NULL OR estimated_received_at >= $2) AND ($3::timestamptz IS NULL OR estimated_received_at < $3) AND ($4::timestamptz IS NULL OR (COALESCE(estimated_received_at,'0001-01-01T00:00:00Z'::timestamptz),id) < ($4,$5::bigint)) ORDER BY COALESCE(estimated_received_at,'0001-01-01T00:00:00Z'::timestamptz) DESC,id DESC LIMIT $6`, device, f.From, f.To, cursorTime, cursorID, f.Limit+1)
	if err != nil {
		return page, err
	}
	defer rows.Close()
	for rows.Next() {
		w, err := scanWaveform(rows.Scan)
		if err != nil {
			return page, err
		}
		page.Items = append(page.Items, w)
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

// Waveform returns a window with its samples.
func (s *Store) Waveform(ctx context.Context, id int64) (c.Waveform, error) {
	var samples []byte
	row := s.pool.QueryRow(ctx, `SELECT `+waveformColumns+`,samples FROM waveforms WHERE id=$1`, id)
	w, err := scanWaveform(row.Scan, &samples)
	if err != nil {
		return w, dbError(err)
	}
	w.SamplesB64 = base64.StdEncoding.EncodeToString(samples)
	return w, nil
}
