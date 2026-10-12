package main

import (
	"context"
	"encoding/json"
	c "kaelix/contracts"
	"kaelix/internal/apiclient"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestDesktopRemoteLogsPreserveCursorAndMetadata(t *testing.T) {
	const gateway = "5ef49647-9449-4f47-8d1d-c06611c3ba81"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/gateways/"+gateway+"/logs" || r.URL.Query().Get("after_id") != "9007199254740993" {
			t.Error("wrong log cursor")
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(c.GatewayLogPage{Items: []c.StoredGatewayLog{{ID: "9007199254740994", GatewayID: gateway, GatewaySessionID: "session", GatewayLog: c.GatewayLog{LogSeq: 7, Level: "ERROR", Event: "SPI", Message: "rf_init=-2"}}}, NextCursor: "9007199254740994"})
	}))
	defer server.Close()
	cl, _ := apiclient.New(server.URL, "test-token")
	a := NewApp()
	a.client = cl
	a.apiCtx = context.Background()
	page, err := a.GetGatewayLogs(gateway, "9007199254740993")
	if err != nil {
		t.Fatal(err)
	}
	if len(page.Items) != 1 || page.Items[0].ID != "9007199254740994" || page.Items[0].Event != "SPI" || page.NextCursor != "9007199254740994" {
		t.Fatal("diagnostic metadata lost")
	}
	if _, err = a.GetGatewayLogs("invalid", ""); err == nil {
		t.Fatal("invalid gateway accepted")
	}
}
