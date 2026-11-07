package gatewaylink

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// The gateway's web server only exposes a JSON body under this content type.
func TestProvisionIsSentAsJSON(t *testing.T) {
	var method, path, contentType, admin string
	var body map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		method, path, contentType, admin = r.Method, r.URL.Path, r.Header.Get("Content-Type"), r.Header.Get("X-Kaelix-Admin")
		raw, _ := io.ReadAll(r.Body)
		if err := json.Unmarshal(raw, &body); err != nil {
			t.Errorf("body is not JSON: %s", raw)
		}
		w.WriteHeader(204)
	}))
	defer server.Close()
	request := Provision{GatewayID: "uuid", GatewayToken: strings.Repeat("t", 43), AdminKey: strings.Repeat("a", 64), TLSSHA256: strings.Repeat("b", 64), Wifi: []WifiNetwork{{SSID: "rede", Password: "senha-da-rede"}}}
	if err := New(server.URL).Send(context.Background(), request, ""); err != nil {
		t.Fatal(err)
	}
	if method != http.MethodPost || path != "/provision" || contentType != "application/json" {
		t.Fatalf("request: %s %s %q", method, path, contentType)
	}
	if admin != "" {
		t.Fatalf("admin header on an unpaired gateway: %q", admin)
	}
	wifi, _ := body["wifi"].([]any)
	first, _ := wifi[0].(map[string]any)
	if body["gateway_id"] != "uuid" || body["gateway_token"] != request.GatewayToken || body["admin_key"] != request.AdminKey || body["tls_sha256"] != request.TLSSHA256 || first["ssid"] != "rede" || first["password"] != "senha-da-rede" {
		t.Fatalf("body: %v", body)
	}
	if err := New(server.URL).Send(context.Background(), request, "chave"); err != nil {
		t.Fatal(err)
	}
	if admin != "chave" || contentType != "application/json" {
		t.Fatalf("paired request: %q %q", admin, contentType)
	}
}

func TestReadRoutesAndFailures(t *testing.T) {
	status := 200
	response := `{"mac":"AA","firmware_version":"v","gateway_id":"g","paired":true,"connected":true,"ssid":"s","ip":"1.2.3.4","config_version":3,"networks":null}`
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || (r.URL.Path != "/info" && r.URL.Path != "/scan") {
			t.Errorf("unexpected %s %s", r.Method, r.URL.Path)
		}
		w.WriteHeader(status)
		_, _ = w.Write([]byte(response))
	}))
	cl := New(server.URL)
	ctx := context.Background()
	info, err := cl.Info(ctx)
	if err != nil || info.MAC != "AA" || !info.Paired || !info.Connected || info.ConfigVersion != 3 {
		t.Fatalf("info: %+v %v", info, err)
	}
	// An empty scan is a list, not null, for the interface.
	if networks, err := cl.Scan(ctx); err != nil || networks == nil || len(networks) != 0 {
		t.Fatalf("scan: %v %v", networks, err)
	}
	status = 403
	if _, err = cl.Info(ctx); !errors.Is(err, ErrAdminKey) {
		t.Fatalf("refused: %v", err)
	}
	status = 500
	if _, err = cl.Info(ctx); err == nil || errors.Is(err, ErrUnreachable) {
		t.Fatalf("server error: %v", err)
	}
	status, response = 200, "not json"
	if _, err = cl.Info(ctx); err == nil {
		t.Fatal("malformed answer accepted")
	}
	server.Close()
	if _, err = cl.Info(ctx); !errors.Is(err, ErrUnreachable) {
		t.Fatalf("stopped: %v", err)
	}
}
