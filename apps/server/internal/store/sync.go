package store

import (
	"context"
	"encoding/json"
	"errors"
	"slices"
	"time"

	"github.com/jackc/pgx/v5"
	c "kaelix/contracts"
)

const firmwareOpenStates = `('pending','offered','downloaded')`

// Sync records what the gateway reports and answers with what it must relay. A failure to
// open the Wi-Fi passwords must not stop the clock and the transmitters, so it is returned
// next to a usable response, with gateway_config null.
func (s *Store) Sync(ctx context.Context, req c.SyncRequest) (c.SyncResponse, error, error) {
	now := time.Now().UTC()
	out := c.SyncResponse{SchemaVersion: 1, GatewayID: req.GatewayID, GatewaySessionID: req.GatewaySessionID, ServerTime: now.Format(c.SyncTimeLayout), Devices: []c.SyncDeviceUpdate{}}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return out, nil, err
	}
	defer tx.Rollback(ctx)
	// Same order as the ingestion transactions: gateway first, then devices.
	var locked string
	if err = tx.QueryRow(ctx, `SELECT id FROM gateways WHERE id=$1 FOR UPDATE`, req.GatewayID).Scan(&locked); err != nil {
		return out, nil, dbError(err)
	}
	var version *int
	var raw []byte
	err = tx.QueryRow(ctx, `INSERT INTO gateway_configs(gateway_id,reported_version,reported_at,firmware_version,last_sync_at) VALUES($1,$2,$3,$4,$3) ON CONFLICT(gateway_id) DO UPDATE SET reported_version=$2,reported_at=$3,firmware_version=$4,last_sync_at=$3,applied_at=CASE WHEN gateway_configs.version=$2 THEN COALESCE(gateway_configs.applied_at,$3) ELSE NULL END RETURNING version,config`, req.GatewayID, req.ConfigVersion, now, req.FirmwareVersion).Scan(&version, &raw)
	if err != nil {
		return out, nil, err
	}
	devices := slices.Clone(req.Devices)
	slices.SortFunc(devices, func(a, b c.SyncDevice) int {
		if a.DeviceID < b.DeviceID {
			return -1
		}
		if a.DeviceID > b.DeviceID {
			return 1
		}
		return 0
	})
	for _, device := range devices {
		if device.RadiatedConfigVersion != nil {
			if _, err = tx.Exec(ctx, `UPDATE device_configs SET radiated_version=$2,radiated_at=CASE WHEN version=$2 AND radiated_version IS DISTINCT FROM $2 THEN $3 ELSE radiated_at END WHERE device_id=$1`, device.DeviceID, *device.RadiatedConfigVersion, now); err != nil {
				return out, nil, err
			}
		}
		if device.RadiatedCommandSeq != nil {
			if _, err = tx.Exec(ctx, `UPDATE device_commands SET state='radiated',radiated_at=$3 WHERE device_id=$1 AND seq<=$2 AND state='pending'`, device.DeviceID, *device.RadiatedCommandSeq, now); err != nil {
				return out, nil, err
			}
		}
	}
	if err = recordFirmwareResult(ctx, tx, req, now); err != nil {
		return out, nil, err
	}
	updates, err := pendingForRadio(ctx, tx)
	if err != nil {
		return out, nil, err
	}
	out.Devices = updates
	if out.Firmware, err = offerFirmware(ctx, tx, req.GatewayID, now); err != nil {
		return out, nil, err
	}
	if err = tx.Commit(ctx); err != nil {
		return out, nil, err
	}
	var secretsErr error
	if version != nil && *version != req.ConfigVersion {
		delivery, err := s.deliverGatewayConfig(req.GatewayID, *version, raw)
		if errors.Is(err, ErrSecretsUnavailable) {
			secretsErr = err
		} else if err != nil {
			return out, nil, err
		} else {
			out.GatewayConfig = delivery
		}
	}
	return out, secretsErr, nil
}

// pendingForRadio lists the devices with a configuration the telemetry has not confirmed or
// a command it has not confirmed. A command already radiated keeps coming: a lost reply
// would otherwise lose the command, and the transmitter ignores a seq it has executed.
func pendingForRadio(ctx context.Context, tx pgx.Tx) ([]c.SyncDeviceUpdate, error) {
	updates := map[string]*c.SyncDeviceUpdate{}
	rows, err := tx.Query(ctx, `SELECT device_id,version,config FROM device_configs WHERE version IS NOT NULL AND version IS DISTINCT FROM reported_version`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var device string
		var version int
		var raw []byte
		if err = rows.Scan(&device, &version, &raw); err != nil {
			return nil, err
		}
		cfg := c.DefaultDeviceConfig()
		if err = json.Unmarshal(raw, &cfg); err != nil {
			return nil, err
		}
		cfg.Version = version
		updates[device] = &c.SyncDeviceUpdate{DeviceID: device, Config: &cfg}
	}
	if err = rows.Err(); err != nil {
		return nil, err
	}
	rows.Close()
	rows, err = tx.Query(ctx, `SELECT DISTINCT ON (device_id) device_id,seq,kind FROM device_commands WHERE state<>'applied' ORDER BY device_id,seq`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var device string
		var command c.SyncCommand
		if err = rows.Scan(&device, &command.Seq, &command.Kind); err != nil {
			return nil, err
		}
		if updates[device] == nil {
			updates[device] = &c.SyncDeviceUpdate{DeviceID: device}
		}
		updates[device].Command = &command
	}
	if err = rows.Err(); err != nil {
		return nil, err
	}
	ids := make([]string, 0, len(updates))
	for id := range updates {
		ids = append(ids, id)
	}
	slices.Sort(ids)
	out := make([]c.SyncDeviceUpdate, 0, len(ids))
	for _, id := range ids {
		out = append(out, *updates[id])
	}
	return out, nil
}

// recordFirmwareResult closes an update. A failure closes it too: offering the same binary
// again after the gateway rolled back would repeat the failure forever.
func recordFirmwareResult(ctx context.Context, tx pgx.Tx, req c.SyncRequest, now time.Time) error {
	if req.OTAResult != nil {
		state, reason := c.FirmwareFailed, req.OTAResult.Reason
		if req.OTAResult.OK {
			state = c.FirmwareApplied
		}
		if _, err := tx.Exec(ctx, `UPDATE gateway_firmware SET state=$3,reason=$4,finished_at=$5,data=NULL WHERE gateway_id=$1 AND version=$2 AND state IN `+firmwareOpenStates, req.GatewayID, req.OTAResult.Version, state, reason, now); err != nil {
			return err
		}
	}
	_, err := tx.Exec(ctx, `UPDATE gateway_firmware SET state='applied',finished_at=$3,data=NULL WHERE gateway_id=$1 AND version=$2 AND state IN `+firmwareOpenStates, req.GatewayID, req.FirmwareVersion, now)
	return err
}

func offerFirmware(ctx context.Context, tx pgx.Tx, gateway string, now time.Time) (*c.FirmwareOffer, error) {
	var id string
	var offer c.FirmwareOffer
	err := tx.QueryRow(ctx, `SELECT id,version,sha256,size FROM gateway_firmware WHERE gateway_id=$1 AND state IN `+firmwareOpenStates+` ORDER BY created_at DESC LIMIT 1`, gateway).Scan(&id, &offer.Version, &offer.SHA256, &offer.Size)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	offer.Path = "/api/v1/gateway/firmware/" + id
	if _, err = tx.Exec(ctx, `UPDATE gateway_firmware SET state='offered',offered_at=$2 WHERE id=$1 AND state='pending'`, id, now); err != nil {
		return nil, err
	}
	return &offer, nil
}
