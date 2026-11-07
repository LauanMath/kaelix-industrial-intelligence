package contracts

import (
	"encoding/json"
	"math"
	"strings"
	"testing"
)

func near(t *testing.T, name string, got, want, tolerance float64) {
	t.Helper()
	if math.Abs(got-want) > tolerance {
		t.Fatalf("%s: got %.4f, want %.4f", name, got, want)
	}
}

// The expected values are the published figures of the specification.
func TestLoRaAirtime(t *testing.T) {
	near(t, "telemetry", LoRaAirtimeMS(71, 9, 125), 411, 0.5)
	near(t, "full fragment, SF7 BW250", LoRaAirtimeMS(254, 7, 250), 199.808, 0.001)
	near(t, "tiny frame never has negative blocks", LoRaAirtimeMS(0, 12, 125), (12.25+8)*4096.0/125, 0.001)
	if LoRaAirtimeMS(10, 7, 125) >= LoRaAirtimeMS(254, 7, 125) {
		t.Fatal("airtime must grow with the payload")
	}
}

func TestBurstProfileTable(t *testing.T) {
	for profile, seconds := range []float64{32, 10.2, 5.1, 2.6} {
		near(t, "6 channels, 512 samples", BurstTransmissionMS(512, 63, profile)/1000, seconds, 0.09)
	}
	if got := FragmentCount(512, 63); got != 26 {
		t.Fatalf("fragments: %d", got)
	}
	if got := FragmentCount(4096, 63); got != 205 {
		t.Fatalf("largest window fragments: %d", got)
	}
	for _, mask := range []int{1, 3, 7, 15, 31, 63} {
		if FragmentPayload(mask) != 240 {
			t.Fatalf("mask %d payload %d", mask, FragmentPayload(mask))
		}
	}
}

func TestDefaultsAreValidAndFit(t *testing.T) {
	cfg := DefaultDeviceConfig()
	if issues := ValidateDeviceConfig(cfg); len(issues) != 0 {
		t.Fatalf("defaults rejected: %+v", issues)
	}
	// 0.812 acquisition + 1.122 control + 5.123 air + 1.3 gaps + 0.3 map.
	near(t, "default cycle", CycleTimeSeconds(cfg), 8.657, 0.002)
	if MinimumCycleInterval(cfg) != 9 {
		t.Fatalf("minimum %d", MinimumCycleInterval(cfg))
	}
}

func TestMinimumCycleRejectionCarriesTheMinimum(t *testing.T) {
	cfg := DefaultDeviceConfig()
	cfg.CycleIntervalS = 9
	if issues := ValidateDeviceConfig(cfg); len(issues) != 0 {
		t.Fatalf("exact minimum rejected: %+v", issues)
	}
	cfg.CycleIntervalS = 8
	issues := ValidateDeviceConfig(cfg)
	if len(issues) != 1 || issues[0].Path != "cycle_interval_s" || issues[0].Minimum == nil || *issues[0].Minimum != 9 || !strings.Contains(issues[0].Reason, "9 s") {
		t.Fatalf("issues: %+v", issues)
	}
	// Turning the raw signal off removes the burst from the calculation.
	cfg.WaveEvery = 0
	if issues := ValidateDeviceConfig(cfg); len(issues) != 0 {
		t.Fatalf("without burst: %+v", issues)
	}
	// The shortest possible cycle still exceeds one second, so 1 s always returns a minimum.
	cfg = DefaultDeviceConfig()
	cfg.WindowSamples, cfg.WaveEvery, cfg.CycleIntervalS = 256, 0, 1
	issues = ValidateDeviceConfig(cfg)
	if len(issues) != 1 || *issues[0].Minimum != 2 {
		t.Fatalf("issues: %+v", issues)
	}
	// Largest window on the slowest profile, within the 3600 s range.
	cfg = DefaultDeviceConfig()
	cfg.WindowSamples, cfg.BurstProfile, cfg.CycleIntervalS = 4096, 0, 60
	issues = ValidateDeviceConfig(cfg)
	if len(issues) != 1 || issues[0].Minimum == nil || *issues[0].Minimum < 250 || *issues[0].Minimum > 300 {
		t.Fatalf("issues: %+v", issues)
	}
	cfg.CycleIntervalS = *issues[0].Minimum
	if issues = ValidateDeviceConfig(cfg); len(issues) != 0 {
		t.Fatalf("returned minimum rejected: %+v", issues)
	}
}

func TestDeviceConfigRanges(t *testing.T) {
	for _, tc := range []struct {
		path   string
		change func(*DeviceConfig)
	}{
		{"cycle_interval_s", func(c *DeviceConfig) { c.CycleIntervalS = 0 }},
		{"cycle_interval_s", func(c *DeviceConfig) { c.CycleIntervalS = 3601 }},
		{"window_samples", func(c *DeviceConfig) { c.WindowSamples = 500 }},
		{"channels", func(c *DeviceConfig) { c.Channels = 0 }},
		{"channels", func(c *DeviceConfig) { c.Channels = 64 }},
		{"channels", func(c *DeviceConfig) { c.Channels = 3 }},
		{"accel_range", func(c *DeviceConfig) { c.AccelRange = 4 }},
		{"accel_range", func(c *DeviceConfig) { c.AccelRange = -1 }},
		{"gyro_range", func(c *DeviceConfig) { c.GyroRange = 4 }},
		{"dlpf", func(c *DeviceConfig) { c.DLPF = 7 }},
		{"tx_power_dbm", func(c *DeviceConfig) { c.TxPowerDBm = 1 }},
		{"tx_power_dbm", func(c *DeviceConfig) { c.TxPowerDBm = 18 }},
		{"burst_profile", func(c *DeviceConfig) { c.BurstProfile = 4 }},
		{"wave_every", func(c *DeviceConfig) { c.WaveEvery = 256 }},
		{"wave_every", func(c *DeviceConfig) { c.WaveEvery = -1 }},
		{"link_loss_s", func(c *DeviceConfig) { c.LinkLossS = 59 }},
		{"link_loss_s", func(c *DeviceConfig) { c.LinkLossS = 86401 }},
		{"checkin_interval_s", func(c *DeviceConfig) { c.CheckinIntervalS = 59 }},
		{"checkin_interval_s", func(c *DeviceConfig) { c.CheckinIntervalS = 3601 }},
		{"tz_offset_min", func(c *DeviceConfig) { c.TZOffsetMin = -721 }},
		{"tz_offset_min", func(c *DeviceConfig) { c.TZOffsetMin = 841 }},
		{"routine", func(c *DeviceConfig) {
			c.Routine = make([]RoutineWindow, 7)
			for i := range c.Routine {
				c.Routine[i] = RoutineWindow{1, 0, 60}
			}
		}},
		{"routine[0].days", func(c *DeviceConfig) { c.Routine = []RoutineWindow{{0, 0, 60}} }},
		{"routine[0].days", func(c *DeviceConfig) { c.Routine = []RoutineWindow{{128, 0, 60}} }},
		{"routine[0].start_min", func(c *DeviceConfig) { c.Routine = []RoutineWindow{{1, 1441, 60}} }},
		{"routine[0].end_min", func(c *DeviceConfig) { c.Routine = []RoutineWindow{{1, 0, 1441}} }},
		{"routine[0].end_min", func(c *DeviceConfig) { c.Routine = []RoutineWindow{{1, 60, 60}} }},
	} {
		cfg := DefaultDeviceConfig()
		tc.change(&cfg)
		found := false
		for _, issue := range ValidateDeviceConfig(cfg) {
			found = found || issue.Path == tc.path
		}
		if !found {
			t.Fatalf("%s not rejected: %+v", tc.path, cfg)
		}
	}
	// Limits are inclusive, and a window may cross midnight.
	cfg := DefaultDeviceConfig()
	cfg.CycleIntervalS, cfg.LinkLossS, cfg.CheckinIntervalS, cfg.TZOffsetMin, cfg.TxPowerDBm, cfg.Channels = 3600, 60, 3600, 840, 17, 4
	cfg.Routine = []RoutineWindow{{127, 1320, 360}, {1, 0, 1440}}
	if issues := ValidateDeviceConfig(cfg); len(issues) != 0 {
		t.Fatalf("limits rejected: %+v", issues)
	}
}

func TestAbsentFieldsTakeDefaultsNotZero(t *testing.T) {
	cfg := DefaultDeviceConfig()
	if err := json.Unmarshal([]byte(`{"cycle_interval_s":30,"gyro_range":2}`), &cfg); err != nil {
		t.Fatal(err)
	}
	if cfg.CycleIntervalS != 30 || cfg.GyroRange != 2 || cfg.WindowSamples != 512 || cfg.AccelRange != 1 || cfg.TZOffsetMin != -180 || cfg.Channels != 63 {
		t.Fatalf("%+v", cfg)
	}
	gateway := DefaultGatewayConfig()
	if err := json.Unmarshal([]byte(`{"ap_when_offline":false,"wifi":[{"ssid":"a"}]}`), &gateway); err != nil {
		t.Fatal(err)
	}
	if gateway.APWhenOffline || gateway.TxPowerDBm != 17 || gateway.HeartbeatIntervalS != 30 || gateway.SyncIntervalS != 5 || gateway.Wifi[0].Password != nil {
		t.Fatalf("%+v", gateway)
	}
}

func TestGatewayConfigRanges(t *testing.T) {
	text := func(s string) *string { return &s }
	for _, tc := range []struct {
		path   string
		change func(*GatewayConfigInput)
	}{
		{"wifi", func(c *GatewayConfigInput) { c.Wifi = make([]WifiCredential, 6) }},
		{"wifi[0].ssid", func(c *GatewayConfigInput) { c.Wifi = []WifiCredential{{SSID: ""}} }},
		{"wifi[0].ssid", func(c *GatewayConfigInput) { c.Wifi = []WifiCredential{{SSID: strings.Repeat("a", 33)}} }},
		{"wifi[1].ssid", func(c *GatewayConfigInput) { c.Wifi = []WifiCredential{{SSID: "a"}, {SSID: "a"}} }},
		{"wifi[0].password", func(c *GatewayConfigInput) { c.Wifi = []WifiCredential{{SSID: "a", Password: text("curta")}} }},
		{"wifi[0].password", func(c *GatewayConfigInput) {
			c.Wifi = []WifiCredential{{SSID: "a", Password: text(strings.Repeat("z", 64))}}
		}},
		{"heartbeat_interval_s", func(c *GatewayConfigInput) { c.HeartbeatIntervalS = 9 }},
		{"heartbeat_interval_s", func(c *GatewayConfigInput) { c.HeartbeatIntervalS = 301 }},
		{"sync_interval_s", func(c *GatewayConfigInput) { c.SyncIntervalS = 1 }},
		{"sync_interval_s", func(c *GatewayConfigInput) { c.SyncIntervalS = 61 }},
		{"tx_power_dbm", func(c *GatewayConfigInput) { c.TxPowerDBm = 1 }},
		{"tx_power_dbm", func(c *GatewayConfigInput) { c.TxPowerDBm = 18 }},
		{"ap_when_offline", func(c *GatewayConfigInput) { c.Wifi = nil; c.APWhenOffline = false }},
	} {
		cfg := DefaultGatewayConfig()
		tc.change(&cfg)
		found := false
		for _, issue := range ValidateGatewayConfig(cfg) {
			found = found || issue.Path == tc.path
			if strings.Contains(issue.Reason, "curta") {
				t.Fatal("validation echoed a password")
			}
		}
		if !found {
			t.Fatalf("%s not rejected: %+v", tc.path, cfg)
		}
	}
	valid := DefaultGatewayConfig()
	valid.Wifi = []WifiCredential{{SSID: "aberta", Password: text("")}, {SSID: "wpa", Password: text("12345678")}, {SSID: "psk", Password: text(strings.Repeat("a", 64))}, {SSID: "mantida"}}
	if issues := ValidateGatewayConfig(valid); len(issues) != 0 {
		t.Fatalf("%+v", issues)
	}
}

func TestPhysicalConversion(t *testing.T) {
	near(t, "1 g at ±2 g", AccelG(16384, 0), 1, 1e-12)
	near(t, "1 g at ±4 g", AccelG(8192, 1), 1, 1e-12)
	near(t, "-1 g at ±16 g", AccelG(-2048, 3), -1, 1e-12)
	near(t, "1 °/s at ±250 °/s", GyroDPS(131, 0), 1, 1e-12)
	near(t, "2 °/s at ±500 °/s", GyroDPS(131, 1), 2, 1e-12)
	near(t, "8 °/s at ±2000 °/s", GyroDPS(131, 3), 8, 1e-12)
}

// A reading stored before the v3 fields existed must keep its JSON unchanged.
func TestOldReadingMarshalsWithoutNewFields(t *testing.T) {
	const old = `{"event_seq":1,"gateway_rx_uptime_ms":12000,"gateway_received_at":null,"packet_version":2,"device_id":"D6F61B44","device_sequence":1,"state":"UNKNOWN","diag_code":0,"rms_g":0.0035,"temperature_c":null,"rssi_dbm":-55,"snr_db":13.5,"packet_crc16":33642}`
	var r Reading
	if err := json.Unmarshal([]byte(old), &r); err != nil {
		t.Fatal(err)
	}
	raw, err := json.Marshal(r)
	if err != nil {
		t.Fatal(err)
	}
	if string(raw) != old {
		t.Fatalf("re-encoded differently:\n%s\n%s", raw, old)
	}
	// Explicit nulls in new optional fields mean the same as absent.
	var withNull Reading
	if err = json.Unmarshal([]byte(strings.Replace(old, `"packet_crc16":33642`, `"packet_crc16":33642,"config_version":null,"device_time":null`, 1)), &withNull); err != nil {
		t.Fatal(err)
	}
	again, _ := json.Marshal(withNull)
	if string(again) != old {
		t.Fatalf("null fields changed the payload: %s", again)
	}
}

func TestDeviceTimeMustBeUTC(t *testing.T) {
	const base = `{"event_seq":1,"gateway_rx_uptime_ms":0,"gateway_received_at":null,"packet_version":3,"device_id":"D6F61B44","device_sequence":1,"state":"UNKNOWN","diag_code":0,"rms_g":null,"temperature_c":null,"rssi_dbm":-55,"snr_db":13.5,"packet_crc16":1,"device_time":"%s"}`
	var r Reading
	if json.Unmarshal([]byte(strings.Replace(base, "%s", "2026-11-02T04:59:22+00:00", 1)), &r) == nil {
		t.Fatal("non-Z device_time accepted")
	}
	if err := json.Unmarshal([]byte(strings.Replace(base, "%s", "2026-11-02T04:59:22Z", 1)), &r); err != nil {
		t.Fatal(err)
	}
}

func TestSyncAndWaveformRequiredFields(t *testing.T) {
	var s SyncRequest
	if json.Unmarshal([]byte(`{"schema_version":1,"gateway_id":"a","gateway_session_id":"b","firmware_version":"x","devices":[]}`), &s) == nil {
		t.Fatal("missing config_version accepted")
	}
	if err := json.Unmarshal([]byte(`{"schema_version":1,"gateway_id":"a","gateway_session_id":"b","config_version":0,"firmware_version":"x","ota_result":null,"devices":[]}`), &s); err != nil {
		t.Fatal(err)
	}
	if json.Unmarshal([]byte(`{"schema_version":1,"gateway_id":"a","gateway_session_id":"b","config_version":0,"firmware_version":"x","ota_result":{"version":"v"},"devices":[]}`), &s) == nil {
		t.Fatal("ota_result without ok accepted")
	}
	var w WaveformUpload
	if json.Unmarshal([]byte(`{"schema_version":1}`), &w) == nil {
		t.Fatal("incomplete waveform accepted")
	}
}
