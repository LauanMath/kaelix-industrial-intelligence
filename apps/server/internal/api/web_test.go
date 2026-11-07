package api

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func webHandler() http.Handler { return NewWeb(nil, slog.New(slog.NewTextHandler(io.Discard, nil))) }

func TestWebServesThePageWithItsPolicy(t *testing.T) {
	w := httptest.NewRecorder()
	webHandler().ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/", nil))
	if w.Code != 200 || !strings.HasPrefix(w.Header().Get("Content-Type"), "text/html") {
		t.Fatalf("status %d type %q", w.Code, w.Header().Get("Content-Type"))
	}
	if !strings.Contains(w.Body.String(), "Kaelix Coleta") {
		t.Fatal("page body")
	}
	for _, want := range []string{"default-src 'none'", "connect-src 'self'", "frame-ancestors 'none'"} {
		if !strings.Contains(w.Header().Get("Content-Security-Policy"), want) {
			t.Fatalf("policy lacks %q: %s", want, w.Header().Get("Content-Security-Policy"))
		}
	}
}

// The page is reachable by anyone on the network, so its routes are the whole exposure.
func TestWebExposesOnlyWhatThePageUses(t *testing.T) {
	refused := []struct{ method, path string }{
		{http.MethodPost, "/api/v1/ingest/readings"},
		{http.MethodPost, "/api/v1/ingest/waveforms"},
		{http.MethodPost, "/api/v1/gateway/sync"},
		{http.MethodPost, "/api/v1/gateways"},
		{http.MethodPost, "/api/v1/gateways/x/rotate-token"},
		{http.MethodGet, "/api/v1/gateways/x/config"},
		{http.MethodPut, "/api/v1/gateways/x/config"},
		{http.MethodPost, "/api/v1/gateways/x/firmware"},
		{http.MethodPost, "/api/v1/devices"},
		{http.MethodPost, "/api/v1/devices/x/commands"},
		{http.MethodGet, "/api/v1/tls/fingerprint"},
	}
	for _, route := range refused {
		w := httptest.NewRecorder()
		webHandler().ServeHTTP(w, httptest.NewRequest(route.method, route.path, nil))
		if w.Code != http.StatusNotFound && w.Code != http.StatusMethodNotAllowed {
			t.Errorf("%s %s: %d", route.method, route.path, w.Code)
		}
	}
}

func TestWebRoutesRequireASession(t *testing.T) {
	protected := []struct{ method, path string }{
		{http.MethodGet, "/api/v1/gateways"},
		{http.MethodGet, "/api/v1/devices"},
		{http.MethodGet, "/api/v1/devices/x/readings"},
		{http.MethodGet, "/api/v1/devices/x/config"},
		{http.MethodPut, "/api/v1/devices/x/config"},
		{http.MethodGet, "/api/v1/collections"},
		{http.MethodPost, "/api/v1/collections"},
		{http.MethodPut, "/api/v1/collections/1"},
		{http.MethodGet, "/api/v1/collections/1/summary"},
		{http.MethodPost, "/api/v1/auth/logout"},
	}
	for _, route := range protected {
		w := httptest.NewRecorder()
		webHandler().ServeHTTP(w, httptest.NewRequest(route.method, route.path, nil))
		if w.Code != http.StatusUnauthorized {
			t.Errorf("%s %s: %d", route.method, route.path, w.Code)
		}
	}
}
