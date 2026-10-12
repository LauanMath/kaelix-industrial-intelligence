package api

import (
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	c "kaelix/contracts"
)

const fixture = `{"schema_version":1,"gateway_id":"5ef49647-9449-4f47-8d1d-c06611c3ba81","gateway_session_id":"17162ec5-4fe1-4dac-a871-879120324a57","upload_uptime_ms":13000,"readings":[{"event_seq":1,"gateway_rx_uptime_ms":12000,"gateway_received_at":null,"packet_version":2,"device_id":"D6F61B44","device_sequence":1,"state":"UNKNOWN","diag_code":0,"rms_g":null,"temperature_c":null,"rssi_dbm":-55,"snr_db":13.5,"packet_crc16":12345}]}`

func TestReadingContract(t *testing.T) {
	cases := []struct {
		name, json string
		want       bool
	}{
		{"valid nullable reading", fixture, true},
		{"missing rms", strings.Replace(fixture, `"rms_g":null,`, "", 1), false},
		{"missing monotonic clock", strings.Replace(fixture, `"gateway_rx_uptime_ms":12000,`, "", 1), false},
		{"unknown field", strings.Replace(fixture, `"rms_g":null,`, `"rms_g":null,"waveform":[],`, 1), false},
		{"unknown state", strings.Replace(fixture, `"UNKNOWN"`, `"OK"`, 1), false},
		{"negative rms", strings.Replace(fixture, `"rms_g":null`, `"rms_g":-1`, 1), false},
		{"future receive uptime", strings.Replace(fixture, `"gateway_rx_uptime_ms":12000`, `"gateway_rx_uptime_ms":14000`, 1), false},
		{"sequence overflow", strings.Replace(fixture, `"device_sequence":1`, `"device_sequence":4294967296`, 1), false},
		{"null sequence", strings.Replace(fixture, `"device_sequence":1`, `"device_sequence":null`, 1), false},
		{"unsupported packet", strings.Replace(fixture, `"packet_version":2`, `"packet_version":3`, 1), false},
		{"bad device ID", strings.Replace(fixture, "D6F61B44", "not-a-device", 1), false},
		{"null RSSI", strings.Replace(fixture, `"rssi_dbm":-55`, `"rssi_dbm":null`, 1), false},
		{"nonfinite JSON", strings.Replace(fixture, `"rms_g":null`, `"rms_g":NaN`, 1), false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var b c.ReadingBatch
			err := json.Unmarshal([]byte(tc.json), &b)
			if err == nil {
				err = validateBatch(b, time.Now())
			}
			if (err == nil) != tc.want {
				t.Fatalf("valid=%v error=%v", err == nil, err)
			}
		})
	}
}
func TestHistoryQuery(t *testing.T) {
	for _, query := range []string{"limit=0", "limit=1001", "cursor=bad", "from=tomorrow", "from=2026-01-02T00:00:00Z&to=2026-01-01T00:00:00Z", "unknown=1", "limit=1&limit=2"} {
		r := httptest.NewRequest("GET", "/readings?"+query, nil)
		if _, err := historyFilter(r, "D6F61B44"); err == nil {
			t.Fatalf("accepted %s", query)
		}
	}
}
