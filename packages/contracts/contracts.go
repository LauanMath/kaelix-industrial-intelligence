// Package contracts defines the versioned HTTP contract between gateways, API and desktop.
package contracts

import "time"

type Reading struct {
	EventSeq          int64      `json:"event_seq"`
	GatewayRxUptimeMS int64      `json:"gateway_rx_uptime_ms"`
	GatewayReceivedAt *time.Time `json:"gateway_received_at"`
	PacketVersion     int        `json:"packet_version"`
	DeviceID          string     `json:"device_id"`
	DeviceSequence    *uint32    `json:"device_sequence"`
	State             string     `json:"state"`
	DiagCode          *uint8     `json:"diag_code"`
	RMSG              *float64   `json:"rms_g"`
	TemperatureC      *float64   `json:"temperature_c"`
	RSSIDBm           *float64   `json:"rssi_dbm"`
	SNRDB             *float64   `json:"snr_db"`
	PacketCRC16       *uint16    `json:"packet_crc16"`
	// Packet v3 fields. They are omitted when empty, so a v2 reading stored before
	// they existed still compares equal when the gateway retransmits it.
	BootCount      *uint32    `json:"boot_count,omitempty"`
	CommandSeq     *uint16    `json:"command_seq,omitempty"`
	RMSXG          *float64   `json:"rms_x_g,omitempty"`
	RMSYG          *float64   `json:"rms_y_g,omitempty"`
	Kurtosis       *float64   `json:"kurtosis,omitempty"`
	CrestFactor    *float64   `json:"crest_factor,omitempty"`
	DominantFreqHz *float64   `json:"dominant_freq_hz,omitempty"`
	SampleRateHz   *float64   `json:"sample_rate_hz,omitempty"`
	ConfigVersion  *uint16    `json:"config_version,omitempty"`
	ResetReason    *uint8     `json:"reset_reason,omitempty"`
	Flags          *uint8     `json:"flags,omitempty"`
	DeviceTime     *time.Time `json:"device_time,omitempty"`
	UptimeS        *uint32    `json:"uptime_s,omitempty"`
	WaveFragments  *uint8     `json:"wave_fragments,omitempty"`
	WaveSamples    *uint16    `json:"wave_samples,omitempty"`
	Channels       *uint8     `json:"channels,omitempty"`
	TxPowerDBm     *int8      `json:"tx_power_dbm,omitempty"`
}

// Bits of Reading.Flags.
const (
	FlagTimeSynced     = 1 << 0
	FlagWaveFollows    = 1 << 1
	FlagAutoResponse   = 1 << 2
	FlagConfigRejected = 1 << 3
	FlagNoMeasurement  = 1 << 4
	FlagSaturated      = 1 << 5
)

type ReadingBatch struct {
	SchemaVersion    int       `json:"schema_version"`
	GatewayID        string    `json:"gateway_id"`
	GatewaySessionID string    `json:"gateway_session_id"`
	PreviousSession  bool      `json:"previous_session,omitempty"`
	UploadUptimeMS   int64     `json:"upload_uptime_ms"`
	Readings         []Reading `json:"readings"`
}
type Acknowledgement struct {
	EventSeq int64  `json:"event_seq"`
	Status   string `json:"status"`
}
type IngestResult struct {
	SchemaVersion    int               `json:"schema_version"`
	GatewayID        string            `json:"gateway_id"`
	GatewaySessionID string            `json:"gateway_session_id"`
	ServerReceivedAt time.Time         `json:"server_received_at"`
	Acknowledged     []Acknowledgement `json:"acknowledged"`
}
type Measurement struct {
	ID               int64  `json:"id"`
	GatewayID        string `json:"gateway_id"`
	GatewaySessionID string `json:"gateway_session_id"`
	Reading
	IngestedAt          time.Time  `json:"ingested_at"`
	EstimatedReceivedAt *time.Time `json:"estimated_received_at"`
	QueueDelayMS        int64      `json:"queue_delay_ms"`
}
type Device struct {
	ID        string       `json:"id"`
	Profile   string       `json:"profile"`
	Notes     string       `json:"notes"`
	MotorID   *string      `json:"motor_id"`
	CreatedAt time.Time    `json:"created_at"`
	Latest    *Measurement `json:"latest"`
}
type Motor struct {
	ID                      string       `json:"id"`
	Name                    string       `json:"name"`
	Location                string       `json:"location"`
	DeviceID                *string      `json:"device_id"`
	ExpectedIntervalSeconds int          `json:"expected_interval_seconds"`
	CreatedAt               time.Time    `json:"created_at"`
	Latest                  *Measurement `json:"latest"`
}
type MotorInput struct {
	ID                      string  `json:"id"`
	Name                    string  `json:"name"`
	Location                string  `json:"location"`
	DeviceID                *string `json:"device_id"`
	ExpectedIntervalSeconds int     `json:"expected_interval_seconds"`
}
type DeviceInput struct {
	ID      string `json:"id"`
	Profile string `json:"profile"`
	Notes   string `json:"notes"`
}
type GatewayInput struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Location string `json:"location"`
}
type HeartbeatHealth struct {
	RadioInitialized bool   `json:"radio_initialized"`
	WifiConnected    bool   `json:"wifi_connected"`
	NTPSynchronized  bool   `json:"ntp_synchronized"`
	QueueSize        int    `json:"queue_size"`
	QueueCapacity    int    `json:"queue_capacity"`
	FreeHeapBytes    uint32 `json:"free_heap_bytes"`
	WifiRSSIDBm      *int64 `json:"wifi_rssi_dbm"`
	// Optional, omitted when absent so earlier snapshots keep comparing equal.
	SpoolUsedBytes     *int64  `json:"spool_used_bytes,omitempty"`
	SpoolCapacityBytes *int64  `json:"spool_capacity_bytes,omitempty"`
	SSID               *string `json:"ssid,omitempty"`
	IP                 *string `json:"ip,omitempty"`
	ResetReason        *string `json:"reset_reason,omitempty"`
}
type HeartbeatCounters struct {
	LoRaValid            int64 `json:"lora_valid"`
	LoRaRejected         int64 `json:"lora_rejected"`
	SPIErrors            int64 `json:"spi_errors"`
	HTTPFailedAttempts   int64 `json:"http_failed_attempts"`
	ReadingsAcknowledged int64 `json:"readings_acknowledged"`
	QueueDropped         int64 `json:"queue_dropped"`
	// Optional and non-decreasing within a session, like the fields above.
	SpoolDropped          *int64 `json:"spool_dropped,omitempty"`
	WaveformsAcknowledged *int64 `json:"waveforms_acknowledged,omitempty"`
}
type Heartbeat struct {
	SchemaVersion    int               `json:"schema_version"`
	GatewayID        string            `json:"gateway_id"`
	GatewaySessionID string            `json:"gateway_session_id"`
	HeartbeatSeq     int64             `json:"heartbeat_seq"`
	UptimeMS         int64             `json:"uptime_ms"`
	SentAt           *time.Time        `json:"sent_at"`
	FirmwareVersion  string            `json:"firmware_version"`
	Health           HeartbeatHealth   `json:"health"`
	Counters         HeartbeatCounters `json:"counters"`
}
type HeartbeatResult struct {
	SchemaVersion    int       `json:"schema_version"`
	GatewayID        string    `json:"gateway_id"`
	GatewaySessionID string    `json:"gateway_session_id"`
	HeartbeatSeq     int64     `json:"heartbeat_seq"`
	ServerReceivedAt time.Time `json:"server_received_at"`
}
type ErrorDetail struct {
	Path   string `json:"path"`
	Reason string `json:"reason"`
	// Minimum is the smallest accepted value when the field is below a calculated limit.
	Minimum *int `json:"minimum,omitempty"`
}
type APIError struct {
	SchemaVersion int       `json:"schema_version"`
	Error         ErrorBody `json:"error"`
}
type ErrorBody struct {
	Code      string        `json:"code"`
	Message   string        `json:"message"`
	Retryable bool          `json:"retryable"`
	Details   []ErrorDetail `json:"details"`
}
type Gateway struct {
	GatewayInput
	CreatedAt       time.Time  `json:"created_at"`
	LastIngestedAt  *time.Time `json:"last_ingested_at"`
	LastHeartbeatAt *time.Time `json:"last_heartbeat_at"`
	Heartbeat       *Heartbeat `json:"heartbeat"`
	ActiveSessionID *string    `json:"active_session_id"`
	Activity        string     `json:"activity"`
}
type HistoryPage struct {
	Items      []Measurement `json:"items"`
	NextCursor string        `json:"next_cursor,omitempty"`
}
