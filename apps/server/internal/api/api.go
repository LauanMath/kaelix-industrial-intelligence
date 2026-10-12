package api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	c "kaelix/contracts"
	"kaelix/server/internal/store"
)

type API struct {
	store    *store.Store
	logger   *slog.Logger
	loginMu  sync.Mutex
	attempts map[string]attempt
}
type attempt struct {
	Count int
	Start time.Time
}

func New(s *store.Store, logger *slog.Logger) http.Handler {
	a := &API{store: s, logger: logger, attempts: map[string]attempt{}}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) { write(w, 200, map[string]string{"status": "ok"}) })
	mux.HandleFunc("GET /readyz", a.ready)
	mux.HandleFunc("POST /api/v1/auth/login", a.login)
	mux.Handle("POST /api/v1/auth/logout", a.user(http.HandlerFunc(a.logout)))
	mux.Handle("POST /api/v1/ingest/readings", a.gateway(http.HandlerFunc(a.ingest)))
	mux.Handle("POST /api/v1/ingest/logs", a.gateway(http.HandlerFunc(a.ingestLogs)))
	mux.Handle("GET /api/v1/gateways/{id}/logs", a.user(http.HandlerFunc(a.gatewayLogs)))
	mux.Handle("POST /api/v1/ingest/heartbeat", a.gateway(http.HandlerFunc(a.heartbeat)))
	mux.Handle("GET /api/v1/readings/recent",a.user(http.HandlerFunc(a.recentReadings)))
 mux.Handle("GET /api/v1/devices", a.user(http.HandlerFunc(a.devices)))
	mux.Handle("POST /api/v1/devices", a.user(http.HandlerFunc(a.createDevice)))
	mux.Handle("PUT /api/v1/devices/{id}", a.user(http.HandlerFunc(a.updateDevice)))
	mux.Handle("GET /api/v1/devices/{id}/readings", a.user(http.HandlerFunc(a.deviceHistory)))
	mux.Handle("GET /api/v1/motors", a.user(http.HandlerFunc(a.motors)))
	mux.Handle("POST /api/v1/motors", a.user(http.HandlerFunc(a.createMotor)))
	mux.Handle("PUT /api/v1/motors/{id}", a.user(http.HandlerFunc(a.updateMotor)))
	mux.Handle("GET /api/v1/motors/{id}/readings", a.user(http.HandlerFunc(a.motorHistory)))
	mux.Handle("GET /api/v1/gateways", a.user(http.HandlerFunc(a.gateways)))
	mux.Handle("POST /api/v1/gateways", a.user(http.HandlerFunc(a.createGateway)))
	mux.Handle("POST /api/v1/gateways/{id}/rotate-token", a.user(http.HandlerFunc(a.rotateGatewayToken)))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		defer func() {
			if v := recover(); v != nil {
				a.logger.Error("request panic", "method", r.Method, "path", r.URL.Path)
				problem(w, 500, "internal_error", "request failed")
			}
		}()
		ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
		defer cancel()
		mux.ServeHTTP(w, r.WithContext(ctx))
	})
}
func write(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
func problem(w http.ResponseWriter, status int, code, message string) {
	codes := map[string]string{"unsupported_media_type": "INVALID_REQUEST", "invalid_json": "INVALID_REQUEST", "validation_error": "INVALID_REQUEST", "invalid_query": "INVALID_REQUEST", "unauthorized": "UNAUTHORIZED", "forbidden": "FORBIDDEN", "conflict": "EVENT_CONFLICT", "body_too_large": "PAYLOAD_TOO_LARGE", "rate_limited": "RATE_LIMITED", "unavailable": "SERVICE_UNAVAILABLE", "internal_error": "SERVICE_UNAVAILABLE", "not_found": "INVALID_REQUEST"}
	if mapped, ok := codes[code]; ok {
		code = mapped
	}
	write(w, status, c.APIError{SchemaVersion: 1, Error: c.ErrorBody{Code: code, Message: message, Retryable: status == 429 || status >= 500, Details: []c.ErrorDetail{}}})
}
func validationProblem(w http.ResponseWriter, err error) {
	var v *validationError
	if errors.As(err, &v) {
		write(w, 422, c.APIError{SchemaVersion: 1, Error: c.ErrorBody{Code: "INVALID_REQUEST", Message: "request validation failed", Retryable: false, Details: []c.ErrorDetail{{Path: v.Path, Reason: v.Reason}}}})
		return
	}
	problem(w, 422, "validation_error", err.Error())
}
func decode(w http.ResponseWriter, r *http.Request, value any) bool {
	if strings.TrimSpace(strings.Split(r.Header.Get("Content-Type"), ";")[0]) != "application/json" {
		problem(w, 400, "unsupported_media_type", "use application/json")
		return false
	}
	if encoding := r.Header.Get("Content-Encoding"); encoding != "" && encoding != "identity" {
		problem(w, 400, "invalid_json", "compressed JSON is not supported")
		return false
	}
	limit := int64(64 << 10)
	if strings.HasPrefix(r.URL.Path, "/api/v1/ingest/") {
		limit = 32 << 10
	}
	r.Body = http.MaxBytesReader(w, r.Body, limit)
	raw, err := io.ReadAll(r.Body)
	if err != nil {
		var large *http.MaxBytesError
		if errors.As(err, &large) {
			problem(w, 413, "body_too_large", "request body exceeds endpoint limit")
		} else {
			problem(w, 400, "invalid_json", "could not read request body")
		}
		return false
	}
	if err = c.ValidateJSON(raw); err != nil {
		problem(w, 400, "invalid_json", "invalid JSON, duplicate keys or trailing data")
		return false
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err = decoder.Decode(value); err != nil {
		problem(w, 400, "invalid_json", "invalid JSON body, missing required fields or unknown fields")
		return false
	}
	return true
}
func (a *API) fail(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, store.ErrNotFound):
		problem(w, 404, "not_found", "resource not found")
	case errors.Is(err, store.ErrConflict):
		problem(w, 409, "conflict", "identifier already exists, device is already associated, or event payload differs")
	case errors.Is(err, store.ErrInvalidHeartbeat):
		problem(w, 422, "validation_error", "heartbeat uptime or counters decreased within session")
	case errors.Is(err, store.ErrUnauthorized):
		problem(w, 401, "unauthorized", "invalid credentials")
	default:
		a.logger.Error("database operation failed", "error", err)
		problem(w, 503, "unavailable", "database operation unavailable; retry later")
	}
}
func bearer(r *http.Request) string {
	s := r.Header.Get("Authorization")
	if !strings.HasPrefix(s, "Bearer ") {
		return ""
	}
	s = strings.TrimPrefix(s, "Bearer ")
	if len(s) != 43 {
		return ""
	}
	return s
}
func (a *API) user(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token := bearer(r)
		if token == "" {
			problem(w, 401, "unauthorized", "user session required")
			return
		}
		ok, err := a.store.UserAuthorized(r.Context(), token)
		if err != nil {
			a.fail(w, err)
			return
		}
		if !ok {
			problem(w, 401, "unauthorized", "invalid or expired user session")
			return
		}
		next.ServeHTTP(w, r)
	})
}

type gatewayKey struct{}

func (a *API) gateway(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token := bearer(r)
		if token == "" {
			problem(w, 401, "unauthorized", "gateway credential required")
			return
		}
		id, err := a.store.GatewayAuthorized(r.Context(), token)
		if errors.Is(err, store.ErrNotFound) {
			problem(w, 401, "unauthorized", "invalid gateway credential")
			return
		}
		if err != nil {
			a.fail(w, err)
			return
		}
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), gatewayKey{}, id)))
	})
}
func (a *API) ready(w http.ResponseWriter, r *http.Request) {
	if err := a.store.Ping(r.Context()); err != nil {
		problem(w, 503, "unavailable", "database unavailable")
		return
	}
	write(w, 200, map[string]string{"status": "ready"})
}
func (a *API) allowLogin(r *http.Request) bool {
	ip, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		ip = r.RemoteAddr
	}
	now := time.Now()
	a.loginMu.Lock()
	defer a.loginMu.Unlock()
	for key, value := range a.attempts {
		if now.Sub(value.Start) >= time.Minute {
			delete(a.attempts, key)
		}
	}
	value := a.attempts[ip]
	if value.Start.IsZero() {
		value.Start = now
	}
	// Bound memory even when clients supply many source addresses. Forwarded headers are not trusted.
	if _, exists := a.attempts[ip]; !exists && len(a.attempts) >= 4096 {
		return false
	}
	if value.Count >= 10 {
		return false
	}
	value.Count++
	a.attempts[ip] = value
	return true
}
func (a *API) login(w http.ResponseWriter, r *http.Request) {
	if !a.allowLogin(r) {
		w.Header().Set("Retry-After", "60")
		problem(w, 429, "rate_limited", "too many login attempts")
		return
	}
	var body struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if !decode(w, r, &body) {
		return
	}
	body.Email = strings.ToLower(strings.TrimSpace(body.Email))
	if body.Email == "" || len(body.Email) > 254 || len(body.Password) < 1 || len(body.Password) > 72 {
		problem(w, 400, "validation_error", "invalid email or password length")
		return
	}
	token, expiry, err := a.store.Login(r.Context(), body.Email, body.Password)
	if err != nil {
		a.fail(w, err)
		return
	}
	write(w, 200, map[string]any{"access_token": token, "token_type": "Bearer", "expires_at": expiry})
}
func (a *API) logout(w http.ResponseWriter, r *http.Request) {
	if err := a.store.Logout(r.Context(), bearer(r)); err != nil {
		a.fail(w, err)
		return
	}
	w.WriteHeader(204)
}
func (a *API) ingest(w http.ResponseWriter, r *http.Request) {
	var b c.ReadingBatch
	if !decode(w, r, &b) {
		return
	}
	if b.SchemaVersion != 1 {
		problem(w, 422, "UNSUPPORTED_SCHEMA", "schema_version must be 1")
		return
	}
	if err := validateBatch(b, time.Now().UTC()); err != nil {
		validationProblem(w, err)
		return
	}
	if b.GatewayID != r.Context().Value(gatewayKey{}) {
		problem(w, 403, "forbidden", "gateway identity does not match credential")
		return
	}
	result, err := a.store.Ingest(r.Context(), b)
	if err != nil {
		a.fail(w, err)
		return
	}
	write(w, 200, result)
}
func (a *API) heartbeat(w http.ResponseWriter, r *http.Request) {
	var h c.Heartbeat
	if !decode(w, r, &h) {
		return
	}
	if h.SchemaVersion != 1 {
		problem(w, 422, "UNSUPPORTED_SCHEMA", "schema_version must be 1")
		return
	}
	if err := validateHeartbeat(h); err != nil {
		validationProblem(w, err)
		return
	}
	if h.GatewayID != r.Context().Value(gatewayKey{}) {
		problem(w, 403, "forbidden", "gateway identity does not match credential")
		return
	}
	result, err := a.store.SaveHeartbeat(r.Context(), h)
	if err != nil {
		a.fail(w, err)
		return
	}
	write(w, 200, result)
}
func (a *API) devices(w http.ResponseWriter, r *http.Request) {
	items, err := a.store.Devices(r.Context())
	if err != nil {
		a.fail(w, err)
		return
	}
	write(w, 200, map[string]any{"items": items})
}
func (a *API) createDevice(w http.ResponseWriter, r *http.Request) { a.saveDevice(w, r, false) }
func (a *API) updateDevice(w http.ResponseWriter, r *http.Request) { a.saveDevice(w, r, true) }
func (a *API) saveDevice(w http.ResponseWriter, r *http.Request, update bool) {
	var d c.DeviceInput
	if !decode(w, r, &d) {
		return
	}
	if update {
		if d.ID != "" && d.ID != r.PathValue("id") {
			problem(w, 422, "validation_error", "id differs from path")
			return
		}
		d.ID = r.PathValue("id")
	}
	if err := validateDevice(d); err != nil {
		problem(w, 422, "validation_error", err.Error())
		return
	}
	var out c.Device
	var err error
	status := 201
	if update {
		out, err = a.store.UpdateDevice(r.Context(), d)
		status = 200
	} else {
		out, err = a.store.CreateDevice(r.Context(), d)
	}
	if err != nil {
		a.fail(w, err)
		return
	}
	write(w, status, out)
}
func (a *API) motors(w http.ResponseWriter, r *http.Request) {
	items, err := a.store.Motors(r.Context())
	if err != nil {
		a.fail(w, err)
		return
	}
	write(w, 200, map[string]any{"items": items})
}
func (a *API) createMotor(w http.ResponseWriter, r *http.Request) { a.saveMotor(w, r, false) }
func (a *API) updateMotor(w http.ResponseWriter, r *http.Request) { a.saveMotor(w, r, true) }
func (a *API) saveMotor(w http.ResponseWriter, r *http.Request, update bool) {
	var m c.MotorInput
	if !decode(w, r, &m) {
		return
	}
	if update {
		if m.ID != "" && m.ID != r.PathValue("id") {
			problem(w, 422, "validation_error", "id differs from path")
			return
		}
		m.ID = r.PathValue("id")
	}
	if err := validateMotor(&m); err != nil {
		problem(w, 422, "validation_error", err.Error())
		return
	}
	out, err := a.store.SaveMotor(r.Context(), m, update)
	if err != nil {
		a.fail(w, err)
		return
	}
	status := 201
	if update {
		status = 200
	}
	write(w, status, out)
}
func (a *API) gateways(w http.ResponseWriter, r *http.Request) {
	items, err := a.store.Gateways(r.Context())
	if err != nil {
		a.fail(w, err)
		return
	}
	write(w, 200, map[string]any{"items": items})
}
func (a *API) createGateway(w http.ResponseWriter, r *http.Request) {
	var g c.GatewayInput
	if !decode(w, r, &g) {
		return
	}
	if err := validateGateway(&g); err != nil {
		problem(w, 422, "validation_error", err.Error())
		return
	}
	out, token, err := a.store.CreateGateway(r.Context(), g)
	if err != nil {
		a.fail(w, err)
		return
	}
	write(w, 201, map[string]any{"gateway": out, "gateway_token": token})
}
func (a *API) rotateGatewayToken(w http.ResponseWriter, r *http.Request) {
	token, err := a.store.RotateGatewayToken(r.Context(), r.PathValue("id"))
	if err != nil {
		a.fail(w, err)
		return
	}
	write(w, 200, map[string]string{"gateway_token": token})
}
func (a *API) motorHistory(w http.ResponseWriter, r *http.Request) {
	device, err := a.store.MotorDevice(r.Context(), r.PathValue("id"))
	if err != nil {
		a.fail(w, err)
		return
	}
	if device == nil { // Validate query parameters even when no sensor has been associated.
		if _, err := historyFilter(r, ""); err != nil {
			problem(w, 400, "invalid_query", err.Error())
			return
		}
		write(w, 200, c.HistoryPage{Items: []c.Measurement{}})
		return
	}
	a.history(w, r, *device)
}
func (a *API) deviceHistory(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if err := a.store.DeviceExists(r.Context(), id); err != nil {
		a.fail(w, err)
		return
	}
	a.history(w, r, id)
}
func historyFilter(r *http.Request, device string) (store.HistoryFilter, error) {
	f := store.HistoryFilter{Limit: 100}
	q := r.URL.Query()
	for key, values := range q {
		if key != "limit" && key != "cursor" && key != "from" && key != "to" {
			return f, errors.New("unknown query parameter")
		}
		if len(values) != 1 {
			return f, errors.New("repeated query parameter")
		}
	}
	if q.Has("limit") {
		limit, err := strconv.Atoi(q.Get("limit"))
		if err != nil || limit < 1 || limit > 1000 {
			return f, errors.New("limit must be between 1 and 1000")
		}
		f.Limit = limit
	}
	for key, target := range map[string]**time.Time{"from": &f.From, "to": &f.To} {
		if q.Has(key) {
			t, err := time.Parse(time.RFC3339Nano, q.Get(key))
			if err != nil {
				return f, errors.New("from and to must be RFC3339 timestamps")
			}
			t = t.UTC()
			*target = &t
		}
	}
	if f.From != nil && f.To != nil && !f.From.Before(*f.To) {
		return f, errors.New("from must precede to")
	}
	var err error
	f.Cursor, err = store.ParseCursor(q.Get("cursor"), device)
	return f, err
}
func (a *API) history(w http.ResponseWriter, r *http.Request, device string) {
	f, err := historyFilter(r, device)
	if err != nil {
		problem(w, 400, "invalid_query", err.Error())
		return
	}
	page, err := a.store.History(r.Context(), device, f)
	if err != nil {
		a.fail(w, err)
		return
	}
	write(w, 200, page)
}

func (a *API) recentReadings(w http.ResponseWriter,r *http.Request){
 items,err:=a.store.RecentReadings(r.Context());if err!=nil{a.fail(w,err);return};write(w,200,map[string]any{"items":items})
}
