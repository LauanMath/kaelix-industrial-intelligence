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
		if r.PacketVersion != 2 && r.PacketVersion != 3 {
			return bad("packet_version", "must be 2 or 3")
		}
		if field := firstV3Field(r); r.PacketVersion == 2 && field != "" {
			return bad(field, "must be null or absent in packet version 2")
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
		for name, value := range map[string]*float64{"rms_g": r.RMSG, "temperature_c": r.TemperatureC, "rssi_dbm": r.RSSIDBm, "snr_db": r.SNRDB, "rms_x_g": r.RMSXG, "rms_y_g": r.RMSYG, "kurtosis": r.Kurtosis, "crest_factor": r.CrestFactor, "dominant_freq_hz": r.DominantFreqHz, "sample_rate_hz": r.SampleRateHz} {
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
		if (r.RMSXG != nil && *r.RMSXG < 0) || (r.RMSYG != nil && *r.RMSYG < 0) {
			return bad("rms_x_g", "RMS cannot be negative")
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
		if r.DeviceTime != nil {
			_, offset := r.DeviceTime.Zone()
			if offset != 0 {
				return bad("device_time", "must be UTC")
			}
		}
	}
	return nil
}

// firstV3Field names the first field a packet v2 cannot carry, if any.
func firstV3Field(r c.Reading) string {
	for _, field := range []struct {
		name string
		set  bool
	}{{"boot_count", r.BootCount != nil}, {"command_seq", r.CommandSeq != nil}, {"rms_x_g", r.RMSXG != nil}, {"rms_y_g", r.RMSYG != nil}, {"kurtosis", r.Kurtosis != nil}, {"crest_factor", r.CrestFactor != nil}, {"dominant_freq_hz", r.DominantFreqHz != nil}, {"sample_rate_hz", r.SampleRateHz != nil}, {"config_version", r.ConfigVersion != nil}, {"reset_reason", r.ResetReason != nil}, {"flags", r.Flags != nil}, {"device_time", r.DeviceTime != nil}, {"uptime_s", r.UptimeS != nil}, {"wave_fragments", r.WaveFragments != nil}, {"wave_samples", r.WaveSamples != nil}, {"channels", r.Channels != nil}, {"tx_power_dbm", r.TxPowerDBm != nil}} {
		if field.set {
			return field.name
		}
	}
	return ""
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
	for name, value := range map[string]*int64{"counters.spool_dropped": h.Counters.SpoolDropped, "counters.waveforms_acknowledged": h.Counters.WaveformsAcknowledged, "health.spool_used_bytes": h.Health.SpoolUsedBytes, "health.spool_capacity_bytes": h.Health.SpoolCapacityBytes} {
		if value != nil && (*value < 0 || *value > maxSafeInteger) {
			return invalid(name, "outside the safe integer range")
		}
	}
	if used, capacity := h.Health.SpoolUsedBytes, h.Health.SpoolCapacityBytes; used != nil && capacity != nil && *used > *capacity {
		return invalid("health.spool_used_bytes", "exceeds the capacity")
	}
	for name, value := range map[string]*string{"health.ssid": h.Health.SSID, "health.ip": h.Health.IP, "health.reset_reason": h.Health.ResetReason} {
		if value != nil && !shortText(*value, 64) {
			return invalid(name, "must contain up to 64 characters")
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
func shortText(s string, n int) bool {
	return utf8.ValidString(s) && utf8.RuneCountInString(s) <= n && !strings.ContainsRune(s, 0)
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
