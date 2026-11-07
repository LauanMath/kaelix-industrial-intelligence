package store

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	c "kaelix/contracts"
)

// ErrSecretsUnavailable means the server has no key to seal or open Wi-Fi passwords.
var ErrSecretsUnavailable = errors.New("secret key unavailable")

// ErrCommandsExhausted means a device has used the whole command sequence range.
var ErrCommandsExhausted = errors.New("command sequence exhausted")

// Sealer encrypts secrets at rest. secure.Box implements it.
type Sealer interface {
	Seal(plaintext, additional []byte) (string, error)
	Open(sealed string, additional []byte) ([]byte, error)
}

// UseSealer sets the key holder. It must be called before the store serves requests.
func (s *Store) UseSealer(sealer Sealer) { s.sealer = sealer }

// ConfigState derives the state of a device configuration. Only the telemetry makes a
// configuration applied: having been transmitted is never enough.
func ConfigState(desired, radiated, reported *int) *string {
	if desired == nil {
		return nil
	}
	state := c.StatePending
	switch {
	case reported != nil && *reported == *desired:
		state = c.StateApplied
	case radiated != nil && *radiated == *desired:
		state = c.StateRadiated
	}
	return &state
}

// nextVersion assigns the version of a new configuration. It stays away from the version
// the far end reports: reusing that number would read as applied before anything is sent.
func nextVersion(current, reported *int) int {
	next := 0
	if current != nil {
		next = *current
	}
	if reported != nil && *reported > next {
		next = *reported
	}
	next++
	if next > c.MaxConfigVersion {
		next = 1
	}
	if reported != nil && next == *reported {
		next++
	}
	return next
}

type deviceConfigRow struct {
	version, radiatedVersion, reportedVersion, reportedCommandSeq *int
	config                                                        []byte
	updatedAt, radiatedAt, reportedAt, appliedAt                  *time.Time
	reportedSession                                               *string
	reportedEventSeq, reportedBootCount, reportedSequence         *int64
}

const deviceConfigColumns = `version,config,radiated_version,reported_version,reported_command_seq,updated_at,radiated_at,reported_at,applied_at,reported_session_id,reported_event_seq,reported_boot_count,reported_sequence`

func scanDeviceConfig(row pgx.Row) (deviceConfigRow, error) {
	var r deviceConfigRow
	err := row.Scan(&r.version, &r.config, &r.radiatedVersion, &r.reportedVersion, &r.reportedCommandSeq, &r.updatedAt, &r.radiatedAt, &r.reportedAt, &r.appliedAt, &r.reportedSession, &r.reportedEventSeq, &r.reportedBootCount, &r.reportedSequence)
	return r, err
}

func (r deviceConfigRow) desired() (*c.DeviceConfig, error) {
	if r.version == nil {
		return nil, nil
	}
	cfg := c.DefaultDeviceConfig()
	if err := json.Unmarshal(r.config, &cfg); err != nil {
		return nil, err
	}
	cfg.Version = *r.version
	return &cfg, nil
}

func (r deviceConfigRow) state(device string) (c.DeviceConfigState, error) {
	out := c.DeviceConfigState{DeviceID: device, State: ConfigState(r.version, r.radiatedVersion, r.reportedVersion), RadiatedVersion: r.radiatedVersion, ReportedVersion: r.reportedVersion, ReportedCommandSeq: r.reportedCommandSeq, UpdatedAt: r.updatedAt, RadiatedAt: r.radiatedAt, ReportedAt: r.reportedAt, AppliedAt: r.appliedAt, Defaults: c.DefaultDeviceConfig()}
	var err error
	out.Config, err = r.desired()
	return out, err
}

// DeviceConfig returns the desired configuration, what the transmitter reports and the state.
func (s *Store) DeviceConfig(ctx context.Context, device string) (c.DeviceConfigState, error) {
	if err := s.DeviceExists(ctx, device); err != nil {
		return c.DeviceConfigState{}, err
	}
	r, err := scanDeviceConfig(s.pool.QueryRow(ctx, `SELECT `+deviceConfigColumns+` FROM device_configs WHERE device_id=$1`, device))
	if errors.Is(err, pgx.ErrNoRows) {
		return deviceConfigRow{}.state(device)
	}
	if err != nil {
		return c.DeviceConfigState{}, err
	}
	return r.state(device)
}

// lockDeviceConfig serializes every change to one device's configuration and commands.
func lockDeviceConfig(ctx context.Context, tx pgx.Tx, device string) (deviceConfigRow, error) {
	var known bool
	if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM devices WHERE id=$1)`, device).Scan(&known); err != nil {
		return deviceConfigRow{}, err
	}
	if !known {
		return deviceConfigRow{}, ErrNotFound
	}
	if _, err := tx.Exec(ctx, `INSERT INTO device_configs(device_id) VALUES($1) ON CONFLICT DO NOTHING`, device); err != nil {
		return deviceConfigRow{}, err
	}
	return scanDeviceConfig(tx.QueryRow(ctx, `SELECT `+deviceConfigColumns+` FROM device_configs WHERE device_id=$1 FOR UPDATE`, device))
}

// setDesired stores a new configuration under a new version and clears what the previous
// version had reached.
func setDesired(ctx context.Context, tx pgx.Tx, device string, r *deviceConfigRow, cfg c.DeviceConfig, now time.Time) error {
	content, err := c.MarshalConfigContent(cfg)
	if err != nil {
		return err
	}
	next := nextVersion(r.version, r.reportedVersion)
	if _, err = tx.Exec(ctx, `UPDATE device_configs SET version=$2,config=$3,updated_at=$4,radiated_version=NULL,radiated_at=NULL,applied_at=NULL WHERE device_id=$1`, device, next, content, now); err != nil {
		return err
	}
	r.version, r.config, r.updatedAt, r.radiatedVersion, r.radiatedAt, r.appliedAt = &next, content, &now, nil, nil, nil
	return nil
}

// SaveDeviceConfig replaces the desired configuration. Saving the same content is not an
// alteration: it keeps the version, so a repeated click does not transmit anything again.
func (s *Store) SaveDeviceConfig(ctx context.Context, device string, cfg c.DeviceConfig) (c.DeviceConfigState, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return c.DeviceConfigState{}, err
	}
	defer tx.Rollback(ctx)
	r, err := lockDeviceConfig(ctx, tx, device)
	if err != nil {
		return c.DeviceConfigState{}, err
	}
	current, err := r.desired()
	if err != nil {
		return c.DeviceConfigState{}, err
	}
	same := false
	if current != nil {
		before, err := c.MarshalConfigContent(*current)
		if err != nil {
			return c.DeviceConfigState{}, err
		}
		after, err := c.MarshalConfigContent(cfg)
		if err != nil {
			return c.DeviceConfigState{}, err
		}
		same = bytes.Equal(before, after)
	}
	if !same {
		if err = setDesired(ctx, tx, device, &r, cfg, time.Now().UTC()); err != nil {
			return c.DeviceConfigState{}, err
		}
	}
	if err = tx.Commit(ctx); err != nil {
		return c.DeviceConfigState{}, err
	}
	return r.state(device)
}

// CreateCommand queues a command. Its seq stays above what the transmitter reports, or it
// would be taken as already executed. A factory reset also replaces the desired
// configuration with the defaults under a new version, so the old one does not come back.
func (s *Store) CreateCommand(ctx context.Context, device, kind string) (c.DeviceCommand, error) {
	out := c.DeviceCommand{DeviceID: device, Kind: kind, State: c.StatePending}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return out, err
	}
	defer tx.Rollback(ctx)
	r, err := lockDeviceConfig(ctx, tx, device)
	if err != nil {
		return out, err
	}
	var last int
	if err = tx.QueryRow(ctx, `SELECT COALESCE(MAX(seq),0) FROM device_commands WHERE device_id=$1`, device).Scan(&last); err != nil {
		return out, err
	}
	if r.reportedCommandSeq != nil && *r.reportedCommandSeq > last {
		last = *r.reportedCommandSeq
	}
	if last >= c.MaxCommandSeq {
		return out, ErrCommandsExhausted
	}
	out.Seq = last + 1
	now := time.Now().UTC()
	out.CreatedAt = now
	if _, err = tx.Exec(ctx, `INSERT INTO device_commands(device_id,seq,kind,created_at) VALUES($1,$2,$3,$4)`, device, out.Seq, kind, now); err != nil {
		return out, dbError(err)
	}
	if kind == c.CommandFactoryReset {
		if err = setDesired(ctx, tx, device, &r, c.DefaultDeviceConfig(), now); err != nil {
			return out, err
		}
	}
	return out, tx.Commit(ctx)
}

// Commands lists the latest commands of a device, newest first.
func (s *Store) Commands(ctx context.Context, device string) ([]c.DeviceCommand, error) {
	if err := s.DeviceExists(ctx, device); err != nil {
		return nil, err
	}
	rows, err := s.pool.Query(ctx, `SELECT seq,kind,state,created_at,radiated_at,applied_at FROM device_commands WHERE device_id=$1 ORDER BY seq DESC LIMIT 100`, device)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []c.DeviceCommand{}
	for rows.Next() {
		command := c.DeviceCommand{DeviceID: device}
		if err = rows.Scan(&command.Seq, &command.Kind, &command.State, &command.CreatedAt, &command.RadiatedAt, &command.AppliedAt); err != nil {
			return nil, err
		}
		out = append(out, command)
	}
	return out, rows.Err()
}

// reportAdvances decides whether a stored reading may replace the reported version. A
// device may legitimately go back (it reverts a configuration on link loss), so a live
// reading always counts unless the same session already reported something newer. A
// reading from a previous boot of the gateway is backlog: it only counts when the device's
// own counters show it is newer than what is on record.
func reportAdvances(r deviceConfigRow, session string, previous bool, reading c.Reading) bool {
	if r.reportedVersion == nil {
		return true
	}
	if previous {
		if reading.BootCount == nil || reading.DeviceSequence == nil || r.reportedBootCount == nil || r.reportedSequence == nil {
			return false
		}
		boot, seq := int64(*reading.BootCount), int64(*reading.DeviceSequence)
		return boot > *r.reportedBootCount || (boot == *r.reportedBootCount && seq > *r.reportedSequence)
	}
	if r.reportedSession != nil && r.reportedEventSeq != nil && *r.reportedSession == session {
		return reading.EventSeq > *r.reportedEventSeq
	}
	return true
}

// applyTelemetry records what a newly stored reading confirms: the configuration version
// in use and the last command executed.
func applyTelemetry(ctx context.Context, tx pgx.Tx, session string, previous bool, reading c.Reading, now time.Time) error {
	if reading.ConfigVersion == nil && reading.CommandSeq == nil {
		return nil
	}
	if _, err := tx.Exec(ctx, `INSERT INTO device_configs(device_id) VALUES($1) ON CONFLICT DO NOTHING`, reading.DeviceID); err != nil {
		return err
	}
	r, err := scanDeviceConfig(tx.QueryRow(ctx, `SELECT `+deviceConfigColumns+` FROM device_configs WHERE device_id=$1 FOR UPDATE`, reading.DeviceID))
	if err != nil {
		return err
	}
	if reading.ConfigVersion != nil && reportAdvances(r, session, previous, reading) {
		var boot *int64
		if reading.BootCount != nil {
			value := int64(*reading.BootCount)
			boot = &value
		}
		var sequence *int64
		if reading.DeviceSequence != nil {
			value := int64(*reading.DeviceSequence)
			sequence = &value
		}
		if _, err = tx.Exec(ctx, `UPDATE device_configs SET reported_version=$2,reported_at=$3,reported_session_id=$4,reported_event_seq=$5,reported_boot_count=$6,reported_sequence=$7,applied_at=CASE WHEN version=$2 THEN COALESCE(applied_at,$3) ELSE NULL END WHERE device_id=$1`, reading.DeviceID, int(*reading.ConfigVersion), now, session, reading.EventSeq, boot, sequence); err != nil {
			return err
		}
	}
	if reading.CommandSeq != nil {
		seq := int(*reading.CommandSeq)
		if _, err = tx.Exec(ctx, `UPDATE device_configs SET reported_command_seq=GREATEST(COALESCE(reported_command_seq,0),$2) WHERE device_id=$1`, reading.DeviceID, seq); err != nil {
			return err
		}
		if _, err = tx.Exec(ctx, `UPDATE device_commands SET state='applied',applied_at=$3 WHERE device_id=$1 AND seq<=$2 AND state<>'applied'`, reading.DeviceID, seq, now); err != nil {
			return err
		}
	}
	return nil
}

type storedWifi struct {
	SSID   string `json:"ssid"`
	Sealed string `json:"sealed,omitempty"`
}
type storedGatewayConfig struct {
	Wifi               []storedWifi `json:"wifi"`
	HeartbeatIntervalS int          `json:"heartbeat_interval_s"`
	SyncIntervalS      int          `json:"sync_interval_s"`
	APWhenOffline      bool         `json:"ap_when_offline"`
	TxPowerDBm         int          `json:"tx_power_dbm"`
}

func (cfg storedGatewayConfig) view(version int) *c.GatewayConfigView {
	out := &c.GatewayConfigView{Version: version, Wifi: []c.WifiStatus{}, HeartbeatIntervalS: cfg.HeartbeatIntervalS, SyncIntervalS: cfg.SyncIntervalS, APWhenOffline: cfg.APWhenOffline, TxPowerDBm: cfg.TxPowerDBm}
	for _, network := range cfg.Wifi {
		out.Wifi = append(out.Wifi, c.WifiStatus{SSID: network.SSID, HasPassword: network.Sealed != ""})
	}
	return out
}

func wifiBinding(gateway, ssid string) []byte { return []byte(gateway + "\x00" + ssid) }

func gatewayDefaults() c.GatewayConfigView {
	base := c.DefaultGatewayConfig()
	return c.GatewayConfigView{Wifi: []c.WifiStatus{}, HeartbeatIntervalS: base.HeartbeatIntervalS, SyncIntervalS: base.SyncIntervalS, APWhenOffline: base.APWhenOffline, TxPowerDBm: base.TxPowerDBm}
}

const gatewayConfigColumns = `version,config,reported_version,updated_at,reported_at,applied_at,last_sync_at`

func scanGatewayConfig(gateway string, row pgx.Row) (c.GatewayConfigState, *storedGatewayConfig, error) {
	out := c.GatewayConfigState{GatewayID: gateway, Defaults: gatewayDefaults()}
	var version *int
	var raw []byte
	if err := row.Scan(&version, &raw, &out.ReportedVersion, &out.UpdatedAt, &out.ReportedAt, &out.AppliedAt, &out.LastSyncAt); err != nil {
		return out, nil, err
	}
	if version == nil {
		return out, nil, nil
	}
	var stored storedGatewayConfig
	if err := json.Unmarshal(raw, &stored); err != nil {
		return out, nil, err
	}
	out.Config = stored.view(*version)
	state := c.StatePending
	if out.ReportedVersion != nil && *out.ReportedVersion == *version {
		state = c.StateApplied
	}
	out.State = &state
	return out, &stored, nil
}

func (s *Store) gatewayExists(ctx context.Context, q interface {
	QueryRow(context.Context, string, ...any) pgx.Row
}, gateway string) error {
	var known bool
	if err := q.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM gateways WHERE id=$1)`, gateway).Scan(&known); err != nil {
		return err
	}
	if !known {
		return ErrNotFound
	}
	return nil
}

// GatewayConfig returns the desired configuration without any password.
func (s *Store) GatewayConfig(ctx context.Context, gateway string) (c.GatewayConfigState, error) {
	if err := s.gatewayExists(ctx, s.pool, gateway); err != nil {
		return c.GatewayConfigState{}, err
	}
	out, _, err := scanGatewayConfig(gateway, s.pool.QueryRow(ctx, `SELECT `+gatewayConfigColumns+` FROM gateway_configs WHERE gateway_id=$1`, gateway))
	if errors.Is(err, pgx.ErrNoRows) {
		return c.GatewayConfigState{GatewayID: gateway, Defaults: gatewayDefaults()}, nil
	}
	return out, err
}

// sealNetworks turns the requested networks into stored ones. An absent password keeps the
// stored one of the same network; an unchanged one keeps its ciphertext, so saving the
// same form again is not an alteration.
func (s *Store) sealNetworks(gateway string, input []c.WifiCredential, old *storedGatewayConfig) ([]storedWifi, error) {
	previous := map[string]string{}
	if old != nil {
		for _, network := range old.Wifi {
			previous[network.SSID] = network.Sealed
		}
	}
	out := make([]storedWifi, 0, len(input))
	for _, network := range input {
		binding := wifiBinding(gateway, network.SSID)
		sealed := previous[network.SSID]
		if network.Password != nil {
			keep := false
			if sealed != "" && s.sealer != nil {
				if plain, err := s.sealer.Open(sealed, binding); err == nil {
					keep = string(plain) == *network.Password
				}
			}
			if !keep {
				sealed = ""
				if *network.Password != "" {
					if s.sealer == nil {
						return nil, ErrSecretsUnavailable
					}
					var err error
					if sealed, err = s.sealer.Seal([]byte(*network.Password), binding); err != nil {
						return nil, err
					}
				}
			}
		}
		out = append(out, storedWifi{SSID: network.SSID, Sealed: sealed})
	}
	return out, nil
}

// SaveGatewayConfig replaces the desired configuration and assigns its version.
func (s *Store) SaveGatewayConfig(ctx context.Context, gateway string, input c.GatewayConfigInput) (c.GatewayConfigState, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return c.GatewayConfigState{}, err
	}
	defer tx.Rollback(ctx)
	if err = s.gatewayExists(ctx, tx, gateway); err != nil {
		return c.GatewayConfigState{}, err
	}
	if _, err = tx.Exec(ctx, `INSERT INTO gateway_configs(gateway_id) VALUES($1) ON CONFLICT DO NOTHING`, gateway); err != nil {
		return c.GatewayConfigState{}, err
	}
	current, old, err := scanGatewayConfig(gateway, tx.QueryRow(ctx, `SELECT `+gatewayConfigColumns+` FROM gateway_configs WHERE gateway_id=$1 FOR UPDATE`, gateway))
	if err != nil {
		return c.GatewayConfigState{}, err
	}
	networks, err := s.sealNetworks(gateway, input.Wifi, old)
	if err != nil {
		return c.GatewayConfigState{}, err
	}
	next := storedGatewayConfig{Wifi: networks, HeartbeatIntervalS: input.HeartbeatIntervalS, SyncIntervalS: input.SyncIntervalS, APWhenOffline: input.APWhenOffline, TxPowerDBm: input.TxPowerDBm}
	content, err := json.Marshal(next)
	if err != nil {
		return c.GatewayConfigState{}, err
	}
	if old != nil {
		if before, err := json.Marshal(*old); err == nil && bytes.Equal(before, content) {
			return current, tx.Commit(ctx)
		}
	}
	var version *int
	if current.Config != nil {
		version = &current.Config.Version
	}
	assigned := nextVersion(version, current.ReportedVersion)
	now := time.Now().UTC()
	if _, err = tx.Exec(ctx, `UPDATE gateway_configs SET version=$2,config=$3,updated_at=$4,applied_at=NULL WHERE gateway_id=$1`, gateway, assigned, content, now); err != nil {
		return c.GatewayConfigState{}, err
	}
	state := c.StatePending
	current.Config, current.State, current.UpdatedAt, current.AppliedAt = next.view(assigned), &state, &now, nil
	return current, tx.Commit(ctx)
}

// deliverGatewayConfig opens the passwords for the authenticated gateway. This is the only
// path by which a password leaves the database.
func (s *Store) deliverGatewayConfig(gateway string, version int, raw []byte) (*c.GatewayConfigDelivery, error) {
	var stored storedGatewayConfig
	if err := json.Unmarshal(raw, &stored); err != nil {
		return nil, err
	}
	out := &c.GatewayConfigDelivery{Version: version, Wifi: []c.WifiSecret{}, HeartbeatIntervalS: stored.HeartbeatIntervalS, SyncIntervalS: stored.SyncIntervalS, APWhenOffline: stored.APWhenOffline, TxPowerDBm: stored.TxPowerDBm}
	for _, network := range stored.Wifi {
		password := ""
		if network.Sealed != "" {
			if s.sealer == nil {
				return nil, ErrSecretsUnavailable
			}
			plain, err := s.sealer.Open(network.Sealed, wifiBinding(gateway, network.SSID))
			if err != nil {
				return nil, ErrSecretsUnavailable
			}
			password = string(plain)
		}
		out.Wifi = append(out.Wifi, c.WifiSecret{SSID: network.SSID, Password: password})
	}
	return out, nil
}
