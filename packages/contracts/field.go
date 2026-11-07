package contracts

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"
)

// Device configuration state.
const (
	StatePending  = "pending"
	StateRadiated = "radiated"
	StateApplied  = "applied"
)

// Point commands of a transmitter.
const (
	CommandReboot       = "reboot"
	CommandFactoryReset = "factory_reset"
	CommandCaptureNow   = "capture_now"
)

// ValidCommandKind tells whether the transmitter knows this command.
func ValidCommandKind(kind string) bool {
	return kind == CommandReboot || kind == CommandFactoryReset || kind == CommandCaptureNow
}

// DeviceConfigState is GET /devices/{id}/config. Config and State are null until a
// configuration is saved: the transmitter runs what it has stored.
type DeviceConfigState struct {
	DeviceID           string        `json:"device_id"`
	Config             *DeviceConfig `json:"config"`
	State              *string       `json:"state"`
	RadiatedVersion    *int          `json:"radiated_version"`
	ReportedVersion    *int          `json:"reported_version"`
	ReportedCommandSeq *int          `json:"reported_command_seq"`
	UpdatedAt          *time.Time    `json:"updated_at"`
	RadiatedAt         *time.Time    `json:"radiated_at"`
	ReportedAt         *time.Time    `json:"reported_at"`
	AppliedAt          *time.Time    `json:"applied_at"`
	Defaults           DeviceConfig  `json:"defaults"`
}

// DeviceCommandInput is POST /devices/{id}/commands.
type DeviceCommandInput struct {
	Kind string `json:"kind"`
}

// DeviceCommand is one command with its state.
type DeviceCommand struct {
	DeviceID   string     `json:"device_id"`
	Seq        int        `json:"seq"`
	Kind       string     `json:"kind"`
	State      string     `json:"state"`
	CreatedAt  time.Time  `json:"created_at"`
	RadiatedAt *time.Time `json:"radiated_at"`
	AppliedAt  *time.Time `json:"applied_at"`
}

// DeviceCommandList is GET /devices/{id}/commands.
type DeviceCommandList struct {
	Items []DeviceCommand `json:"items"`
}

// WifiCredential is a network in a PUT. An absent password keeps the stored one.
type WifiCredential struct {
	SSID     string  `json:"ssid"`
	Password *string `json:"password,omitempty"`
}

// GatewayConfigInput is PUT /gateways/{id}/config. Version is assigned by the API and
// ignored here, so a client can send back what it read.
type GatewayConfigInput struct {
	Version            int              `json:"version,omitempty"`
	Wifi               []WifiCredential `json:"wifi"`
	HeartbeatIntervalS int              `json:"heartbeat_interval_s"`
	SyncIntervalS      int              `json:"sync_interval_s"`
	APWhenOffline      bool             `json:"ap_when_offline"`
	TxPowerDBm         int              `json:"tx_power_dbm"`
}

// DefaultGatewayConfig is the decode base: an absent field takes its default.
func DefaultGatewayConfig() GatewayConfigInput {
	return GatewayConfigInput{Wifi: []WifiCredential{}, HeartbeatIntervalS: 30, SyncIntervalS: 5, APWhenOffline: true, TxPowerDBm: 17}
}

// WifiStatus is what a user can read about a network: never the password.
type WifiStatus struct {
	SSID        string `json:"ssid"`
	HasPassword bool   `json:"has_password"`
}

// GatewayConfigView is the desired gateway configuration without secrets.
type GatewayConfigView struct {
	Version            int          `json:"version"`
	Wifi               []WifiStatus `json:"wifi"`
	HeartbeatIntervalS int          `json:"heartbeat_interval_s"`
	SyncIntervalS      int          `json:"sync_interval_s"`
	APWhenOffline      bool         `json:"ap_when_offline"`
	TxPowerDBm         int          `json:"tx_power_dbm"`
}

// GatewayConfigState is GET /gateways/{id}/config.
type GatewayConfigState struct {
	GatewayID       string             `json:"gateway_id"`
	Config          *GatewayConfigView `json:"config"`
	State           *string            `json:"state"`
	ReportedVersion *int               `json:"reported_version"`
	UpdatedAt       *time.Time         `json:"updated_at"`
	ReportedAt      *time.Time         `json:"reported_at"`
	AppliedAt       *time.Time         `json:"applied_at"`
	LastSyncAt      *time.Time         `json:"last_sync_at"`
	Defaults        GatewayConfigView  `json:"defaults"`
}

// WifiSecret is a network as delivered to the authenticated gateway.
type WifiSecret struct {
	SSID     string `json:"ssid"`
	Password string `json:"password"`
}

// GatewayConfigDelivery is the configuration carried by a sync response.
type GatewayConfigDelivery struct {
	Version            int          `json:"version"`
	Wifi               []WifiSecret `json:"wifi"`
	HeartbeatIntervalS int          `json:"heartbeat_interval_s"`
	SyncIntervalS      int          `json:"sync_interval_s"`
	APWhenOffline      bool         `json:"ap_when_offline"`
	TxPowerDBm         int          `json:"tx_power_dbm"`
}

func validSSID(ssid string) bool {
	return ssid != "" && len(ssid) <= 32 && utf8.ValidString(ssid) && !strings.ContainsRune(ssid, 0)
}

func validWifiPassword(password string) bool {
	if password == "" {
		return true
	}
	if !utf8.ValidString(password) || strings.ContainsRune(password, 0) {
		return false
	}
	if len(password) == 64 {
		for _, r := range password {
			if !strings.ContainsRune("0123456789abcdefABCDEF", r) {
				return false
			}
		}
		return true
	}
	return len(password) >= 8 && len(password) <= 63
}

// ValidateGatewayConfig checks ranges without ever echoing a password.
func ValidateGatewayConfig(cfg GatewayConfigInput) []ErrorDetail {
	issues := []ErrorDetail{}
	add := func(path, reason string) { issues = append(issues, ErrorDetail{Path: path, Reason: reason}) }
	if len(cfg.Wifi) > 5 {
		add("wifi", "O gateway aceita até 5 redes Wi-Fi")
	}
	seen := map[string]bool{}
	for i, network := range cfg.Wifi {
		path := fmt.Sprintf("wifi[%d]", i)
		if !validSSID(network.SSID) {
			add(path+".ssid", "O nome da rede deve ter de 1 a 32 bytes")
		} else if seen[network.SSID] {
			add(path+".ssid", "Esta rede já está na lista")
		}
		seen[network.SSID] = true
		if network.Password != nil && !validWifiPassword(*network.Password) {
			add(path+".password", "A senha deve ter de 8 a 63 caracteres, 64 hexadecimais, ou ficar vazia em rede aberta")
		}
	}
	if cfg.HeartbeatIntervalS < 10 || cfg.HeartbeatIntervalS > 300 {
		add("heartbeat_interval_s", "O intervalo de heartbeat deve estar entre 10 e 300 s")
	}
	if cfg.SyncIntervalS < 2 || cfg.SyncIntervalS > 60 {
		add("sync_interval_s", "O intervalo de sincronização deve estar entre 2 e 60 s")
	}
	if cfg.TxPowerDBm < 2 || cfg.TxPowerDBm > 17 {
		add("tx_power_dbm", "A potência da resposta deve estar entre 2 e 17 dBm")
	}
	if len(cfg.Wifi) == 0 && !cfg.APWhenOffline {
		add("ap_when_offline", "Sem redes Wi-Fi, a rede própria precisa ficar ativa, ou o gateway fica inacessível")
	}
	return issues
}

// WaveformUpload is POST /ingest/waveforms.
type WaveformUpload struct {
	SchemaVersion     int        `json:"schema_version"`
	GatewayID         string     `json:"gateway_id"`
	GatewaySessionID  string     `json:"gateway_session_id"`
	PreviousSession   bool       `json:"previous_session,omitempty"`
	UploadUptimeMS    int64      `json:"upload_uptime_ms"`
	EventSeq          int64      `json:"event_seq"`
	GatewayRxUptimeMS int64      `json:"gateway_rx_uptime_ms"`
	GatewayReceivedAt *time.Time `json:"gateway_received_at"`
	DeviceID          string     `json:"device_id"`
	DeviceSequence    *uint32    `json:"device_sequence"`
	BootCount         *uint32    `json:"boot_count"`
	Channels          int        `json:"channels"`
	WindowSamples     int        `json:"window_samples"`
	SampleRateHz      *float64   `json:"sample_rate_hz"`
	AccelRange        int        `json:"accel_range"`
	GyroRange         int        `json:"gyro_range"`
	DLPF              int        `json:"dlpf"`
	BurstProfile      int        `json:"burst_profile"`
	FragmentTotal     int        `json:"fragment_total"`
	FragmentsMissing  []int      `json:"fragments_missing"`
	SamplesB64        string     `json:"samples_b64"`
}

// UnmarshalJSON requires every field except previous_session.
func (w *WaveformUpload) UnmarshalJSON(data []byte) error {
	type plain WaveformUpload
	var value plain
	if err := decodeRequired(data, &value, []string{"schema_version", "gateway_id", "gateway_session_id", "upload_uptime_ms", "event_seq", "gateway_rx_uptime_ms", "gateway_received_at", "device_id", "device_sequence", "boot_count", "channels", "window_samples", "sample_rate_hz", "accel_range", "gyro_range", "dlpf", "burst_profile", "fragment_total", "fragments_missing", "samples_b64"}, map[string]bool{"gateway_received_at": true}); err != nil {
		return err
	}
	*w = WaveformUpload(value)
	return nil
}

// WaveformMetadata is the immutable description of a stored window, without samples.
type WaveformMetadata struct {
	GatewayRxUptimeMS int64      `json:"gateway_rx_uptime_ms"`
	GatewayReceivedAt *time.Time `json:"gateway_received_at"`
	DeviceSequence    uint32     `json:"device_sequence"`
	BootCount         uint32     `json:"boot_count"`
	Channels          int        `json:"channels"`
	WindowSamples     int        `json:"window_samples"`
	SampleRateHz      float64    `json:"sample_rate_hz"`
	AccelRange        int        `json:"accel_range"`
	GyroRange         int        `json:"gyro_range"`
	DLPF              int        `json:"dlpf"`
	BurstProfile      int        `json:"burst_profile"`
	FragmentTotal     int        `json:"fragment_total"`
	FragmentsMissing  []int      `json:"fragments_missing"`
	SamplesSHA256     string     `json:"samples_sha256"`
}

// Waveform is one stored window. SamplesB64 only comes in the single-window route.
type Waveform struct {
	ID               int64  `json:"id"`
	GatewayID        string `json:"gateway_id"`
	GatewaySessionID string `json:"gateway_session_id"`
	EventSeq         int64  `json:"event_seq"`
	DeviceID         string `json:"device_id"`
	WaveformMetadata
	IngestedAt          time.Time  `json:"ingested_at"`
	EstimatedReceivedAt *time.Time `json:"estimated_received_at"`
	QueueDelayMS        int64      `json:"queue_delay_ms"`
	SamplesB64          string     `json:"samples_b64,omitempty"`
}

// WaveformPage is GET /devices/{id}/waveforms.
type WaveformPage struct {
	Items      []Waveform `json:"items"`
	NextCursor string     `json:"next_cursor,omitempty"`
}

// CollectionInput is POST /collections and PUT /collections/{id}. PUT replaces the
// fields; device_id cannot change.
type CollectionInput struct {
	Name      string     `json:"name"`
	DeviceID  string     `json:"device_id"`
	Condition string     `json:"condition"`
	Notes     string     `json:"notes"`
	StartedAt *time.Time `json:"started_at,omitempty"`
	EndedAt   *time.Time `json:"ended_at,omitempty"`
}

// Collection covers the readings and windows of a device within an interval.
type Collection struct {
	ID        int64      `json:"id"`
	Name      string     `json:"name"`
	DeviceID  string     `json:"device_id"`
	Condition string     `json:"condition"`
	Notes     string     `json:"notes"`
	StartedAt time.Time  `json:"started_at"`
	EndedAt   *time.Time `json:"ended_at"`
}

// CollectionList is GET /collections.
type CollectionList struct {
	Items []Collection `json:"items"`
}

// Range holds the extremes and the mean of a quantity.
type Range struct {
	Min float64 `json:"min"`
	Avg float64 `json:"avg"`
	Max float64 `json:"max"`
}

// CollectionSummary is GET /collections/{id}/summary. To is the end of the interval, or
// the moment of the query while the collection is open. Readings and windows without a
// known time cannot be placed in the interval and are only counted, by arrival.
type CollectionSummary struct {
	Collection                 Collection `json:"collection"`
	From                       time.Time  `json:"from"`
	To                         time.Time  `json:"to"`
	Readings                   int64      `json:"readings"`
	ReadingsWithoutMeasurement int64      `json:"readings_without_measurement"`
	ReadingsSaturated          int64      `json:"readings_saturated"`
	ReadingsUndated            int64      `json:"readings_undated"`
	SequenceGaps               int64      `json:"sequence_gaps"`
	SequenceMissing            int64      `json:"sequence_missing"`
	Waveforms                  int64      `json:"waveforms"`
	WaveformsIncomplete        int64      `json:"waveforms_incomplete"`
	WaveformsUndated           int64      `json:"waveforms_undated"`
	FragmentsTotal             int64      `json:"fragments_total"`
	FragmentsMissing           int64      `json:"fragments_missing"`
	RSSIDBm                    *Range     `json:"rssi_dbm"`
	SNRDB                      *Range     `json:"snr_db"`
}

// Update states of a gateway firmware.
const (
	FirmwarePending    = "pending"
	FirmwareOffered    = "offered"
	FirmwareDownloaded = "downloaded"
	FirmwareApplied    = "applied"
	FirmwareFailed     = "failed"
	FirmwareSuperseded = "superseded"
)

// GatewayFirmware describes an uploaded binary and how its update went.
type GatewayFirmware struct {
	ID           string     `json:"id"`
	GatewayID    string     `json:"gateway_id"`
	Version      string     `json:"version"`
	SHA256       string     `json:"sha256"`
	Size         int64      `json:"size"`
	State        string     `json:"state"`
	Reason       string     `json:"reason"`
	CreatedAt    time.Time  `json:"created_at"`
	OfferedAt    *time.Time `json:"offered_at"`
	DownloadedAt *time.Time `json:"downloaded_at"`
	FinishedAt   *time.Time `json:"finished_at"`
}

// GatewayFirmwareStatus is GET /gateways/{id}/firmware. Items are newest first.
type GatewayFirmwareStatus struct {
	GatewayID      string            `json:"gateway_id"`
	CurrentVersion *string           `json:"current_version"`
	Items          []GatewayFirmware `json:"items"`
}

// TLSFingerprint is GET /tls/fingerprint.
type TLSFingerprint struct {
	SHA256 string `json:"sha256"`
}

// SyncDevice is what the gateway has already transmitted to one device.
type SyncDevice struct {
	DeviceID              string `json:"device_id"`
	RadiatedConfigVersion *int   `json:"radiated_config_version"`
	RadiatedCommandSeq    *int   `json:"radiated_command_seq"`
}

// OTAResult reports how a firmware update ended.
type OTAResult struct {
	Version string `json:"version"`
	OK      bool   `json:"ok"`
	Reason  string `json:"reason"`
}

// SyncRequest is POST /gateway/sync.
type SyncRequest struct {
	SchemaVersion    int          `json:"schema_version"`
	GatewayID        string       `json:"gateway_id"`
	GatewaySessionID string       `json:"gateway_session_id"`
	ConfigVersion    int          `json:"config_version"`
	FirmwareVersion  string       `json:"firmware_version"`
	OTAResult        *OTAResult   `json:"ota_result"`
	Devices          []SyncDevice `json:"devices"`
}

// UnmarshalJSON requires the identity, the versions and the device list.
func (s *SyncRequest) UnmarshalJSON(data []byte) error {
	type plain SyncRequest
	var value plain
	if err := decodeRequired(data, &value, []string{"schema_version", "gateway_id", "gateway_session_id", "config_version", "firmware_version", "devices"}, nil); err != nil {
		return err
	}
	*s = SyncRequest(value)
	return nil
}

// UnmarshalJSON makes ok explicit: a missing result must not read as a failure.
func (o *OTAResult) UnmarshalJSON(data []byte) error {
	if bytes.Equal(bytes.TrimSpace(data), []byte("null")) {
		return nil
	}
	type plain OTAResult
	var value plain
	if err := decodeRequired(data, &value, []string{"version", "ok"}, nil); err != nil {
		return err
	}
	*o = OTAResult(value)
	return nil
}

// SyncCommand is the command waiting for the telemetry to confirm it.
type SyncCommand struct {
	Seq  int    `json:"seq"`
	Kind string `json:"kind"`
}

// SyncDeviceUpdate is what a gateway must relay to one device.
type SyncDeviceUpdate struct {
	DeviceID string        `json:"device_id"`
	Config   *DeviceConfig `json:"config"`
	Command  *SyncCommand  `json:"command"`
}

// FirmwareOffer points the gateway to the binary it must install.
type FirmwareOffer struct {
	Version string `json:"version"`
	SHA256  string `json:"sha256"`
	Size    int64  `json:"size"`
	Path    string `json:"path"`
}

// SyncResponse answers a gateway sync. ServerTime keeps the fixed millisecond layout of
// the contract, so the firmware does not need a general RFC 3339 parser.
type SyncResponse struct {
	SchemaVersion    int                    `json:"schema_version"`
	GatewayID        string                 `json:"gateway_id"`
	GatewaySessionID string                 `json:"gateway_session_id"`
	ServerTime       string                 `json:"server_time"`
	GatewayConfig    *GatewayConfigDelivery `json:"gateway_config"`
	Devices          []SyncDeviceUpdate     `json:"devices"`
	Firmware         *FirmwareOffer         `json:"firmware"`
}

// SyncTimeLayout is the layout of SyncResponse.ServerTime.
const SyncTimeLayout = "2006-01-02T15:04:05.000Z"

// MarshalConfigContent returns the configuration without its version, the form in which
// two configurations are compared.
func MarshalConfigContent(cfg DeviceConfig) ([]byte, error) {
	cfg.Version = 0
	if cfg.Routine == nil {
		cfg.Routine = []RoutineWindow{}
	}
	return json.Marshal(cfg)
}
