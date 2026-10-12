package api

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/santhosh-tekuri/jsonschema/v6"
	c "kaelix/contracts"
	"kaelix/server/internal/store"
)

const contractDir = "../../../../packages/contracts/gateway-v1"

func contractFile(t *testing.T, name string) []byte {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(contractDir, name))
	if err != nil {
		t.Fatal(err)
	}
	return raw
}
func checkSchema(t *testing.T, schema string, raw []byte) {
	t.Helper()
	compiler := jsonschema.NewCompiler()
	compiler.AssertFormat()
	doc, err := jsonschema.UnmarshalJSON(bytes.NewReader(contractFile(t, schema+".schema.json")))
	if err != nil {
		t.Fatal(err)
	}
	loc := "https://contracts.kaelix.invalid/" + schema + ".schema.json"
	if err = compiler.AddResource(loc, doc); err != nil {
		t.Fatal(err)
	}
	compiled, err := compiler.Compile(loc)
	if err != nil {
		t.Fatal(err)
	}
	value, err := jsonschema.UnmarshalJSON(bytes.NewReader(raw))
	if err != nil {
		t.Fatal(err)
	}
	if err = compiled.Validate(value); err != nil {
		t.Fatalf("%s: %v\n%s", schema, err, raw)
	}
}
func TestFirmwareFixtures(t *testing.T) {
	for _, name := range []string{"readings-request", "readings-response", "readings-duplicate-response", "heartbeat-request", "heartbeat-response", "error-response"} {
		schema := name
		if schema == "readings-duplicate-response" {
			schema = "readings-response"
		}
		checkSchema(t, schema, contractFile(t, "examples/"+name+".json"))
	}
	var batch c.ReadingBatch
	if err := json.Unmarshal(contractFile(t, "examples/readings-request.json"), &batch); err != nil {
		t.Fatal(err)
	}
	if err := validateBatch(batch, time.Now()); err != nil {
		t.Fatal(err)
	}
	var heartbeat c.Heartbeat
	if err := json.Unmarshal(contractFile(t, "examples/heartbeat-request.json"), &heartbeat); err != nil {
		t.Fatal(err)
	}
	if err := validateHeartbeat(heartbeat); err != nil {
		t.Fatal(err)
	}
}
func TestFirmwareContractAgainstPostgres(t *testing.T) {
	db, _ := testDatabase(t)
	defer db.Close()
	gateway, token, err := db.CreateGateway(context.Background(), c.GatewayInput{ID: "5ef49647-9449-4f47-8d1d-c06611c3ba81", Name: "Contract gateway"})
	if err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(New(db, slog.New(slog.NewTextHandler(io.Discard, nil))))
	defer server.Close()
	cl := client{t: t, base: server.URL, token: token}
	batch := parse[c.ReadingBatch](t, contractFile(t, "examples/readings-request.json"))
	raw := cl.request("POST", "/api/v1/ingest/readings", batch, 200)
	checkSchema(t, "readings-response", raw)
	stored := parse[c.IngestResult](t, raw)
	if len(stored.Acknowledged) != 1 || stored.Acknowledged[0].Status != "stored" {
		t.Fatal("missing stored acknowledgement")
	}
	batch.UploadUptimeMS += 1000
	raw = cl.request("POST", "/api/v1/ingest/readings", batch, 200)
	checkSchema(t, "readings-response", raw)
	duplicate := parse[c.IngestResult](t, raw)
	if len(duplicate.Acknowledged) != 1 || duplicate.Acknowledged[0].Status != "duplicate" {
		t.Fatal("missing duplicate acknowledgement")
	}
	// A mixed batch acknowledges each event exactly once, with individual status.
	mixed := parse[c.ReadingBatch](t, contractFile(t, "examples/readings-request.json"))
	additional := mixed.Readings[0]
	additional.EventSeq = 9
	additional.DeviceID = "AABBCCDD"
	mixed.Readings = append(mixed.Readings, additional)
	mixedRaw := cl.request("POST", "/api/v1/ingest/readings", mixed, 200)
	checkSchema(t, "readings-response", mixedRaw)
	mixedAck := parse[c.IngestResult](t, mixedRaw)
	if len(mixedAck.Acknowledged) != 2 || mixedAck.Acknowledged[0].EventSeq != 1 || mixedAck.Acknowledged[0].Status != "duplicate" || mixedAck.Acknowledged[1].EventSeq != 9 || mixedAck.Acknowledged[1].Status != "stored" {
		t.Fatalf("mixed acknowledgement %+v", mixedAck)
	}
	// uint32 device counter, arbitrary uint8 diagnosis, finite radio values and protocol anomaly.
	batch.Readings[0].EventSeq = 2
	batch.Readings[0].State = "ANOMALOUS"
	sequence := uint32(4294967295)
	diagnosis := uint8(255)
	batch.Readings[0].DeviceSequence = &sequence
	batch.Readings[0].DiagCode = &diagnosis
	unusualRSSI, unusualSNR := 1.0, 101.0
	batch.Readings[0].RSSIDBm = &unusualRSSI
	batch.Readings[0].SNRDB = &unusualSNR
	checkSchema(t, "readings-response", cl.request("POST", "/api/v1/ingest/readings", batch, 200))
	// Maximum safe uptime must not overflow time.Duration or fabricate a recent timestamp.
	batch.Readings[0].EventSeq = 3
	batch.UploadUptimeMS = maxSafeInteger
	batch.Readings[0].GatewayRxUptimeMS = 0
	checkSchema(t, "readings-response", cl.request("POST", "/api/v1/ingest/readings", batch, 200))
	page, err := db.History(context.Background(), batch.Readings[0].DeviceID, store.HistoryFilter{Limit: 100})
	if err != nil {
		t.Fatal(err)
	}
	if len(page.Items) != 3 || page.Items[len(page.Items)-1].EstimatedReceivedAt != nil || page.Items[len(page.Items)-1].QueueDelayMS != maxSafeInteger {
		t.Fatalf("large uptime overflow or delay lost: %+v", page)
	}
	// Error structure is compatible for semantic validation and unsupported schemas.
	batch.Readings[0].EventSeq = 4
	batch.Readings[0].RMSG = nil
	raw = cl.request("POST", "/api/v1/ingest/readings", batch, 422)
	checkSchema(t, "error-response", raw)
	invalid := parse[c.APIError](t, raw)
	if invalid.Error.Code != "INVALID_REQUEST" || invalid.Error.Retryable || len(invalid.Error.Details) != 1 || invalid.Error.Details[0].Path != "readings[0].rms_g" {
		t.Fatalf("validation error %+v", invalid)
	}
	batch.SchemaVersion = 2
	raw = cl.request("POST", "/api/v1/ingest/readings", batch, 422)
	checkSchema(t, "error-response", raw)
	if parse[c.APIError](t, raw).Error.Code != "UNSUPPORTED_SCHEMA" {
		t.Fatal("wrong schema error")
	}
	public := cl
	public.token = ""
	checkSchema(t, "error-response", public.request("POST", "/api/v1/ingest/readings", batch, 401))
	h := parse[c.Heartbeat](t, contractFile(t, "examples/heartbeat-request.json"))
	checkSchema(t, "heartbeat-response", cl.request("POST", "/api/v1/ingest/heartbeat", h, 200))
	items, err := db.Gateways(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	original := *items[0].LastHeartbeatAt
	// Exact duplicate and an older new snapshot do not change activity time.
	checkSchema(t, "heartbeat-response", cl.request("POST", "/api/v1/ingest/heartbeat", h, 200))
	h.HeartbeatSeq = 3
	h.UptimeMS = 60000
	h.Counters.LoRaValid = 3
	h.Counters.ReadingsAcknowledged = 3
	checkSchema(t, "heartbeat-response", cl.request("POST", "/api/v1/ingest/heartbeat", h, 200))
	items, err = db.Gateways(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	current := *items[0].LastHeartbeatAt
	if !current.After(original) {
		t.Fatal("fresh heartbeat did not update activity")
	}
	old := h
	old.HeartbeatSeq = 2
	old.UptimeMS = 45000
	old.Counters.LoRaValid = 2
	old.Counters.ReadingsAcknowledged = 2
	checkSchema(t, "heartbeat-response", cl.request("POST", "/api/v1/ingest/heartbeat", old, 200))
	items, err = db.Gateways(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if !items[0].LastHeartbeatAt.Equal(current) || items[0].Heartbeat.HeartbeatSeq != 3 {
		t.Fatal("old snapshot renewed liveness or replaced current")
	}
	changed := h
	changed.Health.QueueSize = 1
	raw = cl.request("POST", "/api/v1/ingest/heartbeat", changed, 409)
	checkSchema(t, "error-response", raw)
	if parse[c.APIError](t, raw).Error.Code != "EVENT_CONFLICT" {
		t.Fatal("wrong heartbeat conflict code")
	}
	decrease := h
	decrease.HeartbeatSeq = 4
	decrease.UptimeMS++
	decrease.Counters.LoRaValid = 1
	checkSchema(t, "error-response", cl.request("POST", "/api/v1/ingest/heartbeat", decrease, 422))
	// A new boot resets counters. Once retired, the former session cannot replace it.
	fresh := h
	fresh.GatewaySessionID = "47162ec5-4fe1-4dac-a871-879120324a57"
	fresh.HeartbeatSeq = 1
	fresh.UptimeMS = 1000
	fresh.Counters = c.HeartbeatCounters{}
	checkSchema(t, "heartbeat-response", cl.request("POST", "/api/v1/ingest/heartbeat", fresh, 200))
	items, err = db.Gateways(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	newActivity := *items[0].LastHeartbeatAt
	h.HeartbeatSeq = 4
	h.UptimeMS = 90000
	h.Counters.LoRaValid = 4
	h.Counters.ReadingsAcknowledged = 4
	checkSchema(t, "heartbeat-response", cl.request("POST", "/api/v1/ingest/heartbeat", h, 200))
	items, err = db.Gateways(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if items[0].ID != gateway.ID || *items[0].ActiveSessionID != fresh.GatewaySessionID || !items[0].LastHeartbeatAt.Equal(newActivity) || items[0].Heartbeat.Counters.LoRaValid != 0 {
		t.Fatal("retired session replaced active state")
	}
}
func TestStrictGatewayBodies(t *testing.T) {
	raw := string(contractFile(t, "examples/heartbeat-request.json"))
	cases := []struct{ name, body string }{
		{"duplicate root", strings.Replace(raw, `"schema_version": 1,`, `"schema_version": 2,"schema_version": 1,`, 1)},
		{"duplicate nested", strings.Replace(raw, `"queue_size": 0,`, `"queue_size": 4,"queue_size": 0,`, 1)},
		{"missing bool", strings.Replace(raw, `"radio_initialized": true,`, "", 1)},
		{"null health", strings.Replace(raw, `"radio_initialized": true,`, `"radio_initialized": null,`, 1)},
		{"non UTC", strings.Replace(raw, `"sent_at": null`, `"sent_at":"2026-10-11T00:00:00+00:00"`, 1)},
		{"trailing document", raw + `{}`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r := httptest.NewRequest(http.MethodPost, "/api/v1/ingest/heartbeat", strings.NewReader(tc.body))
			r.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			var h c.Heartbeat
			if decode(w, r, &h) {
				t.Fatal("invalid body accepted")
			}
			checkSchema(t, "error-response", w.Body.Bytes())
		})
	}
	r := httptest.NewRequest(http.MethodPost, "/api/v1/ingest/readings", strings.NewReader(strings.Repeat(" ", 32*1024)+"{}"))
	r.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	var b c.ReadingBatch
	if decode(w, r, &b) || w.Code != 413 {
		t.Fatalf("32 KiB limit: %d", w.Code)
	}
	checkSchema(t, "error-response", w.Body.Bytes())
}
func TestUUIDRegistration(t *testing.T) {
	db, _ := testDatabase(t)
	defer db.Close()
	g, _, err := db.CreateGateway(context.Background(), c.GatewayInput{Name: "Generated gateway"})
	if err != nil {
		t.Fatal(err)
	}
	if !uuidID.MatchString(g.ID) {
		t.Fatalf("not UUID: %s", g.ID)
	}
}
