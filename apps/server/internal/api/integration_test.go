package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	c "kaelix/contracts"
	"kaelix/server/internal/store"
)

type client struct {
	t     *testing.T
	base  string
	token string
}

func (cl client) request(method, path string, body any, want int) []byte {
	cl.t.Helper()
	var payload []byte
	if body != nil {
		var err error
		payload, err = json.Marshal(body)
		if err != nil {
			cl.t.Fatal(err)
		}
	}
	req, err := http.NewRequest(method, cl.base+path, bytes.NewReader(payload))
	if err != nil {
		cl.t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	if cl.token != "" {
		req.Header.Set("Authorization", "Bearer "+cl.token)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		cl.t.Fatal(err)
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		cl.t.Fatal(err)
	}
	if resp.StatusCode != want {
		cl.t.Fatalf("%s %s: status=%d wanted=%d body=%s", method, path, resp.StatusCode, want, raw)
	}
	return raw
}
func parse[T any](t *testing.T, raw []byte) T {
	t.Helper()
	var out T
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatal(err)
	}
	return out
}
func testDatabase(t *testing.T) (*store.Store, string) {
	t.Helper()
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("set TEST_DATABASE_URL or run scripts/test-server-postgres.sh for PostgreSQL integration tests")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	schema := "test_" + fmt.Sprint(time.Now().UnixNano())
	quoted := pgx.Identifier{schema}.Sanitize()
	if _, err = pool.Exec(ctx, "CREATE SCHEMA "+quoted); err != nil {
		pool.Close()
		t.Fatal(err)
	}
	t.Cleanup(func() { _, _ = pool.Exec(ctx, "DROP SCHEMA "+quoted+" CASCADE"); pool.Close() })
	parsed, err := url.Parse(dsn)
	if err != nil {
		t.Fatal(err)
	}
	q := parsed.Query()
	q.Set("search_path", schema)
	parsed.RawQuery = q.Encode()
	s, err := store.Open(ctx, parsed.String())
	if err != nil {
		t.Fatal(err)
	}
	return s, parsed.String()
}
func TestPostgresWorkflow(t *testing.T) {
	db, dsn := testDatabase(t)
	defer func() { db.Close() }()
	ctx := context.Background()
	const password = "valid-test-password-123"
	if err := db.BootstrapAdmin(ctx, "admin@example.com", password); err != nil {
		t.Fatal(err)
	}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	server := httptest.NewServer(New(db, logger))
	defer func() { server.Close() }()
	public := client{t: t, base: server.URL}
	public.request("GET", "/readyz", nil, 200)
	public.request("GET", "/api/v1/devices", nil, 401)
	public.request("GET", "/api/v1/readings/recent", nil, 401)
	public.request("POST", "/api/v1/auth/login", map[string]string{"email": "admin@example.com", "password": "incorrect"}, 401)
	login := parse[struct {
		Token string `json:"access_token"`
	}](t, public.request("POST", "/api/v1/auth/login", map[string]string{"email": "admin@example.com", "password": password}, 200))
	user := public
	user.token = login.Token
	registration := parse[struct {
		Token string `json:"gateway_token"`
	}](t, user.request("POST", "/api/v1/gateways", c.GatewayInput{ID: "5ef49647-9449-4f47-8d1d-c06611c3ba81", Name: "Main gateway", Location: "Factory"}, 201))
	gateway := public
	gateway.token = registration.Token
	gateway.request("GET", "/api/v1/devices", nil, 401)
	var batch c.ReadingBatch
	if err := json.Unmarshal([]byte(fixture), &batch); err != nil {
		t.Fatal(err)
	}
	user.request("POST", "/api/v1/ingest/readings", batch, 401)
	spoof := batch
	spoof.GatewayID = "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa"
	gateway.request("POST", "/api/v1/ingest/readings", spoof, 403)
	first := parse[c.IngestResult](t, gateway.request("POST", "/api/v1/ingest/readings", batch, 200))
	if len(first.Acknowledged) != 1 || first.Acknowledged[0].Status != "stored" {
		t.Fatalf("first ack: %+v", first)
	}
	batch.UploadUptimeMS = 25000 // Retry must preserve the first stored receive estimate/delay.
	replay := parse[c.IngestResult](t, gateway.request("POST", "/api/v1/ingest/readings", batch, 200))
	if len(replay.Acknowledged) != 1 || replay.Acknowledged[0].Status != "duplicate" {
		t.Fatalf("replay ack: %+v", replay)
	}
	devices := parse[struct {
		Items []c.Device `json:"items"`
	}](t, user.request("GET", "/api/v1/devices", nil, 200))
	if len(devices.Items) != 1 || devices.Items[0].MotorID != nil || devices.Items[0].Latest.RMSG != nil || devices.Items[0].Latest.State != "UNKNOWN" || devices.Items[0].Latest.QueueDelayMS != 1000 {
		t.Fatalf("unassociated/null preservation: %+v", devices)
	}
	device := batch.Readings[0].DeviceID
	user.request("PUT", "/api/v1/devices/"+device, c.DeviceInput{Profile: "MPU", Notes: "Validated in bench"}, 200)
	motor := c.MotorInput{ID: "M-01", Name: "Feed motor", Location: "Line A", DeviceID: &device, ExpectedIntervalSeconds: 11}
	user.request("POST", "/api/v1/motors", motor, 201)
	motor.ID = "M-02"
	user.request("POST", "/api/v1/motors", motor, 409)
	unknown := c.DeviceInput{ID: "A1B2C301", Profile: "MPU"}
	user.request("POST", "/api/v1/devices", unknown, 201)
	user.request("POST", "/api/v1/devices", unknown, 409)
	// A late upload must not replace the current reading; device sequence may repeat after reset.
	next := batch
	next.GatewaySessionID = "27162ec5-4fe1-4dac-a871-879120324a57"
	next.Readings = append([]c.Reading(nil), batch.Readings...)
	later := time.Now().UTC().Add(-time.Minute)
	next.Readings[0].GatewayReceivedAt = &later
	gateway.request("POST", "/api/v1/ingest/readings", next, 200)
	latest := parse[struct {
		Items []c.Motor `json:"items"`
	}](t, user.request("GET", "/api/v1/motors", nil, 200))
	if len(latest.Items) != 1 || latest.Items[0].Latest.GatewaySessionID != "17162ec5-4fe1-4dac-a871-879120324a57" {
		t.Fatalf("late reading replaced latest: %+v", latest)
	}
	history := parse[c.HistoryPage](t, user.request("GET", "/api/v1/motors/M-01/readings?limit=1", nil, 200))
	if len(history.Items) != 1 || history.NextCursor == "" {
		t.Fatalf("first page %+v", history)
	}
	second := parse[c.HistoryPage](t, user.request("GET", "/api/v1/motors/M-01/readings?limit=1&cursor="+url.QueryEscape(history.NextCursor), nil, 200))
	if len(second.Items) != 1 || second.NextCursor != "" || second.Items[0].ID == history.Items[0].ID {
		t.Fatalf("second page %+v", second)
	}
	from := later.Add(-time.Second).Format(time.RFC3339Nano)
	to := later.Add(time.Second).Format(time.RFC3339Nano)
	filtered := parse[c.HistoryPage](t, user.request("GET", "/api/v1/devices/"+device+"/readings?from="+url.QueryEscape(from)+"&to="+url.QueryEscape(to), nil, 200))
	if len(filtered.Items) != 1 || filtered.Items[0].GatewaySessionID != "27162ec5-4fe1-4dac-a871-879120324a57" {
		t.Fatalf("period filtering %+v", filtered)
	}
	user.request("GET", "/api/v1/motors/missing/readings", nil, 404)
	user.request("GET", "/api/v1/devices/"+device+"/readings?cursor=bad", nil, 400)
	// Insert a new reading, then encounter an altered duplicate: the entire transaction must roll back.
	conflict := batch
	conflict.Readings = append([]c.Reading(nil), batch.Readings...)
	newer := conflict.Readings[0]
	newer.EventSeq = 2
	newer.DeviceID = "DEADBEEF"
	conflicting := conflict.Readings[0]
	v := 0.8
	conflicting.RMSG = &v
	conflict.Readings = []c.Reading{newer, conflicting}
	gateway.request("POST", "/api/v1/ingest/readings", conflict, 409)
	user.request("GET", "/api/v1/devices/DEADBEEF/readings", nil, 404)
	all := parse[c.HistoryPage](t, user.request("GET", "/api/v1/devices/"+device+"/readings", nil, 200))
	if len(all.Items) != 2 {
		t.Fatal("conflicting batch partially persisted")
	}
	// Concurrent retransmissions all receive acknowledgements and create exactly one row.
	concurrent := batch
	concurrent.GatewaySessionID = "37162ec5-4fe1-4dac-a871-879120324a57"
	concurrent.Readings = append([]c.Reading(nil), batch.Readings...)
	var wg sync.WaitGroup
	results := make(chan error, 8)
	for range 8 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			ack, err := db.Ingest(ctx, concurrent)
			if err == nil && len(ack.Acknowledged) != 1 {
				err = fmt.Errorf("missing ack")
			}
			results <- err
		}()
	}
	wg.Wait()
	close(results)
	for err := range results {
		if err != nil {
			t.Fatal(err)
		}
	}
	all = parse[c.HistoryPage](t, user.request("GET", "/api/v1/devices/"+device+"/readings", nil, 200))
	if len(all.Items) != 3 {
		t.Fatalf("concurrent dedup: %d", len(all.Items))
	}
	hb := c.Heartbeat{SchemaVersion: 1, GatewayID: "5ef49647-9449-4f47-8d1d-c06611c3ba81", GatewaySessionID: "27162ec5-4fe1-4dac-a871-879120324a57", HeartbeatSeq: 1, UptimeMS: 1234, FirmwareVersion: "0.1.0", Health: c.HeartbeatHealth{QueueSize: 2, QueueCapacity: 64}}
	gateway.request("POST", "/api/v1/ingest/heartbeat", hb, 200)
	gateways := parse[struct {
		Items []c.Gateway `json:"items"`
	}](t, user.request("GET", "/api/v1/gateways", nil, 200))
	if len(gateways.Items) != 1 || gateways.Items[0].Heartbeat == nil || gateways.Items[0].Heartbeat.Health.QueueSize != 2 || gateways.Items[0].LastIngestedAt == nil {
		t.Fatalf("heartbeat %+v", gateways)
	}
	// Reopening applies migrations once and retains historical data and account credentials.
	server.Close()
	db.Close()
	var err error
	db, err = store.Open(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	if err = db.BootstrapAdmin(ctx, "admin@example.com", "different-password-123"); err != nil {
		t.Fatal(err)
	}
	server = httptest.NewServer(New(db, logger))
	public.base = server.URL
	user.base = server.URL
	gateway.base = server.URL
	all = parse[c.HistoryPage](t, user.request("GET", "/api/v1/devices/"+device+"/readings", nil, 200))
	if len(all.Items) != 3 {
		t.Fatal("readings not persisted")
	}
	recent := parse[struct {
		Items []c.Measurement `json:"items"`
	}](t, user.request("GET", "/api/v1/readings/recent", nil, 200))
	if len(recent.Items) != 3 || recent.Items[0].IngestedAt.Before(recent.Items[1].IngestedAt) {
		t.Fatal("recent readings missing or out of ingestion order")
	}

	public.request("POST", "/api/v1/auth/login", map[string]string{"email": "admin@example.com", "password": password}, 200)
	rotation := parse[struct {
		Token string `json:"gateway_token"`
	}](t, user.request("POST", "/api/v1/gateways/5ef49647-9449-4f47-8d1d-c06611c3ba81/rotate-token", nil, 200))
	gateway.request("POST", "/api/v1/ingest/readings", batch, 401)
	gateway.token = rotation.Token
	gateway.request("POST", "/api/v1/ingest/readings", batch, 200)
	// Removing an association does not remove device history.
	motor.ID = "M-01"
	motor.DeviceID = nil
	user.request("PUT", "/api/v1/motors/M-01", motor, 200)
	empty := parse[c.HistoryPage](t, user.request("GET", "/api/v1/motors/M-01/readings", nil, 200))
	if len(empty.Items) != 0 {
		t.Fatal("unassociated motor still exposes device history")
	}
	motor.ID = "M-02"
	motor.DeviceID = &device
	user.request("POST", "/api/v1/motors", motor, 201)
	retained := parse[c.HistoryPage](t, user.request("GET", "/api/v1/motors/M-02/readings", nil, 200))
	if len(retained.Items) != 3 {
		t.Fatal("association change lost history")
	}
	user.request("POST", "/api/v1/auth/logout", nil, 204)
	user.request("GET", "/api/v1/devices", nil, 401)
	newLogin := parse[struct {
		Token string `json:"access_token"`
	}](t, public.request("POST", "/api/v1/auth/login", map[string]string{"email": "admin@example.com", "password": password}, 200))
	user.token = newLogin.Token
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	if _, err = pool.Exec(ctx, `UPDATE sessions SET expires_at=now()-interval '1 second'`); err != nil {
		t.Fatal(err)
	}
	user.request("GET", "/api/v1/devices", nil, 401)
}
func TestLoginRateLimit(t *testing.T) {
	db, _ := testDatabase(t)
	defer db.Close()
	server := httptest.NewServer(New(db, slog.New(slog.NewTextHandler(io.Discard, nil))))
	defer server.Close()
	cl := client{t: t, base: server.URL}
	for range 10 {
		cl.request("POST", "/api/v1/auth/login", map[string]string{"email": "missing@example.com", "password": "invalid"}, 401)
	}
	cl.request("POST", "/api/v1/auth/login", map[string]string{"email": "missing@example.com", "password": "invalid"}, 429)
}
func TestDecodeLimits(t *testing.T) {
	for _, tc := range []struct {
		body   string
		status int
	}{{`{"name":"x"} {}`, 400}, {`{"unknown":1}`, 400}, {strings.Repeat(" ", 65537) + `{}`, 413}, {`null`, 400}} {
		req := httptest.NewRequest("POST", "/", strings.NewReader(tc.body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		var body c.ReadingBatch
		if decode(w, req, &body) {
			t.Fatalf("accepted invalid body")
		}
		if w.Code != tc.status {
			t.Fatalf("status %d expected %d", w.Code, tc.status)
		}
	}
}
