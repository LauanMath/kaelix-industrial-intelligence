package main

import (
	"context"
	"encoding/base64"
	"encoding/binary"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	c "kaelix/contracts"
	"kaelix/internal/apiclient"
	"kaelix/internal/gatewaylink"
	"kaelix/internal/store"
)

func testApp(t *testing.T, base string) *App {
	t.Helper()
	db, err := store.Open(filepath.Join(t.TempDir(), "kaelix.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	cl, err := apiclient.New(base, strings.Repeat("u", 43))
	if err != nil {
		t.Fatal(err)
	}
	a := NewApp()
	a.ctx, a.store, a.client, a.apiCtx = context.Background(), db, cl, context.Background()
	a.connection = ConnectionInfo{Endpoint: cl.BaseURL, Authenticated: true}
	return a
}

func readCSV(t *testing.T, path string) [][]string {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(string(raw), "\xef\xbb\xbf") {
		t.Fatalf("%s has no byte order mark", path)
	}
	reader := csv.NewReader(strings.NewReader(strings.TrimPrefix(string(raw), "\xef\xbb\xbf")))
	reader.Comma = ';'
	rows, err := reader.ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	return rows
}

// A fake API with more data than one page, to prove the export follows every cursor.
func exportServer(t *testing.T, readings, windows int) *httptest.Server {
	t.Helper()
	const device = "AB12CD34"
	start := time.Date(2026, 11, 2, 5, 0, 0, 0, time.UTC)
	ended := start.Add(time.Hour)
	raw := make([]byte, c.WindowBytes(256, 7))
	for i := 0; i+6 <= len(raw); i += 6 {
		binary.LittleEndian.PutUint16(raw[i:], uint16(int16(16384)))
		negative := int16(-8192)
		binary.LittleEndian.PutUint16(raw[i+2:], uint16(negative))
		binary.LittleEndian.PutUint16(raw[i+4:], uint16(int16(i/6)))
	}
	window := func(id int64) c.Waveform {
		at := start.Add(time.Duration(id) * time.Second)
		return c.Waveform{ID: id, GatewayID: "g", GatewaySessionID: "s", EventSeq: id, DeviceID: device, WaveformMetadata: c.WaveformMetadata{Channels: 7, WindowSamples: 256, SampleRateHz: 1000, AccelRange: 0, GyroRange: 0, DLPF: 1, BurstProfile: 2, FragmentTotal: c.FragmentCount(256, 7), FragmentsMissing: []int{1}, DeviceSequence: uint32(id), BootCount: 2}, IngestedAt: at, EstimatedReceivedAt: &at}
	}
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		page := func() (int, int) {
			position, _ := strconv.Atoi(r.URL.Query().Get("cursor"))
			limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
			return position, limit
		}
		switch {
		case r.URL.Path == "/api/v1/collections/7/summary":
			_ = json.NewEncoder(w).Encode(c.CollectionSummary{Collection: c.Collection{ID: 7, Name: "Motor/carga: ação 1", DeviceID: device, StartedAt: start, EndedAt: &ended}, From: start, To: ended, ReadingsUndated: 2, WaveformsUndated: 1})
		case r.URL.Path == "/api/v1/devices/"+device+"/readings":
			if r.URL.Query().Get("from") != start.Format(time.RFC3339Nano) || r.URL.Query().Get("to") != ended.Format(time.RFC3339Nano) {
				t.Errorf("readings interval: %s", r.URL.RawQuery)
			}
			position, limit := page()
			out := c.HistoryPage{Items: []c.Measurement{}}
			// Newest first, like the API.
			for i := position; i < readings && i < position+limit; i++ {
				at := start.Add(time.Duration(readings-i) * time.Second)
				sequence := uint32(readings - i)
				rms, rssi, snr := 0.5, -50.0, 9.0
				out.Items = append(out.Items, c.Measurement{ID: int64(sequence), GatewayID: "g", GatewaySessionID: "s", Reading: c.Reading{EventSeq: int64(sequence), PacketVersion: 3, DeviceID: device, DeviceSequence: &sequence, State: "NORMAL", RMSG: &rms, RSSIDBm: &rssi, SNRDB: &snr}, IngestedAt: at, EstimatedReceivedAt: &at})
			}
			if position+limit < readings {
				out.NextCursor = strconv.Itoa(position + limit)
			}
			_ = json.NewEncoder(w).Encode(out)
		case r.URL.Path == "/api/v1/devices/"+device+"/waveforms":
			position, limit := page()
			out := c.WaveformPage{Items: []c.Waveform{}}
			for i := position; i < windows && i < position+limit; i++ {
				out.Items = append(out.Items, window(int64(windows-i)))
			}
			if position+limit < windows {
				out.NextCursor = strconv.Itoa(position + limit)
			}
			_ = json.NewEncoder(w).Encode(out)
		case strings.HasPrefix(r.URL.Path, "/api/v1/waveforms/"):
			id, _ := strconv.ParseInt(strings.TrimPrefix(r.URL.Path, "/api/v1/waveforms/"), 10, 64)
			out := window(id)
			out.SamplesB64 = base64.StdEncoding.EncodeToString(raw)
			_ = json.NewEncoder(w).Encode(out)
		default:
			t.Errorf("unexpected %s %s", r.Method, r.URL.Path)
			w.WriteHeader(404)
		}
	}))
}

func TestExportPagesThroughEverything(t *testing.T) {
	const readings, windows = 2300, 450
	server := exportServer(t, readings, windows)
	defer server.Close()
	cl, _ := apiclient.New(server.URL, strings.Repeat("u", 43))
	parent := t.TempDir()
	var steps int
	result, err := exportCollection(context.Background(), cl, 7, parent, func(ExportProgress) { steps++ })
	if err != nil {
		t.Fatal(err)
	}
	if result.Readings != readings || result.Windows != windows || result.ReadingsUndated != 2 || result.WindowsUndated != 1 || steps == 0 {
		t.Fatalf("result: %+v, %d steps", result, steps)
	}
	if filepath.Base(result.Folder) != "coleta-7-motor-carga-a-o-1" || filepath.Dir(result.Folder) != parent {
		t.Fatalf("folder: %s", result.Folder)
	}
	rows := readCSV(t, filepath.Join(result.Folder, "leituras.csv"))
	if len(rows) != readings+1 || !reflect.DeepEqual(rows[0], readingColumns) {
		t.Fatalf("leituras.csv: %d rows, header %v", len(rows), rows[0])
	}
	sequence := slicesIndex(rows[0], "device_sequence")
	if rows[1][sequence] != "1" || rows[readings][sequence] != strconv.Itoa(readings) {
		t.Fatalf("not from the oldest: %s %s", rows[1][sequence], rows[readings][sequence])
	}
	absent := slicesIndex(rows[0], "temperature_c")
	if rows[1][absent] != "" {
		t.Fatalf("absent value became %q", rows[1][absent])
	}
	summary := readCSV(t, filepath.Join(result.Folder, "janelas.csv"))
	if len(summary) != windows+1 || summary[1][1] != "janela-1.csv" || summary[windows][1] != fmt.Sprintf("janela-%d.csv", windows) {
		t.Fatalf("janelas.csv: %d rows, %v", len(summary), summary[1])
	}
	if summary[1][slicesIndex(summary[0], "fragments_missing")] != "1" || summary[1][slicesIndex(summary[0], "fragments_missing_count")] != "1" {
		t.Fatalf("missing fragments: %v", summary[1])
	}
	files, err := filepath.Glob(filepath.Join(result.Folder, "janela-*.csv"))
	if err != nil || len(files) != windows {
		t.Fatalf("window files: %d %v", len(files), err)
	}
	window := readCSV(t, filepath.Join(result.Folder, "janela-1.csv"))
	if want := []string{"amostra", "tempo_s", "fragmento", "fragmento_ausente", "ax_bruto", "ax_g", "ay_bruto", "ay_g", "az_bruto", "az_g"}; !reflect.DeepEqual(window[0], want) {
		t.Fatalf("header: %v", window[0])
	}
	if len(window) != 257 {
		t.Fatalf("samples: %d", len(window))
	}
	// Mask 7 has 6 bytes per sample, 240 per fragment: fragment 0 holds samples 0 to 39.
	first := window[1]
	if first[0] != "0" || first[1] != "0.000000" || first[2] != "0" || first[3] != "0" || first[4] != "16384" || first[5] != "1.000000" || first[6] != "-8192" || first[7] != "-0.500000" {
		t.Fatalf("physical and raw values: %v", first)
	}
	// Fragment 1 was lost: it arrived as zeros, and zeros are not data.
	lost := window[41]
	if lost[0] != "40" || lost[2] != "1" || lost[3] != "1" || lost[4] != "" || lost[5] != "" || lost[9] != "" {
		t.Fatalf("lost sample: %v", lost)
	}
	after := window[81]
	if after[0] != "80" || after[2] != "2" || after[3] != "0" || after[4] != "16384" {
		t.Fatalf("after the lost fragment: %v", after)
	}
	// A second export does not overwrite the first, and nothing partial stays behind.
	again, err := exportCollection(context.Background(), cl, 7, parent, func(ExportProgress) {})
	if err != nil || again.Folder == result.Folder {
		t.Fatalf("second export: %+v %v", again, err)
	}
	entries, _ := os.ReadDir(parent)
	if len(entries) != 2 {
		t.Fatalf("left behind: %v", entries)
	}
}

func slicesIndex(items []string, value string) int {
	for i, item := range items {
		if item == value {
			return i
		}
	}
	return -1
}

func TestFailedExportLeavesNothingBehind(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if strings.HasSuffix(r.URL.Path, "/summary") {
			_ = json.NewEncoder(w).Encode(c.CollectionSummary{Collection: c.Collection{ID: 1, Name: "x", DeviceID: "AB12CD34"}})
			return
		}
		w.WriteHeader(503)
	}))
	defer server.Close()
	cl, _ := apiclient.New(server.URL, "")
	parent := t.TempDir()
	if _, err := exportCollection(context.Background(), cl, 1, parent, func(ExportProgress) {}); err == nil {
		t.Fatal("failure not reported")
	}
	if entries, _ := os.ReadDir(parent); len(entries) != 0 {
		t.Fatalf("left behind: %v", entries)
	}
}

// Stale address: the app does not fail silently, and the old session never reaches another server.
func TestStaleEndpointOffersTheLocalOne(t *testing.T) {
	a := testApp(t, "http://127.0.0.1:1")
	// Nothing listens on port 1. The suggestion depends on whether a local API is up.
	probe := a.ProbeConnection()
	if probe.Reachable || probe.Endpoint != "http://127.0.0.1:1" || probe.LocalEndpoint != "http://127.0.0.1:8080" {
		t.Fatalf("probe: %+v", probe)
	}
	if probe.SuggestLocal != probe.LocalReachable {
		t.Fatalf("suggestion: %+v", probe)
	}
	if err := a.store.SaveAPISession(a.ctx, store.APISession{Endpoint: "http://127.0.0.1:1", Email: "o@example.com", Token: "old", ExpiresAt: time.Now().Add(time.Hour).Format(time.RFC3339Nano)}); err != nil {
		t.Fatal(err)
	}
	a.connection.Email = "o@example.com"
	info, err := a.UseLocalEndpoint()
	if err != nil {
		t.Fatal(err)
	}
	if info.Endpoint != apiclient.DefaultEndpoint || info.Authenticated || info.Email != "o@example.com" || a.client != nil {
		t.Fatalf("local: %+v", info)
	}
	saved, err := a.store.APISession(a.ctx)
	if err != nil || saved.Token != "" || saved.Endpoint != apiclient.DefaultEndpoint {
		t.Fatalf("saved: %+v %v", saved, err)
	}
	// A fresh application with no saved address also starts on the local one.
	fresh := NewApp()
	if got := fresh.GetConnection().Endpoint; got != apiclient.DefaultEndpoint {
		t.Fatalf("default: %s", got)
	}
}

type fakeGateway struct {
	paired    atomic.Bool
	connected atomic.Bool
	admin     atomic.Value
	received  atomic.Value
	header    atomic.Value
}

func (f *fakeGateway) handler(t *testing.T) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/info":
			_ = json.NewEncoder(w).Encode(gatewaylink.Info{MAC: "AA:BB:CC:DD:EE:01", FirmwareVersion: "gateway-2.0.0", Paired: f.paired.Load(), Connected: f.connected.Load(), SSID: "local", IP: "192.168.0.9"})
		case "/scan":
			_, _ = w.Write([]byte(`{"networks":[{"ssid":"local","rssi":-40,"secure":true}]}`))
		case "/provision":
			key, _ := f.admin.Load().(string)
			if f.paired.Load() && r.Header.Get("X-Kaelix-Admin") != key {
				w.WriteHeader(403)
				return
			}
			var body gatewaylink.Provision
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Error(err)
			}
			f.received.Store(body)
			f.header.Store(r.Header.Get("X-Kaelix-Admin"))
			f.admin.Store(body.AdminKey)
			f.paired.Store(true)
			w.WriteHeader(204)
		default:
			w.WriteHeader(404)
		}
	})
}

func TestProvisioningAssistantFlow(t *testing.T) {
	const gateway = "5ef49647-9449-4f47-8d1d-c06611c3ba81"
	token := strings.Repeat("t", 43)
	var rotations atomic.Int32
	var saved atomic.Value
	var failConfig atomic.Bool
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.Method + " " + r.URL.Path {
		case "POST /api/v1/gateways":
			w.WriteHeader(201)
			_, _ = w.Write([]byte(`{"gateway":{"id":"` + gateway + `","name":"S3"},"gateway_token":"` + token + `"}`))
		case "GET /api/v1/gateways/" + gateway + "/config":
			_, _ = w.Write([]byte(`{"gateway_id":"` + gateway + `","config":null}`))
		case "PUT /api/v1/gateways/" + gateway + "/config":
			if failConfig.Load() {
				w.WriteHeader(422)
				_, _ = w.Write([]byte(`{"schema_version":1,"error":{"code":"INVALID_REQUEST","message":"x","retryable":false,"details":[{"path":"wifi","reason":"Rede recusada"}]}}`))
				return
			}
			raw, _ := io.ReadAll(r.Body)
			saved.Store(string(raw))
			_, _ = w.Write([]byte(`{"gateway_id":"` + gateway + `"}`))
		case "GET /api/v1/tls/fingerprint":
			_, _ = w.Write([]byte(`{"sha256":"` + strings.Repeat("ab", 32) + `"}`))
		case "POST /api/v1/gateways/" + gateway + "/rotate-token":
			rotations.Add(1)
			_, _ = w.Write([]byte(`{"gateway_token":"` + strings.Repeat("r", 43) + `"}`))
		default:
			t.Errorf("unexpected %s %s", r.Method, r.URL.Path)
			w.WriteHeader(404)
		}
	}))
	defer api.Close()
	fake := &fakeGateway{}
	ap := httptest.NewServer(fake.handler(t))
	defer ap.Close()
	a := testApp(t, api.URL)
	a.linkBaseURL = ap.URL
	password := "senha-do-wifi-1"
	networks := []c.WifiCredential{{SSID: "local", Password: &password}}
	if err := a.SendGatewayProvision(networks); err == nil {
		t.Fatal("sent without starting")
	}
	status, err := a.StartGatewayProvision("S3", "bancada", "")
	if err != nil || status.GatewayID != gateway || status.Existing {
		t.Fatalf("start: %+v %v", status, err)
	}
	// Nothing returned to the interface carries the credential.
	raw, _ := json.Marshal(status)
	if strings.Contains(string(raw), token) {
		t.Fatal("token exposed to the interface")
	}
	info, err := a.ProvisionInfo()
	if err != nil || info.MAC == "" || info.Paired {
		t.Fatalf("info: %+v %v", info, err)
	}
	found, err := a.ProvisionScan()
	if err != nil || len(found) != 1 || !found[0].Secure {
		t.Fatalf("scan: %+v %v", found, err)
	}
	if err = a.SendGatewayProvision(nil); err == nil {
		t.Fatal("no network accepted")
	}
	bad := "curta"
	if err = a.SendGatewayProvision([]c.WifiCredential{{SSID: "local", Password: &bad}}); err == nil {
		t.Fatal("invalid password accepted")
	}
	// A rejection by the API stops before anything reaches the gateway.
	failConfig.Store(true)
	if err = a.SendGatewayProvision(networks); err == nil || err.Error() != "Rede recusada" {
		t.Fatalf("API error: %v", err)
	}
	if fake.received.Load() != nil {
		t.Fatal("gateway paired after an API error")
	}
	failConfig.Store(false)
	if err = a.SendGatewayProvision(networks); err != nil {
		t.Fatal(err)
	}
	body := fake.received.Load().(gatewaylink.Provision)
	if body.GatewayID != gateway || body.GatewayToken != token || len(body.AdminKey) != 64 || body.TLSSHA256 != strings.Repeat("ab", 32) || body.Wifi[0].Password != password {
		t.Fatalf("pairing body: %+v", body)
	}
	if fake.header.Load().(string) != "" {
		t.Fatal("admin header sent to an unpaired gateway")
	}
	if !strings.Contains(saved.Load().(string), password) {
		t.Fatal("network not saved in the API")
	}
	if rotations.Load() != 0 {
		t.Fatal("a new gateway does not need a rotation")
	}
	key, err := a.store.GatewayAdminKey(a.ctx, gateway)
	if err != nil || key != body.AdminKey {
		t.Fatalf("admin key not kept: %v", err)
	}
	if a.provision.token != "" {
		t.Fatal("token kept in memory after the pairing")
	}
	// Pairing the same gateway again rotates once, and proves the saved key.
	if _, err = a.StartGatewayProvision("", "", gateway); err != nil {
		t.Fatal(err)
	}
	if rotations.Load() != 0 {
		t.Fatal("rotated before sending: giving up would have cut the gateway off")
	}
	if err = a.SendGatewayProvision(networks); err != nil {
		t.Fatal(err)
	}
	body = fake.received.Load().(gatewaylink.Provision)
	if rotations.Load() != 1 || body.GatewayToken != strings.Repeat("r", 43) || fake.header.Load().(string) != key || body.AdminKey != key {
		t.Fatalf("second pairing: %d %+v", rotations.Load(), body)
	}
	// A paired gateway this computer has no key for needs the physical reset.
	other := NewApp()
	otherDB, err := store.Open(filepath.Join(t.TempDir(), "other.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer otherDB.Close()
	other.ctx, other.store, other.client, other.apiCtx, other.linkBaseURL = a.ctx, otherDB, a.client, a.ctx, ap.URL
	if _, err = other.StartGatewayProvision("", "", gateway); err != nil {
		t.Fatal(err)
	}
	if err = other.SendGatewayProvision(networks); err == nil || err != gatewaylink.ErrAdminKey {
		t.Fatalf("no key: %v", err)
	}
}

func TestGatewayUnreachableSaysWhatToDo(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	base := server.URL
	server.Close()
	a := testApp(t, base)
	a.linkBaseURL = base
	if _, err := a.ProvisionInfo(); err != gatewaylink.ErrUnreachable {
		t.Fatalf("unreachable: %v", err)
	}
}

func TestFieldBindingsReachTheRoutes(t *testing.T) {
	var calls []string
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls = append(calls, r.Method+" "+r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		switch {
		case strings.HasSuffix(r.URL.Path, "/commands") && r.Method == "GET":
			_, _ = w.Write([]byte(`{"items":[{"seq":1,"kind":"reboot","state":"pending"}]}`))
		case r.URL.Path == "/api/v1/collections" && r.Method == "GET":
			_, _ = w.Write([]byte(`{"items":[{"id":3,"name":"x"}]}`))
		case strings.HasSuffix(r.URL.Path, "/firmware") && r.Method == "POST":
			if r.Header.Get("Content-Type") != "application/octet-stream" || r.URL.Query().Get("version") != "gateway 2" {
				t.Errorf("firmware: %s %s", r.Header.Get("Content-Type"), r.URL.RawQuery)
			}
			w.WriteHeader(201)
			_, _ = w.Write([]byte(`{}`))
		case strings.Contains(r.URL.Path, "/collections/3") && r.Method == "PUT":
			var body c.CollectionInput
			_ = json.NewDecoder(r.Body).Decode(&body)
			if body.EndedAt == nil || body.Name != "x" {
				t.Errorf("stop: %+v", body)
			}
			_, _ = w.Write([]byte(`{"id":3}`))
		default:
			_, _ = w.Write([]byte(`{}`))
		}
	}))
	defer api.Close()
	a := testApp(t, api.URL)
	if items, err := a.GetDeviceCommands("AB12CD34"); err != nil || items[0].Seq != 1 {
		t.Fatalf("commands: %+v %v", items, err)
	}
	if items, err := a.GetCollections(); err != nil || items[0].ID != 3 {
		t.Fatalf("collections: %+v %v", items, err)
	}
	if _, err := a.StopCollection(c.Collection{ID: 3, Name: "x"}); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "gateway.bin")
	if err := os.WriteFile(path, []byte{0xE9, 1}, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := a.uploadFirmware("g", "gateway 2", path); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, make([]byte, maxFirmwareBytes+1), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := a.uploadFirmware("g", "gateway 3", path); err == nil {
		t.Fatal("oversized firmware sent")
	}
	if check := a.CheckDeviceConfig(c.DefaultDeviceConfig()); len(check.Issues) != 0 || *check.MinimumCycleS != 9 {
		t.Fatalf("check: %+v", check)
	}
	broken := c.DefaultDeviceConfig()
	broken.CycleIntervalS = 8
	if check := a.CheckDeviceConfig(broken); len(check.Issues) != 1 || *check.Issues[0].Minimum != 9 {
		t.Fatalf("check: %+v", check)
	}
	for _, want := range []string{"GET /api/v1/devices/AB12CD34/commands", "PUT /api/v1/collections/3", "POST /api/v1/gateways/g/firmware", "GET /api/v1/gateways/g/firmware"} {
		if !slicesContain(calls, want) {
			t.Fatalf("%s not called: %v", want, calls)
		}
	}
}

func slicesContain(items []string, value string) bool { return slicesIndex(items, value) >= 0 }
