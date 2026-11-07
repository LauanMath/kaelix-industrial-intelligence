package api

import (
	_ "embed"
	"log/slog"
	"net/http"

	"kaelix/server/internal/store"
)

//go:embed web/index.html
var collectionPage []byte

// NewWeb returns the handler of the collection page on the local network: the page and only
// the routes it uses. A phone has no use for gateway routes, credentials or firmware.
func NewWeb(s *store.Store, logger *slog.Logger) http.Handler {
	a := &API{store: s, logger: logger, attempts: map[string]attempt{}}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /{$}", servePage)
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) { write(w, 200, map[string]string{"status": "ok"}) })
	mux.HandleFunc("POST /api/v1/auth/login", a.login)
	mux.Handle("POST /api/v1/auth/logout", a.user(http.HandlerFunc(a.logout)))
	mux.Handle("GET /api/v1/gateways", a.user(http.HandlerFunc(a.gateways)))
	mux.Handle("GET /api/v1/devices", a.user(http.HandlerFunc(a.devices)))
	mux.Handle("GET /api/v1/devices/{id}/readings", a.user(http.HandlerFunc(a.deviceHistory)))
	mux.Handle("GET /api/v1/devices/{id}/config", a.user(http.HandlerFunc(a.deviceConfig)))
	mux.Handle("PUT /api/v1/devices/{id}/config", a.user(http.HandlerFunc(a.saveDeviceConfig)))
	mux.Handle("GET /api/v1/collections", a.user(http.HandlerFunc(a.collections)))
	mux.Handle("POST /api/v1/collections", a.user(http.HandlerFunc(a.createCollection)))
	mux.Handle("PUT /api/v1/collections/{id}", a.user(http.HandlerFunc(a.updateCollection)))
	mux.Handle("GET /api/v1/collections/{id}/summary", a.user(http.HandlerFunc(a.collectionSummary)))
	return a.wrap(mux)
}

func servePage(w http.ResponseWriter, r *http.Request) {
	h := w.Header()
	h.Set("Content-Type", "text/html; charset=utf-8")
	h.Set("Content-Security-Policy", "default-src 'none'; script-src 'unsafe-inline'; style-src 'unsafe-inline'; connect-src 'self'; form-action 'none'; base-uri 'none'; frame-ancestors 'none'")
	h.Set("Referrer-Policy", "no-referrer")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(collectionPage)
}
