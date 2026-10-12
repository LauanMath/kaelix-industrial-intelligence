package store

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/jackc/pgx/v5"
	c "kaelix/contracts"
	"strconv"
	"time"
)

func (s *Store) SaveGatewayLogs(ctx context.Context, b c.GatewayLogBatch) (c.GatewayLogAck, error) {
	out := c.GatewayLogAck{SchemaVersion: 1, GatewayID: b.GatewayID, GatewaySessionID: b.GatewaySessionID, ServerReceivedAt: time.Now().UTC(), Acknowledged: []uint32{}}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return out, err
	}
	defer tx.Rollback(ctx)
	var id string
	if err = tx.QueryRow(ctx, `SELECT id FROM gateways WHERE id=$1 FOR UPDATE`, b.GatewayID).Scan(&id); err != nil {
		return out, dbError(err)
	}
	for _, entry := range b.Logs {
		raw, err := json.Marshal(entry)
		if err != nil {
			return out, err
		}
		var inserted int64
		err = tx.QueryRow(ctx, `INSERT INTO gateway_logs(gateway_id,gateway_session_id,log_seq,received_at,payload) VALUES($1,$2,$3,$4,$5) ON CONFLICT(gateway_id,gateway_session_id,log_seq) DO NOTHING RETURNING id`, b.GatewayID, b.GatewaySessionID, entry.LogSeq, out.ServerReceivedAt, raw).Scan(&inserted)
		if errors.Is(err, pgx.ErrNoRows) {
			var same bool
			if err = tx.QueryRow(ctx, `SELECT payload=$4::jsonb FROM gateway_logs WHERE gateway_id=$1 AND gateway_session_id=$2 AND log_seq=$3`, b.GatewayID, b.GatewaySessionID, entry.LogSeq, raw).Scan(&same); err != nil {
				return out, err
			}
			if !same {
				return out, ErrConflict
			}
		} else if err != nil {
			return out, err
		}
		out.Acknowledged = append(out.Acknowledged, entry.LogSeq)
	}
	// Keep at most 5000 diagnostic lines per gateway. Measurements are untouched.
	if _, err = tx.Exec(ctx, `DELETE FROM gateway_logs WHERE gateway_id=$1 AND id < COALESCE((SELECT id FROM gateway_logs WHERE gateway_id=$1 ORDER BY id DESC OFFSET 4999 LIMIT 1),0)`, b.GatewayID); err != nil {
		return out, err
	}
	if err = tx.Commit(ctx); err != nil {
		return out, err
	}
	return out, nil
}
func (s *Store) GatewayLogs(ctx context.Context, gateway string, after int64, limit int) (c.GatewayLogPage, error) {
	out := c.GatewayLogPage{Items: []c.StoredGatewayLog{}, NextCursor: strconv.FormatInt(after, 10)}
	var exists bool
	if err := s.pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM gateways WHERE id=$1)`, gateway).Scan(&exists); err != nil {
		return out, err
	}
	if !exists {
		return out, ErrNotFound
	}
	var rows pgx.Rows
	var err error
	if after == 0 {
		rows, err = s.pool.Query(ctx, `SELECT id,gateway_session_id,payload,received_at FROM (SELECT id,gateway_session_id,payload,received_at FROM gateway_logs WHERE gateway_id=$1 ORDER BY id DESC LIMIT $2) recent ORDER BY id`, gateway, limit)
	} else {
		rows, err = s.pool.Query(ctx, `SELECT id,gateway_session_id,payload,received_at FROM gateway_logs WHERE gateway_id=$1 AND id>$2 ORDER BY id LIMIT $3`, gateway, after, limit+1)
	}
	if err != nil {
		return out, err
	}
	defer rows.Close()
	for rows.Next() {
		var e c.StoredGatewayLog
		var raw []byte
		var id int64
		if err = rows.Scan(&id, &e.GatewaySessionID, &raw, &e.ReceivedAt); err != nil {
			return out, err
		}
		if len(out.Items) == limit {
			out.HasMore = true
			break
		}
		if err = json.Unmarshal(raw, &e.GatewayLog); err != nil {
			return out, err
		}
		e.ID = strconv.FormatInt(id, 10)
		e.GatewayID = gateway
		out.Items = append(out.Items, e)
		out.NextCursor = e.ID
	}
	return out, rows.Err()
}
