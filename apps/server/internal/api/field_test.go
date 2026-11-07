package api

import (
	"bytes"
	"context"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"net/http/httptrace"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	c "kaelix/contracts"
	"kaelix/server/internal/secure"
	"kaelix/server/internal/store"
)

const (
	fieldDevice  = "AB12CD34"
	fieldSession = "17162ec5-4fe1-4dac-a871-879120324a57"
	wifiSecret   = "segredo-do-wifi-123"
)

var quietLog = slog.New(slog.NewTextHandler(io.Discard, nil))

type fieldEnv struct {
	db      *store.Store
	pool    *pgxpool.Pool
	user    client
	gateway client
	id      string
	server  *httptest.Server
}

func newFieldEnv(t *testing.T) *fieldEnv {
	t.Helper()
	db, dsn := testDatabase(t)
	t.Cleanup(db.Close)
	box, err := secure.NewBox(bytes.Repeat([]byte{9}, 32))
	if err != nil {
		t.Fatal(err)
	}
	db.UseSealer(box)
	ctx := context.Background()
	if err = db.BootstrapAdmin(ctx, "field@example.com", "field-test-password-123"); err != nil {
		t.Fatal(err)
	}
	token, _, err := db.Login(ctx, "field@example.com", "field-test-password-123")
	if err != nil {
		t.Fatal(err)
	}
	g, credential, err := db.CreateGateway(ctx, c.GatewayInput{Name: "Field gateway"})
	if err != nil {
		t.Fatal(err)
	}
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	server := httptest.NewServer(NewWithOptions(db, quietLog, Options{TLSFingerprint: strings.Repeat("ab", 32)}))
	t.Cleanup(server.Close)
	return &fieldEnv{db: db, pool: pool, user: client{t: t, base: server.URL, token: token}, gateway: client{t: t, base: server.URL, token: credential}, id: g.ID, server: server}
}

func pointer[T any](v T) *T { return &v }

func (e *fieldEnv) reading(seq int64) c.Reading {
	return c.Reading{EventSeq: seq, GatewayRxUptimeMS: 1000 * seq, PacketVersion: 3, DeviceID: fieldDevice, DeviceSequence: pointer(uint32(seq)), State: "NORMAL", DiagCode: pointer(uint8(0)), RMSG: pointer(0.02), RSSIDBm: pointer(-50.0), SNRDB: pointer(9.0), PacketCRC16: pointer(uint16(1)), BootCount: pointer(uint32(2))}
}

func (e *fieldEnv) send(readings ...c.Reading) c.IngestResult {
	e.user.t.Helper()
	return parse[c.IngestResult](e.user.t, e.gateway.request("POST", "/api/v1/ingest/readings", c.ReadingBatch{SchemaVersion: 1, GatewayID: e.id, GatewaySessionID: fieldSession, UploadUptimeMS: 1 << 40, Readings: readings}, 200))
}

func (e *fieldEnv) sync(configVersion int, devices ...c.SyncDevice) c.SyncResponse {
	e.user.t.Helper()
	if devices == nil {
		devices = []c.SyncDevice{}
	}
	raw := e.gateway.request("POST", "/api/v1/gateway/sync", c.SyncRequest{SchemaVersion: 1, GatewayID: e.id, GatewaySessionID: fieldSession, ConfigVersion: configVersion, FirmwareVersion: "gateway-2.0.0", Devices: devices}, 200)
	checkSchema(e.user.t, "sync-response", raw)
	return parse[c.SyncResponse](e.user.t, raw)
}

func (e *fieldEnv) config() c.DeviceConfigState {
	e.user.t.Helper()
	return parse[c.DeviceConfigState](e.user.t, e.user.request("GET", "/api/v1/devices/"+fieldDevice+"/config", nil, 200))
}

func stateOf(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

func (e *fieldEnv) device() {
	e.user.t.Helper()
	e.user.request("POST", "/api/v1/devices", c.DeviceInput{ID: fieldDevice}, 201)
}

// The new examples of the shared contract keep matching their schemas and the validators.
func TestFieldContractExamples(t *testing.T) {
	for _, pair := range [][2]string{{"readings-request-v3", "readings-request"}, {"heartbeat-request-extended", "heartbeat-request"}, {"waveforms-request", "waveforms-request"}, {"waveforms-response", "readings-response"}, {"sync-request", "sync-request"}, {"sync-response", "sync-response"}} {
		checkSchema(t, pair[1], contractFile(t, "examples/"+pair[0]+".json"))
	}
	if err := validateBatch(parse[c.ReadingBatch](t, contractFile(t, "examples/readings-request-v3.json")), time.Now()); err != nil {
		t.Fatal(err)
	}
	if err := validateHeartbeat(parse[c.Heartbeat](t, contractFile(t, "examples/heartbeat-request-extended.json"))); err != nil {
		t.Fatal(err)
	}
	if _, err := validateWaveform(parse[c.WaveformUpload](t, contractFile(t, "examples/waveforms-request.json"))); err != nil {
		t.Fatal(err)
	}
	if err := validateSync(parse[c.SyncRequest](t, contractFile(t, "examples/sync-request.json"))); err != nil {
		t.Fatal(err)
	}
	// A v2 reading cannot carry v3 fields.
	batch := parse[c.ReadingBatch](t, contractFile(t, "examples/readings-request.json"))
	batch.Readings[0].ConfigVersion = pointer(uint16(1))
	if err := validateBatch(batch, time.Now()); err == nil {
		t.Fatal("v2 reading with config_version accepted")
	}
	batch.Readings[0].ConfigVersion = nil
	batch.Readings[0].PacketVersion = 4
	if err := validateBatch(batch, time.Now()); err == nil {
		t.Fatal("packet_version 4 accepted")
	}
}

func TestHeartbeatExtensions(t *testing.T) {
	h := parse[c.Heartbeat](t, contractFile(t, "examples/heartbeat-request-extended.json"))
	h.Health.SSID = pointer(strings.Repeat("é", 65))
	if validateHeartbeat(h) == nil {
		t.Fatal("65 characters accepted")
	}
	h.Health.SSID = pointer(strings.Repeat("é", 64))
	if err := validateHeartbeat(h); err != nil {
		t.Fatal(err)
	}
	h.Health.SpoolUsedBytes = pointer(int64(2))
	h.Health.SpoolCapacityBytes = pointer(int64(1))
	if validateHeartbeat(h) == nil {
		t.Fatal("spool above capacity accepted")
	}
	env := newFieldEnv(t)
	first := parse[c.Heartbeat](t, contractFile(t, "examples/heartbeat-request-extended.json"))
	first.GatewayID = env.id
	first.Counters.SpoolDropped, first.Counters.WaveformsAcknowledged = pointer(int64(5)), pointer(int64(5))
	env.gateway.request("POST", "/api/v1/ingest/heartbeat", first, 200)
	next := first
	next.HeartbeatSeq, next.UptimeMS = 2, 60000
	next.Counters.WaveformsAcknowledged = pointer(int64(4))
	env.gateway.request("POST", "/api/v1/ingest/heartbeat", next, 422)
	next.Counters.WaveformsAcknowledged = pointer(int64(6))
	next.Counters.SpoolDropped = pointer(int64(4))
	env.gateway.request("POST", "/api/v1/ingest/heartbeat", next, 422)
	next.Counters.SpoolDropped = pointer(int64(5))
	env.gateway.request("POST", "/api/v1/ingest/heartbeat", next, 200)
	items := parse[struct {
		Items []c.Gateway `json:"items"`
	}](t, env.user.request("GET", "/api/v1/gateways", nil, 200))
	if got := items.Items[0].Heartbeat.Health; got.SSID == nil || *got.SSID != "rede-local" || *got.SpoolUsedBytes != 4096 {
		t.Fatalf("health %+v", got)
	}
}

func TestDeviceConfigVersionsAndStates(t *testing.T) {
	env := newFieldEnv(t)
	env.user.request("GET", "/api/v1/devices/"+fieldDevice+"/config", nil, 404)
	env.user.request("PUT", "/api/v1/devices/"+fieldDevice+"/config", c.DefaultDeviceConfig(), 404)
	env.device()
	// No configuration saved: nothing is invented, and nothing goes to the radio.
	empty := env.config()
	if empty.Config != nil || empty.State != nil || empty.ReportedVersion != nil || empty.Defaults.CycleIntervalS != 11 {
		t.Fatalf("empty state: %+v", empty)
	}
	if got := env.sync(0); len(got.Devices) != 0 {
		t.Fatalf("no configuration, no update: %+v", got.Devices)
	}
	// An absent field takes its default; a version sent by the client is ignored.
	saved := parse[c.DeviceConfigState](t, env.user.request("PUT", "/api/v1/devices/"+fieldDevice+"/config", map[string]any{"version": 999, "cycle_interval_s": 20}, 200))
	if saved.Config.Version != 1 || saved.Config.CycleIntervalS != 20 || saved.Config.WindowSamples != 512 || stateOf(saved.State) != c.StatePending || saved.UpdatedAt == nil {
		t.Fatalf("saved: %+v", saved)
	}
	// Saving the same content is not an alteration.
	again := parse[c.DeviceConfigState](t, env.user.request("PUT", "/api/v1/devices/"+fieldDevice+"/config", map[string]any{"cycle_interval_s": 20}, 200))
	if again.Config.Version != 1 {
		t.Fatalf("version bumped without a change: %d", again.Config.Version)
	}
	got := env.sync(0)
	if len(got.Devices) != 1 || got.Devices[0].Config == nil || got.Devices[0].Config.Version != 1 || got.Devices[0].Config.CycleIntervalS != 20 || got.Devices[0].Command != nil {
		t.Fatalf("sync after save: %+v", got.Devices)
	}
	// Radiated only when the gateway reports this very version.
	env.sync(0, c.SyncDevice{DeviceID: fieldDevice, RadiatedConfigVersion: pointer(0)})
	if state := env.config(); stateOf(state.State) != c.StatePending || state.RadiatedAt != nil {
		t.Fatalf("older radiated version changed the state: %+v", state)
	}
	env.sync(0, c.SyncDevice{DeviceID: fieldDevice, RadiatedConfigVersion: pointer(1)})
	radiated := env.config()
	if stateOf(radiated.State) != c.StateRadiated || radiated.RadiatedAt == nil || radiated.AppliedAt != nil {
		t.Fatalf("radiated: %+v", radiated)
	}
	// Still sent while the telemetry has not confirmed it.
	if got = env.sync(0); len(got.Devices) != 1 || got.Devices[0].Config == nil {
		t.Fatalf("unconfirmed configuration dropped: %+v", got.Devices)
	}
	// A reading with another version does not apply it, a v2 reading says nothing.
	r := env.reading(1)
	r.ConfigVersion = pointer(uint16(0))
	env.send(r)
	if state := env.config(); stateOf(state.State) != c.StateRadiated || *state.ReportedVersion != 0 {
		t.Fatalf("other version: %+v", state)
	}
	// The telemetry confirms it.
	r = env.reading(2)
	r.ConfigVersion = pointer(uint16(1))
	env.send(r)
	applied := env.config()
	if stateOf(applied.State) != c.StateApplied || *applied.ReportedVersion != 1 || applied.AppliedAt == nil {
		t.Fatalf("applied: %+v", applied)
	}
	if got = env.sync(0); len(got.Devices) != 0 {
		t.Fatalf("applied configuration still sent: %+v", got.Devices)
	}
	// A duplicate does not touch the state; a new version starts over.
	env.send(r)
	next := parse[c.DeviceConfigState](t, env.user.request("PUT", "/api/v1/devices/"+fieldDevice+"/config", map[string]any{"cycle_interval_s": 30}, 200))
	if next.Config.Version != 2 || stateOf(next.State) != c.StatePending || next.AppliedAt != nil || next.RadiatedAt != nil {
		t.Fatalf("new version: %+v", next)
	}
	// The gateway still reports the previous version as radiated: that is not this one.
	env.sync(0, c.SyncDevice{DeviceID: fieldDevice, RadiatedConfigVersion: pointer(1)})
	if state := env.config(); stateOf(state.State) != c.StatePending {
		t.Fatalf("stale radiated version: %+v", state)
	}
	// Confirmation without the radiated report still counts: telemetry is what decides.
	r = env.reading(3)
	r.ConfigVersion = pointer(uint16(2))
	env.send(r)
	if state := env.config(); stateOf(state.State) != c.StateApplied {
		t.Fatalf("state: %+v", state)
	}
	// Reverted on link loss: live telemetry may go back, and the state follows.
	r = env.reading(4)
	r.ConfigVersion = pointer(uint16(1))
	env.send(r)
	if state := env.config(); stateOf(state.State) == c.StateApplied || *state.ReportedVersion != 1 || state.AppliedAt != nil {
		t.Fatalf("revert: %+v", state)
	}
	if got = env.sync(0); len(got.Devices) != 1 || got.Devices[0].Config.Version != 2 {
		t.Fatalf("reverted configuration not sent again: %+v", got.Devices)
	}
}

// A device reporting a version must not make a new configuration look applied at once.
func TestFirstVersionStaysAwayFromTheReportedOne(t *testing.T) {
	env := newFieldEnv(t)
	r := env.reading(1)
	r.ConfigVersion = pointer(uint16(1))
	env.send(r)
	state := env.config()
	if state.Config != nil || state.State != nil || *state.ReportedVersion != 1 {
		t.Fatalf("reported only: %+v", state)
	}
	saved := parse[c.DeviceConfigState](t, env.user.request("PUT", "/api/v1/devices/"+fieldDevice+"/config", map[string]any{"cycle_interval_s": 20}, 200))
	if saved.Config.Version != 2 || stateOf(saved.State) != c.StatePending {
		t.Fatalf("saved: %+v", saved)
	}
}

func TestConfigValidationThroughTheRoute(t *testing.T) {
	env := newFieldEnv(t)
	env.device()
	path := "/api/v1/devices/" + fieldDevice + "/config"
	raw := env.user.request("PUT", path, map[string]any{"cycle_interval_s": 8}, 422)
	checkSchema(t, "error-response", raw)
	failure := parse[c.APIError](t, raw)
	if failure.Error.Code != "INVALID_REQUEST" || len(failure.Error.Details) != 1 || failure.Error.Details[0].Path != "cycle_interval_s" || failure.Error.Details[0].Minimum == nil || *failure.Error.Details[0].Minimum != 9 {
		t.Fatalf("minimum cycle: %+v", failure)
	}
	for _, body := range []map[string]any{{"window_samples": 300}, {"channels": 3}, {"tx_power_dbm": 18}, {"tz_offset_min": 900}, {"routine": []map[string]int{{"days": 0, "start_min": 0, "end_min": 60}}}} {
		checkSchema(t, "error-response", env.user.request("PUT", path, body, 422))
	}
	env.user.request("PUT", path, map[string]any{"unknown": 1}, 400)
	if state := env.config(); state.Config != nil {
		t.Fatalf("rejected configuration stored: %+v", state)
	}
	gatewayPath := "/api/v1/gateways/" + env.id + "/config"
	failure = parse[c.APIError](t, env.user.request("PUT", gatewayPath, map[string]any{"tx_power_dbm": 18, "sync_interval_s": 1}, 422))
	if len(failure.Error.Details) != 2 {
		t.Fatalf("gateway validation: %+v", failure)
	}
}

// Late backlog from a previous boot must not move the reported version back.
func TestReportedVersionIgnoresOlderBacklog(t *testing.T) {
	env := newFieldEnv(t)
	env.device()
	env.user.request("PUT", "/api/v1/devices/"+fieldDevice+"/config", map[string]any{"cycle_interval_s": 20}, 200)
	live := env.reading(10)
	live.ConfigVersion = pointer(uint16(1))
	live.BootCount, live.DeviceSequence = pointer(uint32(3)), pointer(uint32(100))
	env.send(live)
	old := env.reading(5)
	old.ConfigVersion, old.BootCount, old.DeviceSequence = pointer(uint16(0)), pointer(uint32(3)), pointer(uint32(50))
	env.gateway.request("POST", "/api/v1/ingest/readings", c.ReadingBatch{SchemaVersion: 1, GatewayID: env.id, GatewaySessionID: "27162ec5-4fe1-4dac-a871-879120324a57", PreviousSession: true, UploadUptimeMS: 5000, Readings: []c.Reading{old}}, 200)
	if state := env.config(); stateOf(state.State) != c.StateApplied || *state.ReportedVersion != 1 {
		t.Fatalf("backlog regressed the report: %+v", state)
	}
	// A previous-session reading that the device counters show to be newer does count.
	newer := env.reading(6)
	newer.ConfigVersion, newer.BootCount, newer.DeviceSequence = pointer(uint16(7)), pointer(uint32(4)), pointer(uint32(1))
	env.gateway.request("POST", "/api/v1/ingest/readings", c.ReadingBatch{SchemaVersion: 1, GatewayID: env.id, GatewaySessionID: "27162ec5-4fe1-4dac-a871-879120324a57", PreviousSession: true, UploadUptimeMS: 6000, Readings: []c.Reading{newer}}, 200)
	if state := env.config(); *state.ReportedVersion != 7 {
		t.Fatalf("newer backlog ignored: %+v", state)
	}
}

func TestPreviousSession(t *testing.T) {
	env := newFieldEnv(t)
	known := time.Date(2026, 11, 2, 4, 59, 22, 0, time.UTC)
	timed := env.reading(1)
	timed.GatewayRxUptimeMS = 4000
	timed.GatewayReceivedAt = &known
	blind := env.reading(2)
	blind.GatewayRxUptimeMS = 5000
	session := "27162ec5-4fe1-4dac-a871-879120324a57"
	batch := c.ReadingBatch{SchemaVersion: 1, GatewayID: env.id, GatewaySessionID: session, PreviousSession: true, UploadUptimeMS: 5000, Readings: []c.Reading{timed, blind}}
	ack := parse[c.IngestResult](t, env.gateway.request("POST", "/api/v1/ingest/readings", batch, 200))
	if len(ack.Acknowledged) != 2 || ack.Acknowledged[0].Status != "stored" {
		t.Fatalf("ack: %+v", ack)
	}
	page, err := env.db.History(context.Background(), fieldDevice, store.HistoryFilter{Limit: 10})
	if err != nil || len(page.Items) != 2 {
		t.Fatalf("history: %+v %v", page, err)
	}
	// Ordered with unknown times last, so the timed reading comes first.
	if first := page.Items[0]; first.EstimatedReceivedAt == nil || !first.EstimatedReceivedAt.Equal(known) || first.QueueDelayMS != 0 {
		t.Fatalf("timed: %+v", first)
	}
	if second := page.Items[1]; second.EstimatedReceivedAt != nil || second.QueueDelayMS != 0 {
		t.Fatalf("no gateway time must stay null: %+v", second)
	}
	// The same events resent, also without the flag, are duplicates and keep the first values.
	batch.PreviousSession = false
	batch.UploadUptimeMS = 90000
	ack = parse[c.IngestResult](t, env.gateway.request("POST", "/api/v1/ingest/readings", batch, 200))
	if ack.Acknowledged[0].Status != "duplicate" || ack.Acknowledged[1].Status != "duplicate" {
		t.Fatalf("retry: %+v", ack)
	}
	page, _ = env.db.History(context.Background(), fieldDevice, store.HistoryFilter{Limit: 10})
	if page.Items[1].EstimatedReceivedAt != nil || page.Items[1].QueueDelayMS != 0 {
		t.Fatalf("retry changed the record: %+v", page.Items[1])
	}
	// Without the flag, the rule of the live session is unchanged.
	live := env.reading(3)
	live.GatewayRxUptimeMS = 1000
	env.gateway.request("POST", "/api/v1/ingest/readings", c.ReadingBatch{SchemaVersion: 1, GatewayID: env.id, GatewaySessionID: fieldSession, UploadUptimeMS: 4000, Readings: []c.Reading{live}}, 200)
	page, _ = env.db.History(context.Background(), fieldDevice, store.HistoryFilter{Limit: 1})
	if page.Items[0].QueueDelayMS != 3000 || page.Items[0].EstimatedReceivedAt == nil {
		t.Fatalf("live: %+v", page.Items[0])
	}
	// Waveforms follow the same rule.
	w := waveformUpload(env.id, session, 2, 256, 4)
	w.PreviousSession, w.UploadUptimeMS, w.GatewayRxUptimeMS = true, 5000, 5000
	env.gateway.request("POST", "/api/v1/ingest/waveforms", w, 200)
	list := parse[c.WaveformPage](t, env.user.request("GET", "/api/v1/devices/"+fieldDevice+"/waveforms", nil, 200))
	if len(list.Items) != 1 || list.Items[0].EstimatedReceivedAt != nil || list.Items[0].QueueDelayMS != 0 {
		t.Fatalf("waveform: %+v", list)
	}
}

func waveformUpload(gateway, session string, seq int64, samples, channels int) c.WaveformUpload {
	raw := make([]byte, c.WindowBytes(samples, channels))
	for i := 0; i+1 < len(raw); i += 2 {
		binary.LittleEndian.PutUint16(raw[i:], uint16(i))
	}
	return c.WaveformUpload{SchemaVersion: 1, GatewayID: gateway, GatewaySessionID: session, UploadUptimeMS: 13000, EventSeq: seq, GatewayRxUptimeMS: 12000, DeviceID: fieldDevice, DeviceSequence: pointer(uint32(seq)), BootCount: pointer(uint32(2)), Channels: channels, WindowSamples: samples, SampleRateHz: pointer(999.8), AccelRange: 1, DLPF: 1, BurstProfile: 2, FragmentTotal: c.FragmentCount(samples, channels), FragmentsMissing: []int{}, SamplesB64: base64.StdEncoding.EncodeToString(raw)}
}

func TestWaveformValidation(t *testing.T) {
	base := waveformUpload("5ef49647-9449-4f47-8d1d-c06611c3ba81", fieldSession, 1, 512, 63)
	if _, err := validateWaveform(base); err != nil {
		t.Fatal(err)
	}
	short := base64.StdEncoding.EncodeToString(make([]byte, c.WindowBytes(512, 63)-2))
	long := base64.StdEncoding.EncodeToString(make([]byte, c.WindowBytes(512, 63)+2))
	for name, change := range map[string]func(*c.WaveformUpload){
		"short block": func(w *c.WaveformUpload) { w.SamplesB64 = short },
		"long block":  func(w *c.WaveformUpload) { w.SamplesB64 = long },
		"bad base64":  func(w *c.WaveformUpload) { w.SamplesB64 = "!!!!" },
		"unpadded base64": func(w *c.WaveformUpload) {
			w.SamplesB64 = strings.TrimRight(base64.StdEncoding.EncodeToString(make([]byte, 1)), "=")
		},
		"zero channels":          func(w *c.WaveformUpload) { w.Channels = 0 },
		"mask too wide":          func(w *c.WaveformUpload) { w.Channels = 64 },
		"window size":            func(w *c.WaveformUpload) { w.WindowSamples = 500 },
		"accel range":            func(w *c.WaveformUpload) { w.AccelRange = 4 },
		"gyro range":             func(w *c.WaveformUpload) { w.GyroRange = -1 },
		"filter":                 func(w *c.WaveformUpload) { w.DLPF = 7 },
		"profile":                func(w *c.WaveformUpload) { w.BurstProfile = 4 },
		"fragment total":         func(w *c.WaveformUpload) { w.FragmentTotal = 25 },
		"missing too high":       func(w *c.WaveformUpload) { w.FragmentsMissing = []int{26} },
		"missing repeated":       func(w *c.WaveformUpload) { w.FragmentsMissing = []int{1, 1} },
		"negative missing":       func(w *c.WaveformUpload) { w.FragmentsMissing = []int{-1} },
		"rate":                   func(w *c.WaveformUpload) { w.SampleRateHz = pointer(0.0) },
		"reception after upload": func(w *c.WaveformUpload) { w.GatewayRxUptimeMS = w.UploadUptimeMS + 1 },
		"event":                  func(w *c.WaveformUpload) { w.EventSeq = 0 },
		"device":                 func(w *c.WaveformUpload) { w.DeviceID = "abc" },
	} {
		w := base
		change(&w)
		if _, err := validateWaveform(w); err == nil {
			t.Fatalf("%s accepted", name)
		}
	}
}

func TestWaveformRoundTripIdempotencyAndLimits(t *testing.T) {
	env := newFieldEnv(t)
	path := "/api/v1/ingest/waveforms"
	w := waveformUpload(env.id, fieldSession, 21, 512, 63)
	w.FragmentsMissing = []int{3, 17}
	raw := env.gateway.request("POST", path, w, 200)
	checkSchema(t, "readings-response", raw)
	ack := parse[c.IngestResult](t, raw)
	if len(ack.Acknowledged) != 1 || ack.Acknowledged[0].EventSeq != 21 || ack.Acknowledged[0].Status != "stored" {
		t.Fatalf("ack: %+v", ack)
	}
	// A retry may carry another upload time and is still the same window.
	retry := w
	retry.UploadUptimeMS = 90000
	ack = parse[c.IngestResult](t, env.gateway.request("POST", path, retry, 200))
	if ack.Acknowledged[0].Status != "duplicate" {
		t.Fatalf("retry: %+v", ack)
	}
	conflict := w
	other := make([]byte, c.WindowBytes(512, 63))
	other[0] = 1
	conflict.SamplesB64 = base64.StdEncoding.EncodeToString(other)
	checkSchema(t, "error-response", env.gateway.request("POST", path, conflict, 409))
	list := parse[c.WaveformPage](t, env.user.request("GET", "/api/v1/devices/"+fieldDevice+"/waveforms", nil, 200))
	if len(list.Items) != 1 || list.Items[0].SamplesB64 != "" {
		t.Fatalf("list must not carry samples: %+v", list)
	}
	full := parse[c.Waveform](t, env.user.request("GET", fmt.Sprintf("/api/v1/waveforms/%d", list.Items[0].ID), nil, 200))
	if full.SamplesB64 != w.SamplesB64 || full.FragmentTotal != 26 || len(full.FragmentsMissing) != 2 || full.FragmentsMissing[1] != 17 || full.SampleRateHz != 999.8 || full.BootCount != 2 {
		t.Fatalf("round trip: %+v", full)
	}
	env.user.request("GET", "/api/v1/waveforms/999999", nil, 404)
	env.user.request("GET", "/api/v1/waveforms/abc", nil, 404)
	env.gateway.request("GET", fmt.Sprintf("/api/v1/waveforms/%d", list.Items[0].ID), nil, 401)
	env.user.request("POST", path, w, 401)
	spoof := w
	spoof.GatewayID = "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa"
	env.gateway.request("POST", path, spoof, 403)
	bad := w
	bad.EventSeq = 22
	bad.SamplesB64 = base64.StdEncoding.EncodeToString(make([]byte, 100))
	failure := parse[c.APIError](t, env.gateway.request("POST", path, bad, 422))
	if failure.Error.Details[0].Path != "samples_b64" {
		t.Fatalf("size error: %+v", failure)
	}
	// The largest window fits the 128 KiB limit, and the limit rejects more.
	largest := waveformUpload(env.id, fieldSession, 23, 4096, 63)
	largest.FragmentsMissing = make([]int, 205)
	for i := range largest.FragmentsMissing {
		largest.FragmentsMissing[i] = i
	}
	encoded, _ := json.Marshal(largest)
	if len(encoded) > 128<<10 {
		t.Fatalf("largest window does not fit: %d", len(encoded))
	}
	env.gateway.request("POST", path, largest, 200)
	req, _ := http.NewRequest("POST", env.server.URL+path, strings.NewReader(strings.Repeat(" ", 128<<10)+"{}"))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+env.gateway.token)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != 413 {
		t.Fatalf("128 KiB limit: %d", resp.StatusCode)
	}
	// Exactly at the limit the body is read and judged by its content, not its size.
	req, _ = http.NewRequest("POST", env.server.URL+path, strings.NewReader(strings.Repeat(" ", (128<<10)-2)+"{}"))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+env.gateway.token)
	if resp, err = http.DefaultClient.Do(req); err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != 400 {
		t.Fatalf("at the limit: %d", resp.StatusCode)
	}
	// Other ingest routes keep their limit.
	req, _ = http.NewRequest("POST", env.server.URL+"/api/v1/ingest/readings", strings.NewReader(strings.Repeat(" ", 33<<10)+"{}"))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+env.gateway.token)
	if resp, err = http.DefaultClient.Do(req); err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != 413 {
		t.Fatalf("readings limit: %d", resp.StatusCode)
	}
	// Paging by cursor visits every window once.
	for seq := int64(30); seq < 35; seq++ {
		extra := waveformUpload(env.id, fieldSession, seq, 256, 4)
		stamp := time.Date(2026, 11, 2, 5, 0, int(seq), 0, time.UTC)
		extra.GatewayReceivedAt = &stamp
		env.gateway.request("POST", path, extra, 200)
	}
	seen := map[int64]bool{}
	query := "?limit=2"
	for pages := 0; ; pages++ {
		page := parse[c.WaveformPage](t, env.user.request("GET", "/api/v1/devices/"+fieldDevice+"/waveforms"+query, nil, 200))
		for _, item := range page.Items {
			if seen[item.ID] {
				t.Fatalf("window %d repeated", item.ID)
			}
			seen[item.ID] = true
		}
		if page.NextCursor == "" {
			break
		}
		query = "?limit=2&cursor=" + url.QueryEscape(page.NextCursor)
	}
	if len(seen) != 7 {
		t.Fatalf("windows visited: %d", len(seen))
	}
	from := url.QueryEscape("2026-11-02T05:00:31Z")
	to := url.QueryEscape("2026-11-02T05:00:33Z")
	filtered := parse[c.WaveformPage](t, env.user.request("GET", "/api/v1/devices/"+fieldDevice+"/waveforms?from="+from+"&to="+to, nil, 200))
	if len(filtered.Items) != 2 {
		t.Fatalf("period: %+v", filtered.Items)
	}
	env.user.request("GET", "/api/v1/devices/"+fieldDevice+"/waveforms?cursor=bad", nil, 400)
	env.user.request("GET", "/api/v1/devices/FFFFFFFF/waveforms", nil, 404)
}

func TestSyncGatewayConfigAndPasswords(t *testing.T) {
	env := newFieldEnv(t)
	path := "/api/v1/gateways/" + env.id + "/config"
	first := env.sync(0)
	if first.GatewayConfig != nil || len(first.Devices) != 0 || first.Firmware != nil || first.GatewayID != env.id || first.GatewaySessionID != fieldSession {
		t.Fatalf("empty sync: %+v", first)
	}
	if !regexp.MustCompile(`^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}\.\d{3}Z$`).MatchString(first.ServerTime) {
		t.Fatalf("server_time %q", first.ServerTime)
	}
	empty := parse[c.GatewayConfigState](t, env.user.request("GET", path, nil, 200))
	if empty.Config != nil || empty.State != nil || empty.Defaults.TxPowerDBm != 17 || !empty.Defaults.APWhenOffline || empty.Defaults.HeartbeatIntervalS != 30 || *empty.ReportedVersion != 0 {
		t.Fatalf("no configuration: %+v", empty)
	}
	raw := env.user.request("PUT", path, map[string]any{"wifi": []map[string]any{{"ssid": "rede", "password": wifiSecret}, {"ssid": "aberta", "password": ""}}, "sync_interval_s": 7}, 200)
	if strings.Contains(string(raw), wifiSecret) {
		t.Fatal("password returned by PUT")
	}
	saved := parse[c.GatewayConfigState](t, raw)
	if saved.Config.Version != 1 || saved.Config.TxPowerDBm != 17 || saved.Config.SyncIntervalS != 7 || !saved.Config.Wifi[0].HasPassword || saved.Config.Wifi[1].HasPassword || stateOf(saved.State) != c.StatePending {
		t.Fatalf("saved: %+v", saved)
	}
	raw = env.user.request("GET", path, nil, 200)
	if strings.Contains(string(raw), wifiSecret) || strings.Contains(string(raw), "sealed") || !strings.Contains(string(raw), `"has_password":true`) {
		t.Fatalf("GET leaked or lost password state: %s", raw)
	}
	// At rest the password is sealed, with a different ciphertext from its plaintext.
	var stored string
	if err := env.pool.QueryRow(context.Background(), `SELECT config::text FROM gateway_configs WHERE gateway_id=$1`, env.id).Scan(&stored); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(stored, wifiSecret) || !strings.Contains(stored, `"sealed"`) {
		t.Fatalf("stored in the clear: %s", stored)
	}
	// Delivered only when the version differs.
	got := env.sync(0)
	if got.GatewayConfig == nil || got.GatewayConfig.Version != 1 || got.GatewayConfig.Wifi[0].Password != wifiSecret || got.GatewayConfig.Wifi[1].Password != "" || got.GatewayConfig.TxPowerDBm != 17 || got.GatewayConfig.SyncIntervalS != 7 {
		t.Fatalf("delivery: %+v", got.GatewayConfig)
	}
	if got = env.sync(1); got.GatewayConfig != nil {
		t.Fatalf("delivered with the same version: %+v", got.GatewayConfig)
	}
	state := parse[c.GatewayConfigState](t, env.user.request("GET", path, nil, 200))
	if stateOf(state.State) != c.StateApplied || *state.ReportedVersion != 1 || state.AppliedAt == nil || state.LastSyncAt == nil {
		t.Fatalf("applied: %+v", state)
	}
	// Only the gateway token reaches the sync route, and only for its own identity.
	env.user.request("POST", "/api/v1/gateway/sync", c.SyncRequest{SchemaVersion: 1, GatewayID: env.id, GatewaySessionID: fieldSession, FirmwareVersion: "x", Devices: []c.SyncDevice{}}, 401)
	env.gateway.request("GET", path, nil, 401)
	env.gateway.request("POST", "/api/v1/gateway/sync", c.SyncRequest{SchemaVersion: 1, GatewayID: "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa", GatewaySessionID: fieldSession, FirmwareVersion: "x", Devices: []c.SyncDevice{}}, 403)
	env.gateway.request("POST", "/api/v1/gateway/sync", c.SyncRequest{SchemaVersion: 2, GatewayID: env.id, GatewaySessionID: fieldSession, FirmwareVersion: "x", Devices: []c.SyncDevice{}}, 422)
	// An absent password keeps the stored one; repeating the form is not an alteration.
	same := parse[c.GatewayConfigState](t, env.user.request("PUT", path, map[string]any{"wifi": []map[string]any{{"ssid": "rede"}, {"ssid": "aberta", "password": ""}}, "sync_interval_s": 7}, 200))
	if same.Config.Version != 1 {
		t.Fatalf("no change, version %d", same.Config.Version)
	}
	retyped := parse[c.GatewayConfigState](t, env.user.request("PUT", path, map[string]any{"wifi": []map[string]any{{"ssid": "rede", "password": wifiSecret}, {"ssid": "aberta", "password": ""}}, "sync_interval_s": 7}, 200))
	if retyped.Config.Version != 1 {
		t.Fatalf("same password, version %d", retyped.Config.Version)
	}
	changed := parse[c.GatewayConfigState](t, env.user.request("PUT", path, map[string]any{"wifi": []map[string]any{{"ssid": "rede"}}, "sync_interval_s": 9, "tx_power_dbm": 12}, 200))
	if changed.Config.Version != 2 || stateOf(changed.State) != c.StatePending || changed.AppliedAt != nil || changed.Config.TxPowerDBm != 12 {
		t.Fatalf("changed: %+v", changed)
	}
	if got = env.sync(1); got.GatewayConfig == nil || got.GatewayConfig.Wifi[0].Password != wifiSecret || got.GatewayConfig.SyncIntervalS != 9 || got.GatewayConfig.TxPowerDBm != 12 {
		t.Fatalf("kept password lost: %+v", got.GatewayConfig)
	}
	// An unknown network without a password is an open one; an empty password clears it.
	cleared := parse[c.GatewayConfigState](t, env.user.request("PUT", path, map[string]any{"wifi": []map[string]any{{"ssid": "rede", "password": ""}, {"ssid": "nova"}}}, 200))
	if cleared.Config.Wifi[0].HasPassword || cleared.Config.Wifi[1].HasPassword {
		t.Fatalf("cleared: %+v", cleared.Config.Wifi)
	}
	// Without the key, the clock and the devices still flow; the configuration waits.
	env.user.request("PUT", path, map[string]any{"wifi": []map[string]any{{"ssid": "rede", "password": wifiSecret}}}, 200)
	other, _ := secure.NewBox(bytes.Repeat([]byte{1}, 32))
	env.db.UseSealer(other)
	if got = env.sync(0); got.GatewayConfig != nil || got.ServerTime == "" {
		t.Fatalf("wrong key: %+v", got)
	}
	env.user.request("GET", "/api/v1/gateways/aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa/config", nil, 404)
}

func TestCommandsLifecycle(t *testing.T) {
	env := newFieldEnv(t)
	path := "/api/v1/devices/" + fieldDevice + "/commands"
	env.user.request("POST", path, c.DeviceCommandInput{Kind: c.CommandReboot}, 404)
	env.device()
	env.user.request("POST", path, c.DeviceCommandInput{Kind: "explode"}, 422)
	first := parse[c.DeviceCommand](t, env.user.request("POST", path, c.DeviceCommandInput{Kind: c.CommandCaptureNow}, 201))
	second := parse[c.DeviceCommand](t, env.user.request("POST", path, c.DeviceCommandInput{Kind: c.CommandReboot}, 201))
	if first.Seq != 1 || second.Seq != 2 || first.State != c.StatePending {
		t.Fatalf("seq: %+v %+v", first, second)
	}
	// The oldest unconfirmed command goes first, with no configuration attached.
	got := env.sync(0)
	if len(got.Devices) != 1 || got.Devices[0].Config != nil || got.Devices[0].Command == nil || got.Devices[0].Command.Seq != 1 || got.Devices[0].Command.Kind != c.CommandCaptureNow {
		t.Fatalf("sync: %+v", got.Devices)
	}
	env.sync(0, c.SyncDevice{DeviceID: fieldDevice, RadiatedCommandSeq: pointer(1)})
	list := parse[c.DeviceCommandList](t, env.user.request("GET", path, nil, 200))
	if list.Items[0].Seq != 2 || list.Items[0].State != c.StatePending || list.Items[1].State != c.StateRadiated || list.Items[1].RadiatedAt == nil || list.Items[1].AppliedAt != nil {
		t.Fatalf("radiated: %+v", list)
	}
	// A radiated command keeps coming until the telemetry confirms it.
	if got = env.sync(0); got.Devices[0].Command.Seq != 1 {
		t.Fatalf("radiated command dropped: %+v", got.Devices)
	}
	r := env.reading(1)
	r.CommandSeq = pointer(uint16(0))
	env.send(r)
	if list = parse[c.DeviceCommandList](t, env.user.request("GET", path, nil, 200)); list.Items[1].State != c.StateRadiated {
		t.Fatalf("echo 0: %+v", list)
	}
	r = env.reading(2)
	r.CommandSeq = pointer(uint16(1))
	env.send(r)
	list = parse[c.DeviceCommandList](t, env.user.request("GET", path, nil, 200))
	if list.Items[1].State != c.StateApplied || list.Items[1].AppliedAt == nil || list.Items[0].State != c.StatePending {
		t.Fatalf("applied: %+v", list)
	}
	if got = env.sync(0); got.Devices[0].Command.Seq != 2 {
		t.Fatalf("next command: %+v", got.Devices)
	}
	// An echo at or above a seq applies it, even if no radiated report arrived.
	r = env.reading(3)
	r.CommandSeq = pointer(uint16(5))
	env.send(r)
	list = parse[c.DeviceCommandList](t, env.user.request("GET", path, nil, 200))
	if list.Items[0].State != c.StateApplied || list.Items[0].RadiatedAt != nil {
		t.Fatalf("echo above: %+v", list)
	}
	if got = env.sync(0); len(got.Devices) != 0 {
		t.Fatalf("nothing left: %+v", got.Devices)
	}
	// A new seq stays above what the transmitter already executed, or it would be ignored.
	third := parse[c.DeviceCommand](t, env.user.request("POST", path, c.DeviceCommandInput{Kind: c.CommandReboot}, 201))
	if third.Seq != 6 || third.State != c.StatePending {
		t.Fatalf("seq after echo: %+v", third)
	}
	// The range ends at 65535 and does not wrap, because >= decides the confirmation.
	if _, err := env.pool.Exec(context.Background(), `UPDATE device_configs SET reported_command_seq=65535 WHERE device_id=$1`, fieldDevice); err != nil {
		t.Fatal(err)
	}
	raw := env.user.request("POST", path, c.DeviceCommandInput{Kind: c.CommandReboot}, 409)
	checkSchema(t, "error-response", raw)
	env.gateway.request("GET", path, nil, 401)
}

func TestFactoryResetReplacesTheConfiguration(t *testing.T) {
	env := newFieldEnv(t)
	env.device()
	config := "/api/v1/devices/" + fieldDevice + "/config"
	commands := "/api/v1/devices/" + fieldDevice + "/commands"
	// Without a configuration on record, the reset still defines one.
	env.user.request("POST", commands, c.DeviceCommandInput{Kind: c.CommandFactoryReset}, 201)
	state := env.config()
	if state.Config == nil || state.Config.Version != 1 || state.Config.CycleIntervalS != 11 || stateOf(state.State) != c.StatePending {
		t.Fatalf("reset without configuration: %+v", state)
	}
	env.user.request("PUT", config, map[string]any{"cycle_interval_s": 40, "window_samples": 1024}, 200)
	r := env.reading(1)
	r.ConfigVersion = pointer(uint16(2))
	env.send(r)
	if state = env.config(); stateOf(state.State) != c.StateApplied {
		t.Fatalf("state: %+v", state)
	}
	command := parse[c.DeviceCommand](t, env.user.request("POST", commands, c.DeviceCommandInput{Kind: c.CommandFactoryReset}, 201))
	if command.Kind != c.CommandFactoryReset {
		t.Fatalf("command: %+v", command)
	}
	state = env.config()
	want := c.DefaultDeviceConfig()
	want.Version = 3
	first, _ := json.Marshal(state.Config)
	second, _ := json.Marshal(want)
	if !bytes.Equal(first, second) || stateOf(state.State) != c.StatePending || state.AppliedAt != nil {
		t.Fatalf("reset: %s %+v", first, state)
	}
	got := env.sync(0)
	if len(got.Devices) != 1 || got.Devices[0].Config.Version != 3 || got.Devices[0].Config.WindowSamples != 512 || got.Devices[0].Command.Kind != c.CommandFactoryReset {
		t.Fatalf("sync: %+v", got.Devices)
	}
	// Even when the desired configuration already is the default, the version is new.
	env.user.request("POST", commands, c.DeviceCommandInput{Kind: c.CommandFactoryReset}, 201)
	if state = env.config(); state.Config.Version != 4 {
		t.Fatalf("version: %+v", state.Config)
	}
	// The device comes back at factory version 0: not applied, and the defaults go out.
	r = env.reading(2)
	r.ConfigVersion = pointer(uint16(0))
	env.send(r)
	if state = env.config(); stateOf(state.State) == c.StateApplied {
		t.Fatalf("factory version applied: %+v", state)
	}
	// Other commands leave the configuration alone.
	env.user.request("POST", commands, c.DeviceCommandInput{Kind: c.CommandReboot}, 201)
	if state = env.config(); state.Config.Version != 4 {
		t.Fatalf("reboot changed the configuration: %+v", state.Config)
	}
}

func firmwareBinary(n int) []byte {
	data := bytes.Repeat([]byte{0x5A}, n)
	data[0] = espImageMagic
	return data
}

func (e *fieldEnv) upload(gateway, version string, data []byte, contentType string) (*http.Response, []byte) {
	e.user.t.Helper()
	req, _ := http.NewRequest("POST", e.server.URL+"/api/v1/gateways/"+gateway+"/firmware?version="+url.QueryEscape(version), bytes.NewReader(data))
	req.Header.Set("Content-Type", contentType)
	req.Header.Set("Authorization", "Bearer "+e.user.token)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		e.user.t.Fatal(err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	return resp, raw
}

func TestFirmwareUpdate(t *testing.T) {
	env := newFieldEnv(t)
	data := firmwareBinary(4096)
	sum := sha256.Sum256(data)
	if resp, _ := env.upload(env.id, "gateway-2.1.0", data, "text/plain"); resp.StatusCode != 400 {
		t.Fatalf("content type: %d", resp.StatusCode)
	}
	if resp, _ := env.upload(env.id, "", data, "application/octet-stream"); resp.StatusCode != 422 {
		t.Fatalf("version: %d", resp.StatusCode)
	}
	if resp, _ := env.upload(env.id, "x", []byte("not an image"), "application/octet-stream"); resp.StatusCode != 422 {
		t.Fatalf("magic: %d", resp.StatusCode)
	}
	if resp, _ := env.upload(env.id, "x", firmwareBinary(maxFirmwareBytes+1), "application/octet-stream"); resp.StatusCode != 413 {
		t.Fatalf("size: %d", resp.StatusCode)
	}
	if resp, _ := env.upload("aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa", "x", data, "application/octet-stream"); resp.StatusCode != 404 {
		t.Fatalf("gateway: %d", resp.StatusCode)
	}
	resp, raw := env.upload(env.id, "gateway-2.1.0", data, "application/octet-stream")
	if resp.StatusCode != 201 {
		t.Fatalf("upload: %d %s", resp.StatusCode, raw)
	}
	item := parse[c.GatewayFirmware](t, raw)
	if item.SHA256 != hex.EncodeToString(sum[:]) || item.Size != 4096 || item.State != c.FirmwarePending {
		t.Fatalf("firmware: %+v", item)
	}
	got := env.sync(0)
	if got.Firmware == nil || got.Firmware.Version != "gateway-2.1.0" || got.Firmware.SHA256 != item.SHA256 || got.Firmware.Size != 4096 || got.Firmware.Path != "/api/v1/gateway/firmware/"+item.ID {
		t.Fatalf("offer: %+v", got.Firmware)
	}
	status := parse[c.GatewayFirmwareStatus](t, env.user.request("GET", "/api/v1/gateways/"+env.id+"/firmware", nil, 200))
	if status.CurrentVersion == nil || *status.CurrentVersion != "gateway-2.0.0" || status.Items[0].State != c.FirmwareOffered {
		t.Fatalf("status: %+v", status)
	}
	// The gateway downloads with its own token; nobody else does.
	if body := env.gateway.request("GET", got.Firmware.Path, nil, 200); !bytes.Equal(body, data) {
		t.Fatal("binary changed")
	}
	env.user.request("GET", got.Firmware.Path, nil, 401)
	other, otherToken, err := env.db.CreateGateway(context.Background(), c.GatewayInput{Name: "Other"})
	if err != nil || other.ID == "" {
		t.Fatal(err)
	}
	client{t: t, base: env.server.URL, token: otherToken}.request("GET", got.Firmware.Path, nil, 404)
	env.gateway.request("GET", "/api/v1/gateway/firmware/missing", nil, 404)
	// A resumable download.
	req, _ := http.NewRequest("GET", env.server.URL+got.Firmware.Path, nil)
	req.Header.Set("Authorization", "Bearer "+env.gateway.token)
	req.Header.Set("Range", "bytes=100-199")
	partial, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	part, _ := io.ReadAll(partial.Body)
	partial.Body.Close()
	if partial.StatusCode != 206 || len(part) != 100 {
		t.Fatalf("range: %d %d", partial.StatusCode, len(part))
	}
	status = parse[c.GatewayFirmwareStatus](t, env.user.request("GET", "/api/v1/gateways/"+env.id+"/firmware", nil, 200))
	if status.Items[0].State != c.FirmwareDownloaded || status.Items[0].DownloadedAt == nil {
		t.Fatalf("status: %+v", status.Items[0])
	}
	// A failed update stops being offered, or the gateway would fail in a loop.
	failure := c.SyncRequest{SchemaVersion: 1, GatewayID: env.id, GatewaySessionID: fieldSession, FirmwareVersion: "gateway-2.0.0", OTAResult: &c.OTAResult{Version: "gateway-2.1.0", OK: false, Reason: "boot not confirmed"}, Devices: []c.SyncDevice{}}
	failed := parse[c.SyncResponse](t, env.gateway.request("POST", "/api/v1/gateway/sync", failure, 200))
	if failed.Firmware != nil {
		t.Fatalf("offered after failure: %+v", failed.Firmware)
	}
	status = parse[c.GatewayFirmwareStatus](t, env.user.request("GET", "/api/v1/gateways/"+env.id+"/firmware", nil, 200))
	if status.Items[0].State != c.FirmwareFailed || status.Items[0].Reason != "boot not confirmed" || status.Items[0].FinishedAt == nil {
		t.Fatalf("failed: %+v", status.Items[0])
	}
	env.gateway.request("GET", got.Firmware.Path, nil, 404)
	// A new upload replaces the target; running that version closes the update.
	_, raw = env.upload(env.id, "gateway-2.2.0", data, "application/octet-stream")
	second := parse[c.GatewayFirmware](t, raw)
	_, raw = env.upload(env.id, "gateway-2.3.0", data, "application/octet-stream")
	third := parse[c.GatewayFirmware](t, raw)
	if got = env.sync(0); got.Firmware.Version != "gateway-2.3.0" {
		t.Fatalf("offer: %+v", got.Firmware)
	}
	env.gateway.request("GET", "/api/v1/gateway/firmware/"+second.ID, nil, 404)
	running := parse[c.SyncResponse](t, env.gateway.request("POST", "/api/v1/gateway/sync", c.SyncRequest{SchemaVersion: 1, GatewayID: env.id, GatewaySessionID: fieldSession, FirmwareVersion: "gateway-2.3.0", Devices: []c.SyncDevice{}}, 200))
	if running.Firmware != nil {
		t.Fatalf("offered the running version: %+v", running.Firmware)
	}
	status = parse[c.GatewayFirmwareStatus](t, env.user.request("GET", "/api/v1/gateways/"+env.id+"/firmware", nil, 200))
	if status.Items[0].ID != third.ID || status.Items[0].State != c.FirmwareApplied || status.Items[1].State != c.FirmwareSuperseded || *status.CurrentVersion != "gateway-2.3.0" {
		t.Fatalf("applied: %+v", status)
	}
}

func TestCollectionsAndSummary(t *testing.T) {
	env := newFieldEnv(t)
	env.user.request("POST", "/api/v1/collections", c.CollectionInput{Name: "x", DeviceID: fieldDevice}, 404)
	env.device()
	env.user.request("POST", "/api/v1/collections", c.CollectionInput{Name: " ", DeviceID: fieldDevice}, 422)
	env.user.request("POST", "/api/v1/collections", c.CollectionInput{Name: "x", DeviceID: "abc"}, 422)
	start := time.Date(2026, 11, 2, 5, 0, 0, 0, time.UTC)
	created := parse[c.Collection](t, env.user.request("POST", "/api/v1/collections", c.CollectionInput{Name: " Motor carregado ", DeviceID: fieldDevice, Condition: "Carga", Notes: "Ensaio", StartedAt: &start}, 201))
	if created.Name != "Motor carregado" || created.EndedAt != nil || !created.StartedAt.Equal(start) {
		t.Fatalf("created: %+v", created)
	}
	now := parse[c.Collection](t, env.user.request("POST", "/api/v1/collections", c.CollectionInput{Name: "Agora", DeviceID: fieldDevice}, 201))
	if time.Since(now.StartedAt) > time.Minute {
		t.Fatalf("server start: %+v", now)
	}
	list := parse[c.CollectionList](t, env.user.request("GET", "/api/v1/collections", nil, 200))
	if len(list.Items) != 2 {
		t.Fatalf("list: %+v", list)
	}
	path := fmt.Sprintf("/api/v1/collections/%d", created.ID)
	// No data yet: zero counts and no invented link quality.
	empty := parse[c.CollectionSummary](t, env.user.request("GET", path+"/summary", nil, 200))
	if empty.Readings != 0 || empty.RSSIDBm != nil || empty.SNRDB != nil {
		t.Fatalf("empty: %+v", empty)
	}
	stamp := func(seconds int) *time.Time { value := start.Add(time.Duration(seconds) * time.Second); return &value }
	mk := func(event int64, boot, sequence uint32, seconds int, flags uint8, rssi float64) c.Reading {
		r := env.reading(event)
		r.GatewayRxUptimeMS = 1000
		r.BootCount, r.DeviceSequence = pointer(boot), pointer(sequence)
		r.GatewayReceivedAt, r.Flags, r.RSSIDBm = stamp(seconds), pointer(flags), pointer(rssi)
		return r
	}
	// Boot 2: 1,2,5 (one gap, two lost). Boot 3: 1,2. Saturated and unmeasured readings count.
	env.send(mk(1, 2, 1, 1, 0, -40), mk(2, 2, 2, 2, c.FlagSaturated, -50), mk(3, 2, 5, 3, 0, -60), mk(4, 3, 1, 4, c.FlagNoMeasurement, -70), mk(5, 3, 2, 5, 0, -80))
	// Outside the interval.
	env.send(mk(6, 3, 3, -60, 0, -10))
	withWindow := func(event int64, seconds int, missing []int) {
		w := waveformUpload(env.id, fieldSession, event, 512, 63)
		w.GatewayReceivedAt, w.FragmentsMissing = stamp(seconds), missing
		env.gateway.request("POST", "/api/v1/ingest/waveforms", w, 200)
	}
	withWindow(1, 1, []int{3, 17})
	withWindow(2, 2, []int{})
	withWindow(9, -60, []int{1})
	// Close the collection with an end after those readings.
	end := start.Add(10 * time.Second)
	env.user.request("PUT", path, c.CollectionInput{Name: "x", EndedAt: pointer(start.Add(-time.Hour))}, 422)
	env.user.request("PUT", path, c.CollectionInput{Name: "x", DeviceID: "AAAAAAAA", EndedAt: &end}, 422)
	env.user.request("PUT", "/api/v1/collections/999999", c.CollectionInput{Name: "x"}, 404)
	closed := parse[c.Collection](t, env.user.request("PUT", path, c.CollectionInput{Name: "Motor carregado", Condition: "Carga", Notes: "Fim", EndedAt: &end}, 200))
	if closed.EndedAt == nil || !closed.EndedAt.Equal(end) || closed.Notes != "Fim" || closed.DeviceID != fieldDevice || !closed.StartedAt.Equal(start) {
		t.Fatalf("closed: %+v", closed)
	}
	summary := parse[c.CollectionSummary](t, env.user.request("GET", path+"/summary", nil, 200))
	if summary.Readings != 5 || summary.SequenceGaps != 1 || summary.SequenceMissing != 2 || summary.ReadingsSaturated != 1 || summary.ReadingsWithoutMeasurement != 1 {
		t.Fatalf("readings: %+v", summary)
	}
	if summary.Waveforms != 2 || summary.WaveformsIncomplete != 1 || summary.FragmentsTotal != 52 || summary.FragmentsMissing != 2 {
		t.Fatalf("windows: %+v", summary)
	}
	if summary.RSSIDBm == nil || summary.RSSIDBm.Min != -80 || summary.RSSIDBm.Max != -40 || summary.RSSIDBm.Avg != -60 || summary.SNRDB.Avg != 9 {
		t.Fatalf("link: %+v %+v", summary.RSSIDBm, summary.SNRDB)
	}
	env.user.request("GET", "/api/v1/collections/999999/summary", nil, 404)
	env.user.request("GET", "/api/v1/collections/abc/summary", nil, 404)
	env.gateway.request("GET", "/api/v1/collections", nil, 401)
}

func TestSequenceGaps(t *testing.T) {
	for _, tc := range []struct {
		name          string
		sequence      []int64
		sameBoot      bool
		gaps, missing int64
	}{
		{"none", []int64{1, 2, 3}, true, 0, 0},
		{"empty", nil, true, 0, 0},
		{"one lost", []int64{1, 3}, true, 1, 1},
		{"late arrival within a boot is no gap", []int64{1, 3, 2, 4}, true, 0, 0},
		{"duplicate", []int64{1, 1, 2}, true, 0, 0},
		{"reset without boot counter is no gap", []int64{100, 101, 1, 2}, false, 0, 0},
		{"forward jump without boot counter", []int64{1, 2, 10}, false, 1, 7},
	} {
		if gaps, missing := store.SequenceGaps(tc.sequence, tc.sameBoot); gaps != tc.gaps || missing != tc.missing {
			t.Fatalf("%s: %d %d", tc.name, gaps, missing)
		}
	}
}

func TestConfigStateTable(t *testing.T) {
	v := func(n int) *int { return &n }
	for _, tc := range []struct {
		desired, radiated, reported *int
		want                        string
	}{
		{nil, nil, v(3), ""},
		{v(5), nil, nil, c.StatePending},
		{v(5), v(4), v(4), c.StatePending},
		{v(5), v(5), v(4), c.StateRadiated},
		{v(5), v(5), v(5), c.StateApplied},
		{v(5), nil, v(5), c.StateApplied},
		{v(5), v(4), v(5), c.StateApplied},
		{v(5), v(5), v(6), c.StateRadiated},
	} {
		if got := stateOf(store.ConfigState(tc.desired, tc.radiated, tc.reported)); got != tc.want {
			t.Fatalf("%+v: %q", tc, got)
		}
	}
}

func TestUserSessionLastsThirtyDays(t *testing.T) {
	env := newFieldEnv(t)
	_, expiry, err := env.db.Login(context.Background(), "field@example.com", "field-test-password-123")
	if err != nil {
		t.Fatal(err)
	}
	if left := time.Until(expiry); left < 29*24*time.Hour || left > 30*24*time.Hour+time.Minute {
		t.Fatalf("session lifetime: %s", left)
	}
}

func TestTLSFingerprintRoute(t *testing.T) {
	env := newFieldEnv(t)
	out := parse[c.TLSFingerprint](t, env.user.request("GET", "/api/v1/tls/fingerprint", nil, 200))
	if out.SHA256 != strings.Repeat("ab", 32) {
		t.Fatalf("fingerprint: %+v", out)
	}
	env.gateway.request("GET", "/api/v1/tls/fingerprint", nil, 401)
}

// A database stopped at migration 004 upgrades in place, and a retry of a reading stored
// before the upgrade is still recognized.
func TestMigrationFrom004KeepsDataAndIdempotency(t *testing.T) {
	db, dsn := testDatabase(t)
	db.Close()
	ctx := context.Background()
	config, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		t.Fatal(err)
	}
	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	if _, err = pool.Exec(ctx, `DROP TABLE device_commands,device_configs,gateway_configs,waveforms,collections,gateway_firmware`); err != nil {
		t.Fatal(err)
	}
	if _, err = pool.Exec(ctx, `DELETE FROM schema_migrations WHERE version=5`); err != nil {
		t.Fatal(err)
	}
	const gateway = "5ef49647-9449-4f47-8d1d-c06611c3ba81"
	if _, err = pool.Exec(ctx, `INSERT INTO gateways(id,name,token_hash) VALUES($1,'old',$2)`, gateway, sha256Bytes("token")); err != nil {
		t.Fatal(err)
	}
	if _, err = pool.Exec(ctx, `INSERT INTO devices(id) VALUES('D6F61B44')`); err != nil {
		t.Fatal(err)
	}
	var old c.ReadingBatch
	if err = json.Unmarshal(contractFile(t, "examples/readings-request.json"), &old); err != nil {
		t.Fatal(err)
	}
	// The payload exactly as version 004 wrote it.
	const payload = `{"event_seq":1,"gateway_rx_uptime_ms":12000,"gateway_received_at":null,"packet_version":2,"device_id":"D6F61B44","device_sequence":1,"state":"UNKNOWN","diag_code":0,"rms_g":0.0035979999229311943,"temperature_c":null,"rssi_dbm":-55,"snr_db":13.5,"packet_crc16":33642}`
	if _, err = pool.Exec(ctx, `INSERT INTO readings(gateway_id,gateway_session_id,event_seq,device_id,estimated_received_at,queue_delay_ms,payload) VALUES($1,$2,1,'D6F61B44',now(),1000,$3::jsonb)`, gateway, old.GatewaySessionID, payload); err != nil {
		t.Fatal(err)
	}
	upgraded, err := store.Open(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer upgraded.Close()
	result, err := upgraded.Ingest(ctx, old)
	if err != nil {
		t.Fatal(err)
	}
	if result.Acknowledged[0].Status != "duplicate" {
		t.Fatalf("retry after the upgrade: %+v", result)
	}
	page, err := upgraded.History(ctx, "D6F61B44", store.HistoryFilter{Limit: 10})
	if err != nil || len(page.Items) != 1 || page.Items[0].QueueDelayMS != 1000 {
		t.Fatalf("old data: %+v %v", page, err)
	}
	if _, err = upgraded.DeviceConfig(ctx, "D6F61B44"); err != nil {
		t.Fatal(err)
	}
}

func sha256Bytes(value string) []byte { sum := sha256.Sum256([]byte(value)); return sum[:] }

// The network listener serves gateways and nothing else, over a connection that is reused.
// None of these paths reach the database, so this runs without one.
func TestGatewayListenerRefusesUserRoutes(t *testing.T) {
	dir := t.TempDir()
	certificate, fingerprint, err := secure.LoadOrCreateCertificate(dir)
	if err != nil {
		t.Fatal(err)
	}
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	server := GatewayServer("", NewGateway(nil, quietLog), certificate)
	go server.ServeTLS(listener, "", "")
	t.Cleanup(func() { server.Close() })
	// The way the firmware checks the peer: no CA, only the pinned fingerprint.
	pinned := &tls.Config{InsecureSkipVerify: true, VerifyPeerCertificate: func(raw [][]byte, _ [][]*x509.Certificate) error {
		sum := sha256.Sum256(raw[0])
		if hex.EncodeToString(sum[:]) != fingerprint {
			return errors.New("fingerprint mismatch")
		}
		return nil
	}}
	transport := &http.Transport{TLSClientConfig: pinned}
	t.Cleanup(transport.CloseIdleConnections)
	tlsClient := &http.Client{Transport: transport, Timeout: 5 * time.Second}
	base := "https://" + listener.Addr().String()
	status := func(method, path string) int {
		t.Helper()
		req, _ := http.NewRequest(method, base+path, strings.NewReader("{}"))
		req.Header.Set("Content-Type", "application/json")
		resp, err := tlsClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		_, _ = io.Copy(io.Discard, resp.Body)
		return resp.StatusCode
	}
	if got := status("GET", "/healthz"); got != 200 {
		t.Fatalf("healthz: %d", got)
	}
	for _, route := range [][2]string{{"POST", "/api/v1/auth/login"}, {"POST", "/api/v1/auth/logout"}, {"GET", "/readyz"}, {"GET", "/api/v1/devices"}, {"POST", "/api/v1/devices"}, {"GET", "/api/v1/motors"}, {"GET", "/api/v1/gateways"}, {"POST", "/api/v1/gateways"}, {"GET", "/api/v1/readings/recent"}, {"GET", "/api/v1/devices/AB12CD34/config"}, {"PUT", "/api/v1/devices/AB12CD34/config"}, {"POST", "/api/v1/devices/AB12CD34/commands"}, {"GET", "/api/v1/devices/AB12CD34/waveforms"}, {"GET", "/api/v1/waveforms/1"}, {"GET", "/api/v1/gateways/x/config"}, {"PUT", "/api/v1/gateways/x/config"}, {"POST", "/api/v1/gateways/x/firmware"}, {"GET", "/api/v1/gateways/x/logs"}, {"POST", "/api/v1/gateways/x/rotate-token"}, {"GET", "/api/v1/collections"}, {"POST", "/api/v1/collections"}, {"GET", "/api/v1/tls/fingerprint"}} {
		if got := status(route[0], route[1]); got != 404 {
			t.Fatalf("%s %s reachable over the network: %d", route[0], route[1], got)
		}
	}
	for _, route := range [][2]string{{"POST", "/api/v1/ingest/readings"}, {"POST", "/api/v1/ingest/heartbeat"}, {"POST", "/api/v1/ingest/logs"}, {"POST", "/api/v1/ingest/waveforms"}, {"POST", "/api/v1/gateway/sync"}, {"GET", "/api/v1/gateway/firmware/x"}} {
		if got := status(route[0], route[1]); got != 401 {
			t.Fatalf("%s %s must be a gateway route: %d", route[0], route[1], got)
		}
	}
	// Reuse is what spares the gateway a handshake on every 5 s poll.
	reused := 0
	for range 5 {
		req, _ := http.NewRequest("GET", base+"/healthz", nil)
		req = req.WithContext(httptrace.WithClientTrace(req.Context(), &httptrace.ClientTrace{GotConn: func(info httptrace.GotConnInfo) {
			if info.Reused {
				reused++
			}
		}}))
		resp, err := tlsClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		_, _ = io.Copy(io.Discard, resp.Body)
		resp.Body.Close()
		if resp.ProtoMajor != 1 {
			t.Fatalf("protocol %s", resp.Proto)
		}
	}
	if reused != 5 {
		t.Fatalf("connections reused: %d of 5", reused)
	}
	if server.IdleTimeout <= 60*time.Second {
		t.Fatalf("idle limit %s does not outlast the longest sync interval", server.IdleTimeout)
	}
	// A client pinning another fingerprint never completes the handshake.
	wrong := &http.Client{Transport: &http.Transport{TLSClientConfig: &tls.Config{InsecureSkipVerify: true, VerifyPeerCertificate: func([][]byte, [][]*x509.Certificate) error { return errors.New("mismatch") }}}, Timeout: 5 * time.Second}
	if _, err = wrong.Get(base + "/healthz"); err == nil {
		t.Fatal("pinned client accepted another certificate")
	}
	// Reuses the certificate after a restart, so the pin keeps working.
	if _, again, err := secure.LoadOrCreateCertificate(dir); err != nil || again != fingerprint {
		t.Fatalf("reload: %v %s", err, again)
	}
	if _, err = os.Stat(filepath.Join(dir, secure.PrivateKeyFile)); err != nil {
		t.Fatal(err)
	}
}
