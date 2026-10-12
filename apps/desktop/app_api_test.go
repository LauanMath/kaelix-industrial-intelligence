package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	c "kaelix/contracts"
	"kaelix/internal/store"
)

func TestDesktopAPISessionAndRealWorkspace(t *testing.T) {
	var unauthorized atomic.Bool
	var logouts atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/api/v1/auth/login" {
			var body map[string]string
			_ = json.NewDecoder(r.Body).Decode(&body)
			if body["email"] != "operator@example.com" || body["password"] != "password-for-test" {
				t.Error("incorrect login body")
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"access_token": strings.Repeat("a", 43), "expires_at": time.Now().Add(time.Hour)})
			return
		}
		if unauthorized.Load() {
			w.WriteHeader(401)
			return
		}
		if r.Header.Get("Authorization") != "Bearer "+strings.Repeat("a", 43) {
			t.Error("wrong user session")
		}
		switch r.URL.Path {
		case "/api/v1/auth/logout":
			logouts.Add(1)
			w.WriteHeader(204)
		case "/api/v1/devices":
			_ = json.NewEncoder(w).Encode(map[string]any{"items": []c.Device{{ID: "D6F61B44"}}})
		case "/api/v1/motors", "/api/v1/gateways":
			_, _ = w.Write([]byte(`{"items":[]}`))
		case "/api/v1/readings/recent":
			_, _ = w.Write([]byte(`{"items":[{"id":19,"device_id":"D6F61B44","state":"UNKNOWN","rms_g":0.003043,"temperature_c":null}]}`))
		default:
			w.WriteHeader(404)
		}
	}))
	defer server.Close()
	path := filepath.Join(t.TempDir(), "kaelix.db")
	db, err := store.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	a := NewApp()
	a.ctx = context.Background()
	a.store = db
	info, err := a.ConnectAPI(server.URL, "operator@example.com", "password-for-test")
	if err != nil {
		t.Fatal(err)
	}
	if !info.Authenticated {
		t.Fatal("missing session")
	}
	data, err := a.GetWorkspace()
	if err != nil {
		t.Fatal(err)
	}
	if len(data.Motors) != 0 || len(data.Devices) != 1 || len(data.RecentReadings) != 1 || data.RecentReadings[0].State != "UNKNOWN" || data.RecentReadings[0].TemperatureC != nil {
		t.Fatalf("workspace altered data: %+v", data)
	}
	saved, err := db.APISession(a.ctx)
	if err != nil || saved.Token == "" {
		t.Fatalf("session persistence: %v", err)
	}
	resumed := NewApp()
	resumed.ctx = a.ctx
	resumed.store = db
	resumed.restoreAPI()
	if !resumed.GetConnection().Authenticated {
		t.Fatal("session not restored")
	}
	if _, err = resumed.GetWorkspace(); err != nil {
		t.Fatal(err)
	}
	if err = resumed.DisconnectAPI(); err != nil {
		t.Fatal(err)
	}
	if logouts.Load() != 1 {
		t.Fatal("session not revoked")
	}
	saved, _ = db.APISession(a.ctx)
	if saved.Token != "" {
		t.Fatal("logout retained token")
	}
	unauthorized.Store(true)
	if _, err = a.GetWorkspace(); err == nil {
		t.Fatal("expired session accepted")
	}
	if a.GetConnection().Authenticated {
		t.Fatal("expired session retained")
	}
}
