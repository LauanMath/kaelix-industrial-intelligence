package contracts

import (
	"encoding/json"
	"time"
)

// Logs use a separate transport: diagnostic failures never acknowledge readings.
type GatewayLog struct {
	LogSeq    uint32 `json:"log_seq"`
	UptimeMS  int64  `json:"uptime_ms"`
	Level     string `json:"level"`
	Event     string `json:"event"`
	Message   string `json:"message"`
	Truncated bool   `json:"truncated"`
}
type GatewayLogBatch struct {
	SchemaVersion    int          `json:"schema_version"`
	GatewayID        string       `json:"gateway_id"`
	GatewaySessionID string       `json:"gateway_session_id"`
	Logs             []GatewayLog `json:"logs"`
}
type GatewayLogAck struct {
	SchemaVersion    int       `json:"schema_version"`
	GatewayID        string    `json:"gateway_id"`
	GatewaySessionID string    `json:"gateway_session_id"`
	ServerReceivedAt time.Time `json:"server_received_at"`
	Acknowledged     []uint32  `json:"acknowledged"`
}
type StoredGatewayLog struct {
	ID               string `json:"id"` // decimal cursor, safe across Go/TypeScript bigint sizes
	GatewayID        string `json:"gateway_id"`
	GatewaySessionID string `json:"gateway_session_id"`
	GatewayLog
	ReceivedAt time.Time `json:"received_at"`
}
type GatewayLogPage struct {
	Items      []StoredGatewayLog `json:"items"`
	NextCursor string             `json:"next_cursor"`
	HasMore    bool               `json:"has_more"`
}

func (b *GatewayLogBatch) UnmarshalJSON(data []byte) error {
	type plain GatewayLogBatch
	var out plain
	if err := decodeRequired(data, &out, []string{"schema_version", "gateway_id", "gateway_session_id", "logs"}, nil); err != nil {
		return err
	}
	*b = GatewayLogBatch(out)
	return nil
}
func (e *GatewayLog) UnmarshalJSON(data []byte) error {
	type plain GatewayLog
	var out plain
	if err := decodeRequired(data, &out, []string{"log_seq", "uptime_ms", "level", "event", "message", "truncated"}, nil); err != nil {
		return err
	}
	*e = GatewayLog(out)
	return nil
}
func (e *StoredGatewayLog) UnmarshalJSON(data []byte) error {
	type plainLog GatewayLog
	var out struct {
		plainLog
		ID               string    `json:"id"`
		GatewayID        string    `json:"gateway_id"`
		GatewaySessionID string    `json:"gateway_session_id"`
		ReceivedAt       time.Time `json:"received_at"`
	}
	if err := json.Unmarshal(data, &out); err != nil {
		return err
	}
	*e = StoredGatewayLog{ID: out.ID, GatewayID: out.GatewayID, GatewaySessionID: out.GatewaySessionID, GatewayLog: GatewayLog(out.plainLog), ReceivedAt: out.ReceivedAt}
	return nil
}
