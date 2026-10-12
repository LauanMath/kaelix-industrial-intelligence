package api

import (
	c "kaelix/contracts"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"unicode/utf8"
)

var logEvent = regexp.MustCompile(`^[A-Z][A-Z0-9_]{0,23}$`)

func validateGatewayLogs(b c.GatewayLogBatch) error {
	if b.SchemaVersion != 1 {
		return invalid("schema_version", "must be 1")
	}
	if !uuidID.MatchString(b.GatewayID) || !uuidID.MatchString(b.GatewaySessionID) {
		return invalid("gateway_id", "gateway and session must be lowercase UUIDs")
	}
	if len(b.Logs) < 1 || len(b.Logs) > 8 {
		return invalid("logs", "must contain 1 to 8 records")
	}
	seen := map[uint32]bool{}
	for _, e := range b.Logs {
		if e.LogSeq == 0 || seen[e.LogSeq] {
			return invalid("logs.log_seq", "must be positive and unique")
		}
		seen[e.LogSeq] = true
		if e.UptimeMS < 0 || e.UptimeMS > maxSafeInteger {
			return invalid("logs.uptime_ms", "outside safe integer range")
		}
		if e.Level != "DEBUG" && e.Level != "INFO" && e.Level != "WARN" && e.Level != "ERROR" {
			return invalid("logs.level", "unsupported severity")
		}
		if !logEvent.MatchString(e.Event) {
			return invalid("logs.event", "invalid event tag")
		}
		if !utf8.ValidString(e.Message) || len(e.Message) > 383 || strings.ContainsRune(e.Message, 0) {
			return invalid("logs.message", "must be UTF-8, at most 383 bytes, without NUL")
		}
	}
	return nil
}
func (a *API) ingestLogs(w http.ResponseWriter, r *http.Request) {
	var b c.GatewayLogBatch
	if !decode(w, r, &b) {
		return
	}
	if err := validateGatewayLogs(b); err != nil {
		validationProblem(w, err)
		return
	}
	if b.GatewayID != r.Context().Value(gatewayKey{}) {
		problem(w, 403, "forbidden", "gateway identity does not match credential")
		return
	}
	out, err := a.store.SaveGatewayLogs(r.Context(), b)
	if err != nil {
		a.fail(w, err)
		return
	}
	write(w, 200, out)
}
func (a *API) gatewayLogs(w http.ResponseWriter, r *http.Request) {
	gateway := r.PathValue("id")
	if !uuidID.MatchString(gateway) {
		problem(w, 400, "invalid_query", "invalid gateway")
		return
	}
	after := int64(0)
	limit := 200
	if raw := r.URL.Query().Get("after_id"); raw != "" {
		var err error
		after, err = strconv.ParseInt(raw, 10, 64)
		if err != nil || after < 0 {
			problem(w, 400, "invalid_query", "invalid cursor")
			return
		}
	}
	if raw := r.URL.Query().Get("limit"); raw != "" {
		var err error
		limit, err = strconv.Atoi(raw)
		if err != nil || limit < 1 || limit > 500 {
			problem(w, 400, "invalid_query", "limit must be 1 to 500")
			return
		}
	}
	out, err := a.store.GatewayLogs(r.Context(), gateway, after, limit)
	if err != nil {
		a.fail(w, err)
		return
	}
	write(w, 200, out)
}
