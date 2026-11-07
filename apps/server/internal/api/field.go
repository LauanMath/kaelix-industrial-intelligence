package api

import (
	"bytes"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"strconv"
	"strings"
	"time"

	c "kaelix/contracts"
	"kaelix/server/internal/store"
)

const (
	maxFirmwareBytes = 8 << 20
	// A gateway downloads over a slow Wi-Fi link, well past the normal write deadline.
	firmwareWriteTimeout = 10 * time.Minute
	espImageMagic        = 0xE9
)

func validateWaveform(u c.WaveformUpload) ([]byte, error) {
	if !uuidID.MatchString(u.GatewayID) {
		return nil, invalid("gateway_id", "must be a lowercase UUID")
	}
	if !uuidID.MatchString(u.GatewaySessionID) {
		return nil, invalid("gateway_session_id", "must be a lowercase UUID")
	}
	if u.UploadUptimeMS < 0 || u.UploadUptimeMS > maxSafeInteger {
		return nil, invalid("upload_uptime_ms", "outside the safe integer range")
	}
	if u.GatewayRxUptimeMS < 0 || u.GatewayRxUptimeMS > u.UploadUptimeMS {
		return nil, invalid("gateway_rx_uptime_ms", "must not exceed upload_uptime_ms")
	}
	if u.EventSeq < 1 || u.EventSeq > maxSequence {
		return nil, invalid("event_seq", "must be a positive uint32")
	}
	if u.GatewayReceivedAt != nil {
		if _, offset := u.GatewayReceivedAt.Zone(); offset != 0 {
			return nil, invalid("gateway_received_at", "must be UTC")
		}
	}
	if !deviceID.MatchString(u.DeviceID) {
		return nil, invalid("device_id", "must be eight uppercase hexadecimal characters")
	}
	if u.Channels < 1 || u.Channels > 63 {
		return nil, invalid("channels", "must be a mask from 1 to 63")
	}
	if !c.ValidWindowSamples(u.WindowSamples) {
		return nil, invalid("window_samples", "must be 256, 512, 1024, 2048 or 4096")
	}
	if u.SampleRateHz == nil || math.IsNaN(*u.SampleRateHz) || math.IsInf(*u.SampleRateHz, 0) || *u.SampleRateHz <= 0 {
		return nil, invalid("sample_rate_hz", "must be a positive finite number")
	}
	for name, value := range map[string]struct{ value, max int }{"accel_range": {u.AccelRange, 3}, "gyro_range": {u.GyroRange, 3}, "dlpf": {u.DLPF, 6}, "burst_profile": {u.BurstProfile, 3}} {
		if value.value < 0 || value.value > value.max {
			return nil, invalid(name, fmt.Sprintf("must be between 0 and %d", value.max))
		}
	}
	if u.FragmentTotal != c.FragmentCount(u.WindowSamples, u.Channels) {
		return nil, invalid("fragment_total", fmt.Sprintf("must be %d for this window and channel mask", c.FragmentCount(u.WindowSamples, u.Channels)))
	}
	seen := map[int]bool{}
	for _, index := range u.FragmentsMissing {
		if index < 0 || index >= u.FragmentTotal || seen[index] {
			return nil, invalid("fragments_missing", "must hold unique indexes below fragment_total")
		}
		seen[index] = true
	}
	samples, err := base64.StdEncoding.Strict().DecodeString(u.SamplesB64)
	if err != nil {
		return nil, invalid("samples_b64", "must be standard base64")
	}
	if want := c.WindowBytes(u.WindowSamples, u.Channels); len(samples) != want {
		return nil, invalid("samples_b64", fmt.Sprintf("must decode to exactly %d bytes", want))
	}
	return samples, nil
}

func (a *API) ingestWaveform(w http.ResponseWriter, r *http.Request) {
	var u c.WaveformUpload
	if !decode(w, r, &u) {
		return
	}
	if u.SchemaVersion != 1 {
		problem(w, 422, "UNSUPPORTED_SCHEMA", "schema_version must be 1")
		return
	}
	samples, err := validateWaveform(u)
	if err != nil {
		validationProblem(w, err)
		return
	}
	if u.GatewayID != r.Context().Value(gatewayKey{}) {
		problem(w, 403, "forbidden", "gateway identity does not match credential")
		return
	}
	result, err := a.store.IngestWaveform(r.Context(), u, samples)
	if err != nil {
		a.fail(w, err)
		return
	}
	write(w, 200, result)
}

func validateSync(s c.SyncRequest) error {
	if !uuidID.MatchString(s.GatewayID) {
		return invalid("gateway_id", "must be a lowercase UUID")
	}
	if !uuidID.MatchString(s.GatewaySessionID) {
		return invalid("gateway_session_id", "must be a lowercase UUID")
	}
	if s.ConfigVersion < 0 || s.ConfigVersion > c.MaxConfigVersion {
		return invalid("config_version", "must be between 0 and 65535")
	}
	if s.FirmwareVersion == "" || !shortText(s.FirmwareVersion, 64) {
		return invalid("firmware_version", "must contain 1 to 64 characters")
	}
	if s.OTAResult != nil {
		if s.OTAResult.Version == "" || !shortText(s.OTAResult.Version, 64) {
			return invalid("ota_result.version", "must contain 1 to 64 characters")
		}
		if !shortText(s.OTAResult.Reason, 256) {
			return invalid("ota_result.reason", "must contain up to 256 characters")
		}
	}
	if len(s.Devices) > 64 {
		return invalid("devices", "must contain up to 64 devices")
	}
	for i, device := range s.Devices {
		if !deviceID.MatchString(device.DeviceID) {
			return invalid(fmt.Sprintf("devices[%d].device_id", i), "must be eight uppercase hexadecimal characters")
		}
		if value := device.RadiatedConfigVersion; value != nil && (*value < 0 || *value > c.MaxConfigVersion) {
			return invalid(fmt.Sprintf("devices[%d].radiated_config_version", i), "must be between 0 and 65535")
		}
		if value := device.RadiatedCommandSeq; value != nil && (*value < 0 || *value > c.MaxCommandSeq) {
			return invalid(fmt.Sprintf("devices[%d].radiated_command_seq", i), "must be between 0 and 65535")
		}
	}
	return nil
}

func (a *API) gatewaySync(w http.ResponseWriter, r *http.Request) {
	var s c.SyncRequest
	if !decode(w, r, &s) {
		return
	}
	if s.SchemaVersion != 1 {
		problem(w, 422, "UNSUPPORTED_SCHEMA", "schema_version must be 1")
		return
	}
	if err := validateSync(s); err != nil {
		validationProblem(w, err)
		return
	}
	if s.GatewayID != r.Context().Value(gatewayKey{}) {
		problem(w, 403, "forbidden", "gateway identity does not match credential")
		return
	}
	result, secretsErr, err := a.store.Sync(r.Context(), s)
	if err != nil {
		a.fail(w, err)
		return
	}
	if secretsErr != nil {
		a.logger.Error("gateway configuration not delivered: Wi-Fi passwords cannot be opened", "gateway", s.GatewayID)
	}
	write(w, 200, result)
}

func (a *API) gatewayFirmwareDownload(w http.ResponseWriter, r *http.Request) {
	gateway, _ := r.Context().Value(gatewayKey{}).(string)
	item, data, err := a.store.GatewayFirmwareData(r.Context(), gateway, r.PathValue("id"))
	if err != nil {
		a.fail(w, err)
		return
	}
	if err = http.NewResponseController(w).SetWriteDeadline(time.Now().Add(firmwareWriteTimeout)); err != nil {
		a.logger.Warn("could not extend firmware write deadline", "error", err)
	}
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("ETag", `"`+item.SHA256+`"`)
	http.ServeContent(w, r, "", time.Time{}, bytes.NewReader(data))
}

// configProblem reports every invalid field at once, so a form can mark all of them.
func configProblem(w http.ResponseWriter, issues []c.ErrorDetail) {
	write(w, 422, c.APIError{SchemaVersion: 1, Error: c.ErrorBody{Code: "INVALID_REQUEST", Message: "configuration validation failed", Retryable: false, Details: issues}})
}

func (a *API) deviceConfig(w http.ResponseWriter, r *http.Request) {
	out, err := a.store.DeviceConfig(r.Context(), r.PathValue("id"))
	if err != nil {
		a.fail(w, err)
		return
	}
	write(w, 200, out)
}

func (a *API) saveDeviceConfig(w http.ResponseWriter, r *http.Request) {
	// Decoding over the defaults gives an absent field its default, never zero.
	cfg := c.DefaultDeviceConfig()
	if !decode(w, r, &cfg) {
		return
	}
	if cfg.Routine == nil {
		cfg.Routine = []c.RoutineWindow{}
	}
	if issues := c.ValidateDeviceConfig(cfg); len(issues) > 0 {
		configProblem(w, issues)
		return
	}
	out, err := a.store.SaveDeviceConfig(r.Context(), r.PathValue("id"), cfg)
	if err != nil {
		a.fail(w, err)
		return
	}
	write(w, 200, out)
}

func (a *API) deviceCommands(w http.ResponseWriter, r *http.Request) {
	items, err := a.store.Commands(r.Context(), r.PathValue("id"))
	if err != nil {
		a.fail(w, err)
		return
	}
	write(w, 200, c.DeviceCommandList{Items: items})
}

func (a *API) createDeviceCommand(w http.ResponseWriter, r *http.Request) {
	var input c.DeviceCommandInput
	if !decode(w, r, &input) {
		return
	}
	if !c.ValidCommandKind(input.Kind) {
		configProblem(w, []c.ErrorDetail{{Path: "kind", Reason: "O comando deve ser reboot, factory_reset ou capture_now"}})
		return
	}
	out, err := a.store.CreateCommand(r.Context(), r.PathValue("id"), input.Kind)
	if err != nil {
		a.fail(w, err)
		return
	}
	write(w, 201, out)
}

func (a *API) deviceWaveforms(w http.ResponseWriter, r *http.Request) {
	device := r.PathValue("id")
	if err := a.store.DeviceExists(r.Context(), device); err != nil {
		a.fail(w, err)
		return
	}
	f, err := historyFilter(r, device)
	if err != nil {
		problem(w, 400, "invalid_query", err.Error())
		return
	}
	page, err := a.store.Waveforms(r.Context(), device, f)
	if err != nil {
		a.fail(w, err)
		return
	}
	write(w, 200, page)
}

func pathID(r *http.Request) (int64, bool) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	return id, err == nil && id > 0
}

func (a *API) waveform(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r)
	if !ok {
		a.fail(w, store.ErrNotFound)
		return
	}
	out, err := a.store.Waveform(r.Context(), id)
	if err != nil {
		a.fail(w, err)
		return
	}
	write(w, 200, out)
}

func (a *API) gatewayConfig(w http.ResponseWriter, r *http.Request) {
	out, err := a.store.GatewayConfig(r.Context(), r.PathValue("id"))
	if err != nil {
		a.fail(w, err)
		return
	}
	write(w, 200, out)
}

func (a *API) saveGatewayConfig(w http.ResponseWriter, r *http.Request) {
	input := c.DefaultGatewayConfig()
	if !decode(w, r, &input) {
		return
	}
	if issues := c.ValidateGatewayConfig(input); len(issues) > 0 {
		configProblem(w, issues)
		return
	}
	out, err := a.store.SaveGatewayConfig(r.Context(), r.PathValue("id"), input)
	if err != nil {
		a.fail(w, err)
		return
	}
	write(w, 200, out)
}

func (a *API) gatewayFirmwareStatus(w http.ResponseWriter, r *http.Request) {
	out, err := a.store.GatewayFirmwareStatus(r.Context(), r.PathValue("id"))
	if err != nil {
		a.fail(w, err)
		return
	}
	write(w, 200, out)
}

// uploadGatewayFirmware takes the raw binary as the body and the version as a query
// parameter, because the JSON limits do not suit a binary of megabytes.
func (a *API) uploadGatewayFirmware(w http.ResponseWriter, r *http.Request) {
	if strings.TrimSpace(strings.Split(r.Header.Get("Content-Type"), ";")[0]) != "application/octet-stream" {
		problem(w, 400, "unsupported_media_type", "use application/octet-stream")
		return
	}
	version := strings.TrimSpace(r.URL.Query().Get("version"))
	if version == "" || !shortText(version, 64) {
		configProblem(w, []c.ErrorDetail{{Path: "version", Reason: "Informe a versão do firmware, com até 64 caracteres"}})
		return
	}
	data, err := io.ReadAll(http.MaxBytesReader(w, r.Body, maxFirmwareBytes))
	if err != nil {
		var large *http.MaxBytesError
		if errors.As(err, &large) {
			problem(w, 413, "body_too_large", "firmware exceeds 8 MiB")
		} else {
			problem(w, 400, "invalid_json", "could not read request body")
		}
		return
	}
	if len(data) == 0 || data[0] != espImageMagic {
		configProblem(w, []c.ErrorDetail{{Path: "body", Reason: "O arquivo não é uma imagem de firmware ESP32"}})
		return
	}
	out, err := a.store.SaveGatewayFirmware(r.Context(), r.PathValue("id"), version, data)
	if err != nil {
		a.fail(w, err)
		return
	}
	write(w, 201, out)
}

func (a *API) tlsFingerprint(w http.ResponseWriter, r *http.Request) {
	if a.fingerprint == "" {
		problem(w, 503, "unavailable", "gateway listener not configured")
		return
	}
	write(w, 200, c.TLSFingerprint{SHA256: a.fingerprint})
}

func validateCollection(input *c.CollectionInput) []c.ErrorDetail {
	issues := []c.ErrorDetail{}
	input.Name = strings.TrimSpace(input.Name)
	input.Condition = strings.TrimSpace(input.Condition)
	if input.Name == "" || !shortText(input.Name, 160) {
		issues = append(issues, c.ErrorDetail{Path: "name", Reason: "Informe um nome com até 160 caracteres"})
	}
	if !shortText(input.Condition, 160) {
		issues = append(issues, c.ErrorDetail{Path: "condition", Reason: "A condição deve ter até 160 caracteres"})
	}
	if !shortText(input.Notes, 2000) {
		issues = append(issues, c.ErrorDetail{Path: "notes", Reason: "As observações devem ter até 2000 caracteres"})
	}
	if input.StartedAt != nil && input.EndedAt != nil && !input.EndedAt.After(*input.StartedAt) {
		issues = append(issues, c.ErrorDetail{Path: "ended_at", Reason: "O fim precisa ser posterior ao início"})
	}
	return issues
}

func (a *API) collections(w http.ResponseWriter, r *http.Request) {
	items, err := a.store.Collections(r.Context())
	if err != nil {
		a.fail(w, err)
		return
	}
	write(w, 200, c.CollectionList{Items: items})
}

func (a *API) createCollection(w http.ResponseWriter, r *http.Request) {
	var input c.CollectionInput
	if !decode(w, r, &input) {
		return
	}
	issues := validateCollection(&input)
	if !deviceID.MatchString(input.DeviceID) {
		issues = append(issues, c.ErrorDetail{Path: "device_id", Reason: "Escolha um transmissor"})
	}
	if len(issues) > 0 {
		configProblem(w, issues)
		return
	}
	out, err := a.store.CreateCollection(r.Context(), input)
	if err != nil {
		a.fail(w, err)
		return
	}
	write(w, 201, out)
}

func (a *API) updateCollection(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r)
	if !ok {
		a.fail(w, store.ErrNotFound)
		return
	}
	var input c.CollectionInput
	if !decode(w, r, &input) {
		return
	}
	current, err := a.store.Collection(r.Context(), id)
	if err != nil {
		a.fail(w, err)
		return
	}
	issues := validateCollection(&input)
	if input.DeviceID != "" && input.DeviceID != current.DeviceID {
		issues = append(issues, c.ErrorDetail{Path: "device_id", Reason: "O transmissor de uma coleta não muda"})
	}
	if input.StartedAt == nil && input.EndedAt != nil && !input.EndedAt.After(current.StartedAt) {
		issues = append(issues, c.ErrorDetail{Path: "ended_at", Reason: "O fim precisa ser posterior ao início"})
	}
	if len(issues) > 0 {
		configProblem(w, issues)
		return
	}
	out, err := a.store.UpdateCollection(r.Context(), id, input)
	if err != nil {
		a.fail(w, err)
		return
	}
	write(w, 200, out)
}

func (a *API) collectionSummary(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r)
	if !ok {
		a.fail(w, store.ErrNotFound)
		return
	}
	out, err := a.store.CollectionSummary(r.Context(), id)
	if err != nil {
		a.fail(w, err)
		return
	}
	write(w, 200, out)
}
