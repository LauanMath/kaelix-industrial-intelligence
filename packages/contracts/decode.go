package contracts

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"
)

// Required sensor values must be sent explicitly, including null when unavailable.
func (r *Reading) UnmarshalJSON(data []byte) error {
	type plain Reading
	var value plain
	if err := decodeRequired(data, &value, []string{"event_seq", "gateway_rx_uptime_ms", "gateway_received_at", "packet_version", "device_id", "device_sequence", "state", "diag_code", "rms_g", "temperature_c", "rssi_dbm", "snr_db", "packet_crc16"}, map[string]bool{"gateway_received_at": true, "rms_g": true, "temperature_c": true}); err != nil {
		return err
	}
	*r = Reading(value)
	return nil
}
func (b *ReadingBatch) UnmarshalJSON(data []byte) error {
	type plain ReadingBatch
	var value plain
	if err := decodeRequired(data, &value, []string{"schema_version", "gateway_id", "gateway_session_id", "upload_uptime_ms", "readings"}, nil); err != nil {
		return err
	}
	*b = ReadingBatch(value)
	return nil
}
func decodeRequired(data []byte, target any, required []string, nullable map[string]bool) error {
	if err := ValidateJSON(data); err != nil {
		return err
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(data, &fields); err != nil {
		return err
	}
	for _, key := range required {
		value, ok := fields[key]
		if !ok || (!nullable[key] && bytes.Equal(bytes.TrimSpace(value), []byte("null"))) {
			return fmt.Errorf("%s is required", key)
		}
	}
	for _, key := range []string{"gateway_received_at", "sent_at"} {
		if raw, ok := fields[key]; ok && string(raw) != "null" {
			var stamp string
			if json.Unmarshal(raw, &stamp) != nil || !strings.HasSuffix(stamp, "Z") {
				return fmt.Errorf("%s must be UTC RFC3339 ending in Z or null", key)
			}
		}
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	return decoder.Decode(target)
}

// Override the embedded Reading decoder so measurement metadata is also decoded.
func (m *Measurement) UnmarshalJSON(data []byte) error {
	type plainReading Reading
	var value struct {
		plainReading
		ID                  int64      `json:"id"`
		GatewayID           string     `json:"gateway_id"`
		GatewaySessionID    string     `json:"gateway_session_id"`
		IngestedAt          time.Time  `json:"ingested_at"`
		EstimatedReceivedAt *time.Time `json:"estimated_received_at"`
		QueueDelayMS        int64      `json:"queue_delay_ms"`
	}
	if err := json.Unmarshal(data, &value); err != nil {
		return err
	}
	*m = Measurement{ID: value.ID, GatewayID: value.GatewayID, GatewaySessionID: value.GatewaySessionID, Reading: Reading(value.plainReading), IngestedAt: value.IngestedAt, EstimatedReceivedAt: value.EstimatedReceivedAt, QueueDelayMS: value.QueueDelayMS}
	return nil
}

func (h *Heartbeat) UnmarshalJSON(data []byte) error {
	type plain Heartbeat
	var value plain
	if err := decodeRequired(data, &value, []string{"schema_version", "gateway_id", "gateway_session_id", "heartbeat_seq", "uptime_ms", "sent_at", "firmware_version", "health", "counters"}, map[string]bool{"sent_at": true}); err != nil {
		return err
	}
	*h = Heartbeat(value)
	return nil
}
func (h *HeartbeatHealth) UnmarshalJSON(data []byte) error {
	type plain HeartbeatHealth
	var value plain
	if err := decodeRequired(data, &value, []string{"radio_initialized", "wifi_connected", "ntp_synchronized", "queue_size", "queue_capacity", "free_heap_bytes", "wifi_rssi_dbm"}, map[string]bool{"wifi_rssi_dbm": true}); err != nil {
		return err
	}
	*h = HeartbeatHealth(value)
	return nil
}
func (h *HeartbeatCounters) UnmarshalJSON(data []byte) error {
	type plain HeartbeatCounters
	var value plain
	if err := decodeRequired(data, &value, []string{"lora_valid", "lora_rejected", "spi_errors", "http_failed_attempts", "readings_acknowledged", "queue_dropped"}, nil); err != nil {
		return err
	}
	*h = HeartbeatCounters(value)
	return nil
}

// ValidateJSON rejects duplicate keys at every depth and trailing JSON values.
func ValidateJSON(data []byte) error {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()
	var visit func(int) error
	visit = func(depth int) error {
		if depth > 64 {
			return fmt.Errorf("JSON nesting is too deep")
		}
		token, err := decoder.Token()
		if err != nil {
			return err
		}
		delimiter, ok := token.(json.Delim)
		if !ok {
			return nil
		}
		switch delimiter {
		case '{':
			seen := map[string]bool{}
			for decoder.More() {
				key, err := decoder.Token()
				if err != nil {
					return err
				}
				name, ok := key.(string)
				if !ok || seen[name] {
					return fmt.Errorf("duplicate or invalid JSON key")
				}
				seen[name] = true
				if err = visit(depth + 1); err != nil {
					return err
				}
			}
		case '[':
			for decoder.More() {
				if err := visit(depth + 1); err != nil {
					return err
				}
			}
		default:
			return fmt.Errorf("invalid JSON delimiter")
		}
		_, err = decoder.Token()
		return err
	}
	if err := visit(0); err != nil {
		return err
	}
	if _, err := decoder.Token(); err != io.EOF {
		return fmt.Errorf("trailing JSON data")
	}
	return nil
}
