package store

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"time"

	"github.com/jackc/pgx/v5"
	c "kaelix/contracts"
)

const firmwareColumns = `id,gateway_id,version,sha256,size,state,reason,created_at,offered_at,downloaded_at,finished_at`

func scanFirmware(row pgx.Row) (c.GatewayFirmware, error) {
	var f c.GatewayFirmware
	err := row.Scan(&f.ID, &f.GatewayID, &f.Version, &f.SHA256, &f.Size, &f.State, &f.Reason, &f.CreatedAt, &f.OfferedAt, &f.DownloadedAt, &f.FinishedAt)
	return f, err
}

// SaveGatewayFirmware makes a binary the target of a gateway and replaces any update in
// progress. The previous binaries are dropped; their rows stay as history.
func (s *Store) SaveGatewayFirmware(ctx context.Context, gateway, version string, data []byte) (c.GatewayFirmware, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return c.GatewayFirmware{}, err
	}
	defer tx.Rollback(ctx)
	if err = s.gatewayExists(ctx, tx, gateway); err != nil {
		return c.GatewayFirmware{}, err
	}
	now := time.Now().UTC()
	if _, err = tx.Exec(ctx, `UPDATE gateway_firmware SET state='superseded',finished_at=$2,data=NULL WHERE gateway_id=$1 AND state IN `+firmwareOpenStates, gateway, now); err != nil {
		return c.GatewayFirmware{}, err
	}
	sum := sha256.Sum256(data)
	out, err := scanFirmware(tx.QueryRow(ctx, `INSERT INTO gateway_firmware(id,gateway_id,version,sha256,size,data,created_at) VALUES($1,$2,$3,$4,$5,$6,$7) RETURNING `+firmwareColumns, NewUUID(), gateway, version, hex.EncodeToString(sum[:]), len(data), data, now))
	if err != nil {
		return c.GatewayFirmware{}, dbError(err)
	}
	return out, tx.Commit(ctx)
}

// GatewayFirmwareStatus returns the running version and the latest updates.
func (s *Store) GatewayFirmwareStatus(ctx context.Context, gateway string) (c.GatewayFirmwareStatus, error) {
	out := c.GatewayFirmwareStatus{GatewayID: gateway, Items: []c.GatewayFirmware{}}
	if err := s.gatewayExists(ctx, s.pool, gateway); err != nil {
		return out, err
	}
	if err := s.pool.QueryRow(ctx, `SELECT COALESCE((SELECT firmware_version FROM gateway_configs WHERE gateway_id=$1),heartbeat->>'firmware_version') FROM gateways WHERE id=$1`, gateway).Scan(&out.CurrentVersion); err != nil {
		return out, err
	}
	rows, err := s.pool.Query(ctx, `SELECT `+firmwareColumns+` FROM gateway_firmware WHERE gateway_id=$1 ORDER BY created_at DESC LIMIT 10`, gateway)
	if err != nil {
		return out, err
	}
	defer rows.Close()
	for rows.Next() {
		item, err := scanFirmware(rows)
		if err != nil {
			return out, err
		}
		out.Items = append(out.Items, item)
	}
	return out, rows.Err()
}

// GatewayFirmwareData returns a binary to the gateway it was uploaded for and records the
// download. A closed or foreign update is not found.
func (s *Store) GatewayFirmwareData(ctx context.Context, gateway, id string) (c.GatewayFirmware, []byte, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return c.GatewayFirmware{}, nil, err
	}
	defer tx.Rollback(ctx)
	var data []byte
	if err = tx.QueryRow(ctx, `SELECT data FROM gateway_firmware WHERE id=$1 AND gateway_id=$2 AND data IS NOT NULL AND state IN `+firmwareOpenStates+` FOR UPDATE`, id, gateway).Scan(&data); err != nil {
		return c.GatewayFirmware{}, nil, dbError(err)
	}
	out, err := scanFirmware(tx.QueryRow(ctx, `UPDATE gateway_firmware SET state='downloaded',downloaded_at=COALESCE(downloaded_at,now()) WHERE id=$1 RETURNING `+firmwareColumns, id))
	if err != nil {
		return c.GatewayFirmware{}, nil, err
	}
	return out, data, tx.Commit(ctx)
}
