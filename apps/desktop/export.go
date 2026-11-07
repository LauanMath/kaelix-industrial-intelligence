package main

import (
	"context"
	"encoding/base64"
	"encoding/binary"
	"encoding/csv"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"
	"unicode"

	c "kaelix/contracts"
	"kaelix/internal/apiclient"
)

// ExportResult says what a finished export wrote.
type ExportResult struct {
	Folder          string `json:"folder"`
	Readings        int    `json:"readings"`
	Windows         int    `json:"windows"`
	ReadingsUndated int64  `json:"readingsUndated"`
	WindowsUndated  int64  `json:"windowsUndated"`
}

// ExportProgress reports each step while the data is paged from the API.
type ExportProgress struct {
	Stage string `json:"stage"`
	Done  int    `json:"done"`
	Total int    `json:"total"`
}

var channelNames = []string{"ax", "ay", "az", "gx", "gy", "gz"}

// exportCollection pages through all the data of a collection, not only what a screen has
// loaded. The folder only takes its final name when complete, so a failure never leaves
// something that looks like a whole export.
func exportCollection(ctx context.Context, cl *apiclient.Client, id int64, parent string, progress func(ExportProgress)) (ExportResult, error) {
	var summary c.CollectionSummary
	if err := cl.Request(ctx, "GET", fmt.Sprintf("/api/v1/collections/%d/summary", id), nil, &summary); err != nil {
		return ExportResult{}, err
	}
	name := exportFolderName(summary.Collection)
	work, err := os.MkdirTemp(parent, ".kaelix-exportacao-*")
	if err != nil {
		return ExportResult{}, errors.New("não foi possível criar a pasta da exportação")
	}
	defer os.RemoveAll(work)
	from, to := summary.From.UTC().Format(time.RFC3339Nano), summary.To.UTC().Format(time.RFC3339Nano)
	readings, err := writeReadings(ctx, cl, summary.Collection.DeviceID, from, to, work, progress)
	if err != nil {
		return ExportResult{}, err
	}
	windows, err := writeWindows(ctx, cl, summary.Collection.DeviceID, from, to, work, progress)
	if err != nil {
		return ExportResult{}, err
	}
	folder, err := placeFolder(work, parent, name)
	if err != nil {
		return ExportResult{}, errors.New("não foi possível criar a pasta da exportação")
	}
	return ExportResult{Folder: folder, Readings: readings, Windows: windows, ReadingsUndated: summary.ReadingsUndated, WindowsUndated: summary.WaveformsUndated}, nil
}

func placeFolder(work, parent, name string) (string, error) {
	for attempt := 1; attempt < 1000; attempt++ {
		folder := filepath.Join(parent, name)
		if attempt > 1 {
			folder += fmt.Sprintf("-%d", attempt)
		}
		if _, err := os.Stat(folder); errors.Is(err, os.ErrNotExist) {
			if err = os.Chmod(work, 0o755); err != nil {
				return "", err
			}
			return folder, os.Rename(work, folder)
		}
	}
	return "", errors.New("too many exports with this name")
}

func exportFolderName(collection c.Collection) string {
	var slug strings.Builder
	for _, r := range strings.ToLower(collection.Name) {
		switch {
		case r < unicode.MaxASCII && (unicode.IsLetter(r) || unicode.IsDigit(r)):
			slug.WriteRune(r)
		case slug.Len() > 0 && !strings.HasSuffix(slug.String(), "-"):
			slug.WriteByte('-')
		}
		if slug.Len() >= 40 {
			break
		}
	}
	name := strings.Trim(slug.String(), "-")
	if name == "" {
		return fmt.Sprintf("coleta-%d", collection.ID)
	}
	return fmt.Sprintf("coleta-%d-%s", collection.ID, name)
}

// A semicolon and a byte order mark, like the CSV the monitoring screen already exports.
func createCSV(path string) (*os.File, *csv.Writer, error) {
	file, err := os.Create(path)
	if err != nil {
		return nil, nil, errors.New("não foi possível gravar a exportação")
	}
	if _, err = file.WriteString("\xef\xbb\xbf"); err != nil {
		file.Close()
		return nil, nil, errors.New("não foi possível gravar a exportação")
	}
	writer := csv.NewWriter(file)
	writer.Comma = ';'
	return file, writer, nil
}

func closeCSV(file *os.File, writer *csv.Writer) error {
	writer.Flush()
	err := writer.Error()
	if closeErr := file.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		return errors.New("não foi possível gravar a exportação")
	}
	return nil
}

func number[T ~int | ~int8 | ~uint8 | ~uint16 | ~uint32 | ~int64](value *T) string {
	if value == nil {
		return ""
	}
	return strconv.FormatInt(int64(*value), 10)
}
func decimal(value *float64) string {
	if value == nil {
		return ""
	}
	return strconv.FormatFloat(*value, 'f', -1, 64)
}
func moment(value *time.Time) string {
	if value == nil {
		return ""
	}
	return value.UTC().Format(time.RFC3339Nano)
}

var readingColumns = []string{"device_id", "gateway_id", "gateway_session_id", "event_seq", "packet_version", "boot_count", "device_sequence", "state", "diag_code", "flags", "rms_g", "rms_x_g", "rms_y_g", "kurtosis", "crest_factor", "dominant_freq_hz", "sample_rate_hz", "temperature_c", "rssi_dbm", "snr_db", "config_version", "command_seq", "reset_reason", "uptime_s", "wave_fragments", "wave_samples", "channels", "tx_power_dbm", "packet_crc16", "device_time", "gateway_received_at", "estimated_received_at", "ingested_at", "queue_delay_ms"}

func readingRow(m c.Measurement) []string {
	ingested := m.IngestedAt
	return []string{m.DeviceID, m.GatewayID, m.GatewaySessionID, strconv.FormatInt(m.EventSeq, 10), strconv.Itoa(m.PacketVersion), number(m.BootCount), number(m.DeviceSequence), m.State, number(m.DiagCode), number(m.Flags), decimal(m.RMSG), decimal(m.RMSXG), decimal(m.RMSYG), decimal(m.Kurtosis), decimal(m.CrestFactor), decimal(m.DominantFreqHz), decimal(m.SampleRateHz), decimal(m.TemperatureC), decimal(m.RSSIDBm), decimal(m.SNRDB), number(m.ConfigVersion), number(m.CommandSeq), number(m.ResetReason), number(m.UptimeS), number(m.WaveFragments), number(m.WaveSamples), number(m.Channels), number(m.TxPowerDBm), number(m.PacketCRC16), moment(m.DeviceTime), moment(m.GatewayReceivedAt), moment(m.EstimatedReceivedAt), moment(&ingested), strconv.FormatInt(m.QueueDelayMS, 10)}
}

func writeReadings(ctx context.Context, cl *apiclient.Client, device, from, to, folder string, progress func(ExportProgress)) (int, error) {
	file, writer, err := createCSV(filepath.Join(folder, "leituras.csv"))
	if err != nil {
		return 0, err
	}
	defer file.Close()
	if err = writer.Write(readingColumns); err != nil {
		return 0, errors.New("não foi possível gravar a exportação")
	}
	// The API pages from the newest; the file goes from the oldest.
	rows := [][]string{}
	cursor := ""
	for {
		page, err := cl.History(ctx, device, from, to, cursor, 1000)
		if err != nil {
			return 0, err
		}
		for _, item := range page.Items {
			rows = append(rows, readingRow(item))
		}
		progress(ExportProgress{Stage: "readings", Done: len(rows)})
		if page.NextCursor == "" {
			break
		}
		cursor = page.NextCursor
	}
	slices.Reverse(rows)
	if err = writer.WriteAll(rows); err != nil {
		return 0, errors.New("não foi possível gravar a exportação")
	}
	return len(rows), closeCSV(file, writer)
}

var windowColumns = []string{"id", "arquivo", "device_id", "gateway_id", "gateway_session_id", "event_seq", "boot_count", "device_sequence", "channels", "window_samples", "sample_rate_hz", "accel_range", "gyro_range", "dlpf", "burst_profile", "fragment_total", "fragments_missing_count", "fragments_missing", "gateway_received_at", "estimated_received_at", "ingested_at", "queue_delay_ms"}

func writeWindows(ctx context.Context, cl *apiclient.Client, device, from, to, folder string, progress func(ExportProgress)) (int, error) {
	items := []c.Waveform{}
	cursor := ""
	for {
		page, err := cl.Waveforms(ctx, device, from, to, cursor, 200)
		if err != nil {
			return 0, err
		}
		items = append(items, page.Items...)
		if page.NextCursor == "" {
			break
		}
		cursor = page.NextCursor
	}
	slices.Reverse(items)
	file, writer, err := createCSV(filepath.Join(folder, "janelas.csv"))
	if err != nil {
		return 0, err
	}
	defer file.Close()
	if err = writer.Write(windowColumns); err != nil {
		return 0, errors.New("não foi possível gravar a exportação")
	}
	for i, item := range items {
		full, err := cl.Waveform(ctx, item.ID)
		if err != nil {
			return 0, err
		}
		name := fmt.Sprintf("janela-%d.csv", item.ID)
		if err = writeWindow(full, filepath.Join(folder, name)); err != nil {
			return 0, err
		}
		missing := make([]string, 0, len(full.FragmentsMissing))
		for _, index := range full.FragmentsMissing {
			missing = append(missing, strconv.Itoa(index))
		}
		ingested := full.IngestedAt
		if err = writer.Write([]string{strconv.FormatInt(full.ID, 10), name, full.DeviceID, full.GatewayID, full.GatewaySessionID, strconv.FormatInt(full.EventSeq, 10), strconv.FormatUint(uint64(full.BootCount), 10), strconv.FormatUint(uint64(full.DeviceSequence), 10), strconv.Itoa(full.Channels), strconv.Itoa(full.WindowSamples), strconv.FormatFloat(full.SampleRateHz, 'f', -1, 64), strconv.Itoa(full.AccelRange), strconv.Itoa(full.GyroRange), strconv.Itoa(full.DLPF), strconv.Itoa(full.BurstProfile), strconv.Itoa(full.FragmentTotal), strconv.Itoa(len(full.FragmentsMissing)), strings.Join(missing, " "), moment(full.GatewayReceivedAt), moment(full.EstimatedReceivedAt), moment(&ingested), strconv.FormatInt(full.QueueDelayMS, 10)}); err != nil {
			return 0, errors.New("não foi possível gravar a exportação")
		}
		progress(ExportProgress{Stage: "windows", Done: i + 1, Total: len(items)})
	}
	return len(items), closeCSV(file, writer)
}

// writeWindow writes one line per sample, in raw values and physical units. Samples of a
// lost fragment arrive as zeros: they stay empty here and marked, because they were never
// measured.
func writeWindow(w c.Waveform, path string) error {
	raw, err := base64.StdEncoding.DecodeString(w.SamplesB64)
	if err != nil || len(raw) != c.WindowBytes(w.WindowSamples, w.Channels) {
		return errors.New("janela incompatível com a especificação")
	}
	file, writer, err := createCSV(path)
	if err != nil {
		return err
	}
	defer file.Close()
	active := []int{}
	header := []string{"amostra", "tempo_s", "fragmento", "fragmento_ausente"}
	for bit, channel := range channelNames {
		if w.Channels&(1<<bit) == 0 {
			continue
		}
		active = append(active, bit)
		unit := "g"
		if bit >= 3 {
			unit = "graus_s"
		}
		header = append(header, channel+"_bruto", channel+"_"+unit)
	}
	if err = writer.Write(header); err != nil {
		return errors.New("não foi possível gravar a exportação")
	}
	lost := map[int]bool{}
	for _, index := range w.FragmentsMissing {
		lost[index] = true
	}
	row := make([]string, 4+2*len(active))
	offset := 0
	for sample := 0; sample < w.WindowSamples; sample++ {
		fragment := c.FragmentOfSample(sample, w.Channels)
		row[0] = strconv.Itoa(sample)
		row[1] = strconv.FormatFloat(float64(sample)/w.SampleRateHz, 'f', 6, 64)
		row[2] = strconv.Itoa(fragment)
		row[3] = "0"
		if lost[fragment] {
			row[3] = "1"
		}
		for i, bit := range active {
			value := int16(binary.LittleEndian.Uint16(raw[offset:]))
			offset += 2
			if lost[fragment] {
				row[4+2*i], row[5+2*i] = "", ""
				continue
			}
			row[4+2*i] = strconv.Itoa(int(value))
			if bit < 3 {
				row[5+2*i] = strconv.FormatFloat(c.AccelG(value, w.AccelRange), 'f', 6, 64)
			} else {
				row[5+2*i] = strconv.FormatFloat(c.GyroDPS(value, w.GyroRange), 'f', 4, 64)
			}
		}
		if err = writer.Write(row); err != nil {
			return errors.New("não foi possível gravar a exportação")
		}
	}
	return closeCSV(file, writer)
}
