package contracts

import (
	"fmt"
	"math"
	"math/bits"
	"strconv"
	"strings"
)

// Limits and defaults of the transmitter configuration, from docs/comando-e-coleta.md.
const (
	MaxRoutineWindows   = 6
	MaxConfigVersion    = 65535
	MaxCommandSeq       = 65535
	ChannelAZBit        = 1 << 2
	fragmentPayloadSize = 240
	fragmentOverhead    = 14
)

// Sample and control timings of one measurement cycle, in seconds.
const (
	acquisitionMarginS = 0.3
	telemetryTimeS     = 0.411
	replyTimeS         = 0.711
	fragmentGapS       = 0.05
	burstMapTimeS      = 0.3
	// Tolerance for binary rounding when a calculated time lands on a whole second.
	cycleEpsilonS = 1e-9
)

// RoutineWindow is one active period. Days is a mask with bit 0 for Sunday; EndMin
// below StartMin crosses midnight.
type RoutineWindow struct {
	Days     int `json:"days"`
	StartMin int `json:"start_min"`
	EndMin   int `json:"end_min"`
}

// DeviceConfig is the transmitter configuration. The API assigns Version.
type DeviceConfig struct {
	Version          int             `json:"version"`
	CycleIntervalS   int             `json:"cycle_interval_s"`
	WindowSamples    int             `json:"window_samples"`
	Channels         int             `json:"channels"`
	AccelRange       int             `json:"accel_range"`
	GyroRange        int             `json:"gyro_range"`
	DLPF             int             `json:"dlpf"`
	TxPowerDBm       int             `json:"tx_power_dbm"`
	BurstProfile     int             `json:"burst_profile"`
	WaveEvery        int             `json:"wave_every"`
	LinkLossS        int             `json:"link_loss_s"`
	CheckinIntervalS int             `json:"checkin_interval_s"`
	TZOffsetMin      int             `json:"tz_offset_min"`
	Routine          []RoutineWindow `json:"routine"`
}

// DefaultDeviceConfig returns the defaults of the configuration table. It is also
// the base a request is decoded over, so an absent field takes its default and not zero.
func DefaultDeviceConfig() DeviceConfig {
	return DeviceConfig{CycleIntervalS: 11, WindowSamples: 512, Channels: 63, AccelRange: 1, GyroRange: 0, DLPF: 1, TxPowerDBm: 10, BurstProfile: 2, WaveEvery: 1, LinkLossS: 900, CheckinIntervalS: 300, TZOffsetMin: -180, Routine: []RoutineWindow{}}
}

// BurstProfile is one row of the burst profile table.
type BurstProfile struct {
	SF    int
	BWKHz float64
}

// BurstProfiles is indexed by the burst_profile code.
var BurstProfiles = []BurstProfile{{SF: 9, BWKHz: 125}, {SF: 7, BWKHz: 125}, {SF: 7, BWKHz: 250}, {SF: 7, BWKHz: 500}}

// LoRaAirtimeMS returns the time on air of a frame of payload bytes, in milliseconds,
// with an 8-symbol preamble, explicit header, CRC and coding rate 4/5.
func LoRaAirtimeMS(payload, sf int, bwKHz float64) float64 {
	blocks := math.Ceil(float64(8*payload-4*sf+44) / float64(4*sf))
	symbols := 8 + math.Max(blocks*5, 0)
	return (12.25 + symbols) * float64(int(1)<<sf) / bwKHz
}

// ChannelCount returns how many channels a mask selects.
func ChannelCount(mask int) int { return bits.OnesCount(uint(mask)) }

// WindowBytes is the exact size of a raw sample block.
func WindowBytes(windowSamples, channels int) int {
	return windowSamples * ChannelCount(channels) * 2
}

// FragmentPayload is how many sample bytes one fragment carries: a whole number of
// sample frames, so a lost fragment never splits a frame.
func FragmentPayload(channels int) int {
	frame := 2 * ChannelCount(channels)
	if frame == 0 {
		return 0
	}
	return fragmentPayloadSize - fragmentPayloadSize%frame
}

// FragmentCount is how many fragments a window takes.
func FragmentCount(windowSamples, channels int) int {
	payload := FragmentPayload(channels)
	if payload == 0 {
		return 0
	}
	return (WindowBytes(windowSamples, channels) + payload - 1) / payload
}

// FragmentOfSample returns the fragment that carries a sample index of a window.
func FragmentOfSample(sample, channels int) int {
	payload := FragmentPayload(channels)
	if payload == 0 {
		return 0
	}
	return sample * 2 * ChannelCount(channels) / payload
}

// BurstTransmissionMS sums the time on air of all fragments of a window, without gaps.
func BurstTransmissionMS(windowSamples, channels, profile int) float64 {
	p := BurstProfiles[profile]
	payload := FragmentPayload(channels)
	if payload == 0 {
		return 0
	}
	total := WindowBytes(windowSamples, channels)
	full := total / payload
	out := float64(full) * LoRaAirtimeMS(payload+fragmentOverhead, p.SF, p.BWKHz)
	if rest := total % payload; rest > 0 {
		out += LoRaAirtimeMS(rest+fragmentOverhead, p.SF, p.BWKHz)
	}
	return out
}

// CycleTimeSeconds is the calculated time one cycle takes with this configuration.
// The burst profile and the sizes must already be within range.
func CycleTimeSeconds(cfg DeviceConfig) float64 {
	seconds := float64(cfg.WindowSamples)/1000 + acquisitionMarginS + telemetryTimeS + replyTimeS
	if cfg.WaveEvery > 0 {
		seconds += BurstTransmissionMS(cfg.WindowSamples, cfg.Channels, cfg.BurstProfile)/1000 + float64(FragmentCount(cfg.WindowSamples, cfg.Channels))*fragmentGapS + burstMapTimeS
	}
	return seconds
}

// MinimumCycleInterval is the smallest cycle_interval_s, in whole seconds, that fits the
// calculated time.
func MinimumCycleInterval(cfg DeviceConfig) int {
	return int(math.Ceil(CycleTimeSeconds(cfg) - cycleEpsilonS))
}

// ValidWindowSamples lists the accepted window sizes.
func ValidWindowSamples(n int) bool {
	return n == 256 || n == 512 || n == 1024 || n == 2048 || n == 4096
}

func formatSeconds(v float64) string {
	return strings.ReplaceAll(strconv.FormatFloat(v, 'f', 2, 64), ".", ",")
}

// ValidateDeviceConfig checks every range and the minimum cycle. Reasons are in
// Portuguese because the desktop shows them next to the form field.
func ValidateDeviceConfig(cfg DeviceConfig) []ErrorDetail {
	issues := []ErrorDetail{}
	add := func(path, reason string) { issues = append(issues, ErrorDetail{Path: path, Reason: reason}) }
	between := func(path, label string, value, low, high int) {
		if value < low || value > high {
			add(path, fmt.Sprintf("%s deve estar entre %d e %d", label, low, high))
		}
	}
	between("cycle_interval_s", "O intervalo do ciclo", cfg.CycleIntervalS, 1, 3600)
	if !ValidWindowSamples(cfg.WindowSamples) {
		add("window_samples", "A janela deve ter 256, 512, 1024, 2048 ou 4096 amostras")
	}
	between("channels", "A máscara de canais", cfg.Channels, 1, 63)
	if cfg.Channels >= 1 && cfg.Channels <= 63 && cfg.Channels&ChannelAZBit == 0 {
		add("channels", "A máscara de canais precisa incluir az")
	}
	between("accel_range", "A faixa do acelerômetro", cfg.AccelRange, 0, 3)
	between("gyro_range", "A faixa do giroscópio", cfg.GyroRange, 0, 3)
	between("dlpf", "O código do filtro", cfg.DLPF, 0, 6)
	between("tx_power_dbm", "A potência do rádio", cfg.TxPowerDBm, 2, 17)
	between("burst_profile", "O perfil de rajada", cfg.BurstProfile, 0, len(BurstProfiles)-1)
	between("wave_every", "O sinal bruto a cada N ciclos", cfg.WaveEvery, 0, 255)
	between("link_loss_s", "O silêncio até a resposta automática", cfg.LinkLossS, 60, 86400)
	between("checkin_interval_s", "O intervalo fora da rotina", cfg.CheckinIntervalS, 60, 3600)
	between("tz_offset_min", "O fuso", cfg.TZOffsetMin, -720, 840)
	if len(cfg.Routine) > MaxRoutineWindows {
		add("routine", fmt.Sprintf("A rotina aceita até %d janelas", MaxRoutineWindows))
	}
	for i, w := range cfg.Routine {
		prefix := fmt.Sprintf("routine[%d].", i)
		label := fmt.Sprintf("Na janela %d, ", i+1)
		if w.Days < 1 || w.Days > 127 {
			add(prefix+"days", label+"escolha ao menos um dia")
		}
		between(prefix+"start_min", label+"o início", w.StartMin, 0, 1440)
		between(prefix+"end_min", label+"o fim", w.EndMin, 0, 1440)
		if w.StartMin == w.EndMin {
			add(prefix+"end_min", label+"o início e o fim precisam ser diferentes")
		}
	}
	sized := ValidWindowSamples(cfg.WindowSamples) && cfg.Channels >= 1 && cfg.Channels <= 63 && cfg.BurstProfile >= 0 && cfg.BurstProfile < len(BurstProfiles) && cfg.WaveEvery >= 0
	if sized && cfg.CycleIntervalS >= 1 && cfg.CycleIntervalS <= 3600 {
		if seconds := CycleTimeSeconds(cfg); seconds > float64(cfg.CycleIntervalS)+cycleEpsilonS {
			minimum := MinimumCycleInterval(cfg)
			issues = append(issues, ErrorDetail{Path: "cycle_interval_s", Reason: fmt.Sprintf("A janela não cabe no ciclo: o tempo calculado é de %s s e o intervalo mínimo é de %d s", formatSeconds(seconds), minimum), Minimum: &minimum})
		}
	}
	return issues
}

// DeviceConfigCheck is the result of validating a form without saving it.
type DeviceConfigCheck struct {
	Issues        []ErrorDetail `json:"issues"`
	CycleSeconds  *float64      `json:"cycle_seconds"`
	MinimumCycleS *int          `json:"minimum_cycle_s"`
}

// CheckDeviceConfig validates a form and, when its sizes are valid, calculates the cycle.
func CheckDeviceConfig(cfg DeviceConfig) DeviceConfigCheck {
	out := DeviceConfigCheck{Issues: ValidateDeviceConfig(cfg)}
	if ValidWindowSamples(cfg.WindowSamples) && cfg.Channels >= 1 && cfg.Channels <= 63 && cfg.BurstProfile >= 0 && cfg.BurstProfile < len(BurstProfiles) && cfg.WaveEvery >= 0 {
		seconds := CycleTimeSeconds(cfg)
		minimum := MinimumCycleInterval(cfg)
		out.CycleSeconds, out.MinimumCycleS = &seconds, &minimum
	}
	return out
}

// Physical conversions of raw int16 samples.

// AccelG converts a raw accelerometer sample to g.
func AccelG(raw int16, accelRange int) float64 {
	return float64(raw) / (16384 / float64(int(1)<<accelRange))
}

// GyroDPS converts a raw gyroscope sample to degrees per second.
func GyroDPS(raw int16, gyroRange int) float64 {
	return float64(raw) / (131 / float64(int(1)<<gyroRange))
}
