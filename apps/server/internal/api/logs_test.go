package api

import (
	"context"
	"encoding/json"
	"github.com/jackc/pgx/v5/pgxpool"
	"io"
	c "kaelix/contracts"
	"log/slog"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestLogValidationAndRequiredFields(t *testing.T) {
	valid := c.GatewayLogBatch{SchemaVersion: 1, GatewayID: "5ef49647-9449-4f47-8d1d-c06611c3ba81", GatewaySessionID: "17162ec5-4fe1-4dac-a871-879120324a57", Logs: []c.GatewayLog{{LogSeq: 1, UptimeMS: 100, Level: "ERROR", Event: "SPI", Message: "rf_init=-2", Truncated: false}}}
	if err := validateGatewayLogs(valid); err != nil {
		t.Fatal(err)
	}
	bad := valid
	bad.Logs = append(bad.Logs, bad.Logs[0])
	if validateGatewayLogs(bad) == nil {
		t.Fatal("duplicate sequence accepted")
	}
	raw, _ := json.Marshal(valid)
	var decoded c.GatewayLogBatch
	if json.Unmarshal(raw, &decoded) != nil {
		t.Fatal("valid logs rejected")
	}
	for _, change := range []string{strings.Replace(string(raw), `"truncated":false`, `"unknown":false`, 1), strings.Replace(string(raw), `"uptime_ms":100`, `"uptime_ms":null`, 1)} {
		if json.Unmarshal([]byte(change), &decoded) == nil {
			t.Fatal("missing or null field accepted")
		}
	}
	valid.Logs[0].Level = "TRACE"
	if validateGatewayLogs(valid) == nil {
		t.Fatal("invalid severity accepted")
	}
	valid.Logs[0].Level = "INFO"
	valid.Logs[0].Message = strings.Repeat("a", 384)
	if validateGatewayLogs(valid) == nil {
		t.Fatal("oversized diagnostic accepted")
	}
}
func TestRemoteLogsPostgres(t *testing.T) {
	db, dsn := testDatabase(t)
	defer db.Close()
	ctx := context.Background()
	if err := db.BootstrapAdmin(ctx, "debug@example.com", "test-debug-password-123"); err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(New(db, slog.New(slog.NewTextHandler(io.Discard, nil))))
	defer server.Close()
	public := client{t: t, base: server.URL}
	token, _, err := db.Login(ctx, "debug@example.com", "test-debug-password-123")
	if err != nil {
		t.Fatal(err)
	}
	user := public
	user.token = token
	gateway, credential, err := db.CreateGateway(ctx, c.GatewayInput{Name: "Diagnostic gateway"})
	if err != nil {
		t.Fatal(err)
	}
	device := public
	device.token = credential
	b := c.GatewayLogBatch{SchemaVersion: 1, GatewayID: gateway.ID, GatewaySessionID: "17162ec5-4fe1-4dac-a871-879120324a57", Logs: []c.GatewayLog{{LogSeq: 1, UptimeMS: 100, Level: "ERROR", Event: "SPI", Message: "rf_init=-2"}, {LogSeq: 2, UptimeMS: 200, Level: "INFO", Event: "HEALTH", Message: "version=0x12"}}}
	path := "/api/v1/gateways/" + gateway.ID + "/logs"
	public.request("GET", path, nil, 401)
	device.request("GET", path, nil, 401)
	user.request("POST", "/api/v1/ingest/logs", b, 401)
	device.request("POST", "/api/v1/ingest/logs", b, 200)
	device.request("POST", "/api/v1/ingest/logs", b, 200)
	page := parse[c.GatewayLogPage](t, user.request("GET", path+"?limit=1", nil, 200))
	if len(page.Items) != 1 || page.Items[0].LogSeq != 2 || page.Items[0].GatewayID != gateway.ID || page.Items[0].ReceivedAt.IsZero() {
		t.Fatalf("metadata or initial tail wrong: %+v", page)
	}
	original := parse[c.GatewayLogPage](t, user.request("GET", path, nil, 200))
	if len(original.Items) != 2 {
		t.Fatal("retry created duplicates")
	}
	mixed := b
	mixed.Logs = []c.GatewayLog{{LogSeq: 3, UptimeMS: 300, Level: "WARN", Event: "SPI", Message: "new"}, b.Logs[0]}
	mixed.Logs[1].Message = "conflict"
	device.request("POST", "/api/v1/ingest/logs", mixed, 409)
	page = parse[c.GatewayLogPage](t, user.request("GET", path, nil, 200))
	if len(page.Items) != 2 {
		t.Fatal("conflict did not roll back batch")
	}
	b.Logs = []c.GatewayLog{{LogSeq: 3, UptimeMS: 300, Level: "INFO", Event: "RX", Message: "third"}, {LogSeq: 4, UptimeMS: 400, Level: "INFO", Event: "UPLOAD", Message: "fourth"}}
	device.request("POST", "/api/v1/ingest/logs", b, 200)
	page = parse[c.GatewayLogPage](t, user.request("GET", path+"?after_id="+original.NextCursor+"&limit=1", nil, 200))
	if len(page.Items) != 1 || page.Items[0].LogSeq != 3 || !page.HasMore {
		t.Fatal("incremental page lost a log")
	}
	last := parse[c.GatewayLogPage](t, user.request("GET", path+"?after_id="+page.NextCursor, nil, 200))
	if len(last.Items) != 1 || last.Items[0].LogSeq != 4 {
		t.Fatal("cursor skipped log")
	}
	user.request("GET", path+"?after_id=-1", nil, 400)
	b.GatewayID = "5ef49647-9449-4f47-8d1d-c06611c3ba81"
	device.request("POST", "/api/v1/ingest/logs", b, 403)
	// Diagnostic retention is bounded and does not create sensor measurements.
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	_, err = pool.Exec(ctx, `INSERT INTO gateway_logs(gateway_id,gateway_session_id,log_seq,payload) SELECT $1,'retention-test',n,jsonb_build_object('log_seq',n,'uptime_ms',n,'level','INFO','event','HEALTH','message','seed','truncated',false) FROM generate_series(1,5000) n`, gateway.ID)
	if err != nil {
		t.Fatal(err)
	}
	b.GatewayID = gateway.ID
	b.Logs = []c.GatewayLog{{LogSeq: 5, UptimeMS: 500, Level: "INFO", Event: "RX", Message: "last"}}
	device.request("POST", "/api/v1/ingest/logs", b, 200)
	var count, measurements int
	if err = pool.QueryRow(ctx, `SELECT count(*) FROM gateway_logs WHERE gateway_id=$1`, gateway.ID).Scan(&count); err != nil {
		t.Fatal(err)
	}
	_ = pool.QueryRow(ctx, `SELECT count(*) FROM readings`).Scan(&measurements)
	if count != 5000 || measurements != 0 {
		t.Fatalf("bad retention or diagnostic changed readings: %d/%d", count, measurements)
	}
}
