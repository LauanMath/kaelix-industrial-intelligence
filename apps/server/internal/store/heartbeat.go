package store

import (
	"context"
	"encoding/json"
	"time"

	c "kaelix/contracts"
)

func (s *Store) SaveHeartbeat(ctx context.Context, h c.Heartbeat) (c.HeartbeatResult, error) {
	now := time.Now().UTC()
	out := c.HeartbeatResult{SchemaVersion: 1, GatewayID: h.GatewayID, GatewaySessionID: h.GatewaySessionID, HeartbeatSeq: h.HeartbeatSeq, ServerReceivedAt: now}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return out, err
	}
	defer tx.Rollback(ctx)
	// Serialize state changes for this gateway; different gateways remain independent.
	var active *string
	if err = tx.QueryRow(ctx, `SELECT active_session_id FROM gateways WHERE id=$1 FOR UPDATE`, h.GatewayID).Scan(&active); err != nil {
		return out, dbError(err)
	}
	if _, err = tx.Exec(ctx, `INSERT INTO gateway_sessions(gateway_id,session_id,first_seen_at) VALUES($1,$2,$3) ON CONFLICT DO NOTHING`, h.GatewayID, h.GatewaySessionID, now); err != nil {
		return out, err
	}
	payload, err := json.Marshal(h)
	if err != nil {
		return out, err
	}
	var prior []byte
	var seq int64
	var retired bool
	var firstSeen time.Time
	if err = tx.QueryRow(ctx, `SELECT latest_heartbeat,last_heartbeat_seq,retired,first_seen_at FROM gateway_sessions WHERE gateway_id=$1 AND session_id=$2`, h.GatewayID, h.GatewaySessionID).Scan(&prior, &seq, &retired, &firstSeen); err != nil {
		return out, err
	}
	// Record immutable snapshots. Retransmission never renews liveness.
	tag, err := tx.Exec(ctx, `INSERT INTO gateway_heartbeats(gateway_id,session_id,heartbeat_seq,payload,received_at) VALUES($1,$2,$3,$4,$5) ON CONFLICT DO NOTHING`, h.GatewayID, h.GatewaySessionID, h.HeartbeatSeq, payload, now)
	if err != nil {
		return out, err
	}
	if tag.RowsAffected() == 0 {
		var same bool
		if err = tx.QueryRow(ctx, `SELECT payload=$4::jsonb FROM gateway_heartbeats WHERE gateway_id=$1 AND session_id=$2 AND heartbeat_seq=$3`, h.GatewayID, h.GatewaySessionID, h.HeartbeatSeq, payload).Scan(&same); err != nil {
			return out, err
		}
		if !same {
			return out, ErrConflict
		}
		return out, tx.Commit(ctx)
	}
	if h.HeartbeatSeq <= seq {
		return out, tx.Commit(ctx)
	}
	if len(prior) > 0 {
		var old c.Heartbeat
		if err = json.Unmarshal(prior, &old); err != nil {
			return out, err
		}
		if h.UptimeMS < old.UptimeMS || countersDecrease(old.Counters, h.Counters) {
			return out, ErrInvalidHeartbeat
		}
	}
	if _, err = tx.Exec(ctx, `UPDATE gateway_sessions SET latest_heartbeat=$3,last_heartbeat_seq=$4,last_heartbeat_at=$5 WHERE gateway_id=$1 AND session_id=$2`, h.GatewayID, h.GatewaySessionID, payload, h.HeartbeatSeq, now); err != nil {
		return out, err
	}
	activate := active == nil || *active == h.GatewaySessionID
	if !activate && !retired {
		var activeFirstSeen time.Time
		if err = tx.QueryRow(ctx, `SELECT first_seen_at FROM gateway_sessions WHERE gateway_id=$1 AND session_id=$2`, h.GatewayID, *active).Scan(&activeFirstSeen); err != nil {
			return out, err
		}
		activate = !firstSeen.Before(activeFirstSeen)
	}
	if activate && !retired {
		if active != nil && *active != h.GatewaySessionID {
			if _, err = tx.Exec(ctx, `UPDATE gateway_sessions SET retired=true WHERE gateway_id=$1 AND session_id=$2`, h.GatewayID, *active); err != nil {
				return out, err
			}
		}
		if _, err = tx.Exec(ctx, `UPDATE gateways SET active_session_id=$2,heartbeat=$3,last_heartbeat_at=$4 WHERE id=$1`, h.GatewayID, h.GatewaySessionID, payload, now); err != nil {
			return out, err
		}
	}
	return out, tx.Commit(ctx)
}
func countersDecrease(old, new c.HeartbeatCounters) bool {
	return new.LoRaValid < old.LoRaValid || new.LoRaRejected < old.LoRaRejected || new.SPIErrors < old.SPIErrors || new.HTTPFailedAttempts < old.HTTPFailedAttempts || new.ReadingsAcknowledged < old.ReadingsAcknowledged || new.QueueDropped < old.QueueDropped
}
