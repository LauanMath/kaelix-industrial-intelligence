// Package mdns announces the gateway listener on the local network with the native
// macOS tool, so the gateway finds the Mac without a stored address.
package mdns

import (
	"context"
	"errors"
	"log/slog"
	"os/exec"
	"strconv"
	"time"
)

const (
	// Service is the DNS-SD type gateways browse for.
	Service  = "_kaelix._tcp"
	instance = "Kaelix API"
	retry    = 5 * time.Second
)

func arguments(port int) []string {
	return []string{"-R", instance, Service, "local", strconv.Itoa(port)}
}

// Advertise keeps the service registered until ctx ends, restarting the tool if it exits.
// dns-sd stays in this process group, so launchd also ends it when the job stops. Without
// the tool, the API keeps working and gateways need another way to find it.
func Advertise(ctx context.Context, port int, logger *slog.Logger) {
	path, err := exec.LookPath("dns-sd")
	if err != nil {
		logger.Warn("mDNS not advertised: dns-sd not found", "service", Service, "port", port)
		return
	}
	for ctx.Err() == nil {
		cmd := exec.CommandContext(ctx, path, arguments(port)...)
		cmd.WaitDelay = 2 * time.Second
		logger.Info("advertising by mDNS", "service", Service, "port", port)
		if err = cmd.Run(); err != nil && ctx.Err() == nil && !errors.Is(err, context.Canceled) {
			logger.Warn("mDNS advertisement stopped; retrying", "error", err)
		}
		select {
		case <-ctx.Done():
		case <-time.After(retry):
		}
	}
}
