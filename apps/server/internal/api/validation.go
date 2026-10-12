package api

import (
	"fmt"
	"math"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"

	c "kaelix/contracts"
)

var identifier = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_.-]{0,63}$`)
var deviceID = regexp.MustCompile(`^[0-9A-F]{8}$`)

func boundedText(s string, n int) bool { return len(s) <= n && !strings.ContainsRune(s, 0) }

var uuidID = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)

const maxSafeInteger int64 = 9007199254740991
const maxSequence int64 = 4294967295

type validationError struct{ Path, Reason string }

func (e *validationError) Error() string { return e.Path + ": " + e.Reason }
func invalid(path, reason string) error  { return &validationError{Path: path, Reason: reason} }
func validateBatch(b c.ReadingBatch, _ time.Time) error {
	if b.SchemaVersion != 1 {
		return invalid("schema_version", "must be 1")
	}
	if !uuidID.MatchString(b.GatewayID) {
		return invalid("gateway_id", "must be a lowercase UUID")
	}
	if !uuidID.MatchString(b.GatewaySessionID) {
		return invalid("gateway_session_id", "must be a lowercase UUID")
	}
	if b.UploadUptimeMS < 0 || b.UploadUptimeMS > maxSafeInteger {
		return invalid("upload_uptime_ms", "outside the safe integer range")
	}
	if len(b.Readings) < 1 || len(b.Readings) > 8 {
		return invalid("readings", "must contain 1 to 8 events")
	}
	seen := map[int64]bool{}
	for i, r := range b.Readings {
		bad := func(field, message string) error { return invalid(fmt.Sprintf("readings[%d].%s", i, field), message) }
		if r.EventSeq < 1 || r.EventSeq > maxSequence || seen[r.EventSeq] {
			return bad("event_seq", "must be uint32, positive and unique within the batch")
		}
		seen[r.EventSeq] = true
		if r.GatewayRxUptimeMS < 0 || r.GatewayRxUptimeMS > b.UploadUptimeMS {
			return bad("gateway_rx_uptime_ms", "must not exceed upload_uptime_ms")
		}
		if r.PacketVersion != 2 {
			return bad("packet_version", "must be 2")
		}
		if !deviceID.MatchString(r.DeviceID) {
			return bad("device_id", "must be eight uppercase hexadecimal characters")
		}
		if r.DeviceSequence == nil || r.DiagCode == nil || r.PacketCRC16 == nil {
			return bad("device_sequence", "sequence, diagnosis and CRC are required")
		}
		if r.State != "NORMAL" && r.State != "ANOMALOUS" && r.State != "UNKNOWN" {
			return bad("state", "unsupported state")
		}
		for name, value := range map[string]*float64{"rms_g": r.RMSG, "temperature_c": r.TemperatureC, "rssi_dbm": r.RSSIDBm, "snr_db": r.SNRDB} {
			if value != nil && (math.IsNaN(*value) || math.IsInf(*value, 0)) {
				return bad(name, "must be finite")
			}
		}
		if r.RMSG == nil && r.State != "UNKNOWN" {
			return bad("rms_g", "null is only allowed with UNKNOWN")
		}
		if r.RMSG != nil && *r.RMSG < 0 {
			return bad("rms_g", "cannot be negative")
		}
		if r.RSSIDBm == nil || r.SNRDB == nil {
			return bad("rssi_dbm", "RSSI and SNR must be finite numbers")
		}
		if r.GatewayReceivedAt != nil {
			_, offset := r.GatewayReceivedAt.Zone()
			if offset != 0 {
				return bad("gateway_received_at", "must be UTC")
			}
		}
	}
	return nil
}
func validateHeartbeat(h c.Heartbeat) error {
	if h.SchemaVersion != 1 {
		return invalid("schema_version", "must be 1")
	}
	if !uuidID.MatchString(h.GatewayID) || !uuidID.MatchString(h.GatewaySessionID) {
		return invalid("gateway_id", "gateway and session must be lowercase UUIDs")
	}
	if h.HeartbeatSeq < 1 || h.HeartbeatSeq > maxSequence {
		return invalid("heartbeat_seq", "must be a positive uint32")
	}
	if h.UptimeMS < 0 || h.UptimeMS > maxSafeInteger {
		return invalid("uptime_ms", "outside the safe integer range")
	}
	if !utf8.ValidString(h.FirmwareVersion) || utf8.RuneCountInString(h.FirmwareVersion) < 1 || utf8.RuneCountInString(h.FirmwareVersion) > 64 || strings.ContainsRune(h.FirmwareVersion, 0) {
		return invalid("firmware_version", "must contain 1 to 64 characters")
	}
	if h.Health.QueueSize < 0 || h.Health.QueueCapacity < 1 || h.Health.QueueCapacity > 65535 || h.Health.QueueSize > h.Health.QueueCapacity {
		return invalid("health.queue_size", "invalid queue size or capacity")
	}
	for name, value := range map[string]int64{"lora_valid": h.Counters.LoRaValid, "lora_rejected": h.Counters.LoRaRejected, "spi_errors": h.Counters.SPIErrors, "http_failed_attempts": h.Counters.HTTPFailedAttempts, "readings_acknowledged": h.Counters.ReadingsAcknowledged, "queue_dropped": h.Counters.QueueDropped} {
		if value < 0 || value > maxSafeInteger {
			return invalid("counters."+name, "outside the safe integer range")
		}
	}
	if h.SentAt != nil {
		_, offset := h.SentAt.Zone()
		if offset != 0 {
			return invalid("sent_at", "must be UTC")
		}
	}
	return nil
}
func validateMotor(m *c.MotorInput) error {
	m.Name = strings.TrimSpace(m.Name)
	m.Location = strings.TrimSpace(m.Location)
	if !identifier.MatchString(m.ID) || m.Name == "" || !boundedText(m.Name, 160) || !boundedText(m.Location, 256) {
		return fmt.Errorf("invalid motor identifier, name or location")
	}
	if m.DeviceID != nil && !deviceID.MatchString(*m.DeviceID) {
		return fmt.Errorf("invalid device_id")
	}
	if m.ExpectedIntervalSeconds < 1 || m.ExpectedIntervalSeconds > 86400 {
		return fmt.Errorf("expected_interval_seconds must be between 1 and 86400")
	}
	return nil
}
func validateDevice(d c.DeviceInput) error {
	if !deviceID.MatchString(d.ID) || !boundedText(d.Profile, 160) || !boundedText(d.Notes, 2000) {
		return fmt.Errorf("invalid device identifier, profile or notes")
	}
	return nil
}
func validateGateway(g *c.GatewayInput) error {
	g.Name = strings.TrimSpace(g.Name)
	g.Location = strings.TrimSpace(g.Location)
	if (g.ID != "" && !uuidID.MatchString(g.ID)) || g.Name == "" || !boundedText(g.Name, 160) || !boundedText(g.Location, 256) {
		return fmt.Errorf("invalid gateway identifier, name or location")
	}
	return nil
}
