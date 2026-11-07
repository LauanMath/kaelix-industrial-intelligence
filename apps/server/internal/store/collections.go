package store

import (
	"context"
	"slices"
	"time"

	"github.com/jackc/pgx/v5"
	c "kaelix/contracts"
)

const collectionColumns = `id,name,device_id,condition,notes,started_at,ended_at`

func scanCollection(row pgx.Row) (c.Collection, error) {
	var out c.Collection
	err := row.Scan(&out.ID, &out.Name, &out.DeviceID, &out.Condition, &out.Notes, &out.StartedAt, &out.EndedAt)
	return out, err
}

// CreateCollection starts a collection now, unless the request sets another start.
func (s *Store) CreateCollection(ctx context.Context, input c.CollectionInput) (c.Collection, error) {
	if err := s.DeviceExists(ctx, input.DeviceID); err != nil {
		return c.Collection{}, err
	}
	started := time.Now().UTC()
	if input.StartedAt != nil {
		started = input.StartedAt.UTC()
	}
	out, err := scanCollection(s.pool.QueryRow(ctx, `INSERT INTO collections(name,device_id,condition,notes,started_at,ended_at) VALUES($1,$2,$3,$4,$5,$6) RETURNING `+collectionColumns, input.Name, input.DeviceID, input.Condition, input.Notes, started, input.EndedAt))
	return out, dbError(err)
}

// UpdateCollection replaces the fields of a collection. An absent start keeps the stored one.
func (s *Store) UpdateCollection(ctx context.Context, id int64, input c.CollectionInput) (c.Collection, error) {
	out, err := scanCollection(s.pool.QueryRow(ctx, `UPDATE collections SET name=$2,condition=$3,notes=$4,started_at=COALESCE($5,started_at),ended_at=$6 WHERE id=$1 RETURNING `+collectionColumns, id, input.Name, input.Condition, input.Notes, input.StartedAt, input.EndedAt))
	return out, dbError(err)
}

// Collection returns one collection.
func (s *Store) Collection(ctx context.Context, id int64) (c.Collection, error) {
	out, err := scanCollection(s.pool.QueryRow(ctx, `SELECT `+collectionColumns+` FROM collections WHERE id=$1`, id))
	return out, dbError(err)
}

// Collections lists collections, the latest started first.
func (s *Store) Collections(ctx context.Context) ([]c.Collection, error) {
	rows, err := s.pool.Query(ctx, `SELECT `+collectionColumns+` FROM collections ORDER BY started_at DESC,id DESC LIMIT 500`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []c.Collection{}
	for rows.Next() {
		item, err := scanCollection(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

// SequenceGaps counts the jumps in the device sequence and the readings they lost. Within
// one boot the sequence only grows, so the values are sorted first and a late arrival is not
// a gap. Without a boot counter (packet v2) a reset can restart the sequence, so the arrival
// order stays and only forward jumps count.
func SequenceGaps(sequence []int64, sameBoot bool) (gaps, missing int64) {
	if sameBoot {
		sequence = slices.Clone(sequence)
		slices.Sort(sequence)
	}
	for i := 1; i < len(sequence); i++ {
		if jump := sequence[i] - sequence[i-1] - 1; jump > 0 {
			gaps++
			missing += jump
		}
	}
	return gaps, missing
}

type accumulator struct {
	n             int64
	min, max, sum float64
}

func (a *accumulator) add(value float64) {
	if a.n == 0 || value < a.min {
		a.min = value
	}
	if a.n == 0 || value > a.max {
		a.max = value
	}
	a.sum += value
	a.n++
}
func (a accumulator) result() *c.Range {
	if a.n == 0 {
		return nil
	}
	return &c.Range{Min: a.min, Avg: a.sum / float64(a.n), Max: a.max}
}

// CollectionSummary gathers the losses and the link quality of a collection.
func (s *Store) CollectionSummary(ctx context.Context, id int64) (c.CollectionSummary, error) {
	collection, err := s.Collection(ctx, id)
	if err != nil {
		return c.CollectionSummary{}, err
	}
	to := time.Now().UTC()
	if collection.EndedAt != nil {
		to = *collection.EndedAt
	}
	out := c.CollectionSummary{Collection: collection, From: collection.StartedAt, To: to}
	rows, err := s.pool.Query(ctx, `SELECT (payload->>'boot_count')::bigint,(payload->>'device_sequence')::bigint,COALESCE((payload->>'flags')::int,0),(payload->>'rssi_dbm')::float8,(payload->>'snr_db')::float8 FROM readings WHERE device_id=$1 AND estimated_received_at>=$2 AND estimated_received_at<$3 ORDER BY estimated_received_at,id`, collection.DeviceID, out.From, out.To)
	if err != nil {
		return out, err
	}
	defer rows.Close()
	var rssi, snr accumulator
	boots := map[int64][]int64{}
	var noBoot []int64
	for rows.Next() {
		var boot *int64
		var sequence int64
		var flags int
		var signal, noise float64
		if err = rows.Scan(&boot, &sequence, &flags, &signal, &noise); err != nil {
			return out, err
		}
		out.Readings++
		if flags&c.FlagNoMeasurement != 0 {
			out.ReadingsWithoutMeasurement++
		}
		if flags&c.FlagSaturated != 0 {
			out.ReadingsSaturated++
		}
		rssi.add(signal)
		snr.add(noise)
		if boot == nil {
			noBoot = append(noBoot, sequence)
		} else {
			boots[*boot] = append(boots[*boot], sequence)
		}
	}
	if err = rows.Err(); err != nil {
		return out, err
	}
	out.RSSIDBm, out.SNRDB = rssi.result(), snr.result()
	out.SequenceGaps, out.SequenceMissing = SequenceGaps(noBoot, false)
	for _, sequence := range boots {
		gaps, missing := SequenceGaps(sequence, true)
		out.SequenceGaps += gaps
		out.SequenceMissing += missing
	}
	if err = s.pool.QueryRow(ctx, `SELECT count(*),count(*) FILTER (WHERE jsonb_array_length(metadata->'fragments_missing')>0),COALESCE(sum((metadata->>'fragment_total')::bigint),0),COALESCE(sum(jsonb_array_length(metadata->'fragments_missing')),0) FROM waveforms WHERE device_id=$1 AND estimated_received_at>=$2 AND estimated_received_at<$3`, collection.DeviceID, out.From, out.To).Scan(&out.Waveforms, &out.WaveformsIncomplete, &out.FragmentsTotal, &out.FragmentsMissing); err != nil {
		return out, err
	}
	if err = s.pool.QueryRow(ctx, `SELECT (SELECT count(*) FROM readings WHERE device_id=$1 AND estimated_received_at IS NULL AND ingested_at>=$2 AND ingested_at<$3),(SELECT count(*) FROM waveforms WHERE device_id=$1 AND estimated_received_at IS NULL AND ingested_at>=$2 AND ingested_at<$3)`, collection.DeviceID, out.From, out.To).Scan(&out.ReadingsUndated, &out.WaveformsUndated); err != nil {
		return out, err
	}
	return out, nil
}
