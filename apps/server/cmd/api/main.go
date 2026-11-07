package main

import (
	"context"
	"errors"
	"log/slog"
	"net"
	"net/http"
	"net/mail"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"kaelix/server/internal/api"
	"kaelix/server/internal/mdns"
	"kaelix/server/internal/secure"
	"kaelix/server/internal/store"
)

func main() {
	if err := run(); err != nil {
		slog.Error("API stopped", "error", err)
		os.Exit(1)
	}
}

func envOr(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

func dataDirectory() (string, error) {
	if dir := os.Getenv("API_DATA_DIR"); dir != "" {
		return dir, nil
	}
	base, err := os.UserConfigDir()
	if err != nil {
		return "", errors.New("set API_DATA_DIR: no user configuration directory")
	}
	return filepath.Join(base, "Kaelix", "api"), nil
}

func run() error {
	url := os.Getenv("DATABASE_URL")
	if url == "" {
		return errors.New("DATABASE_URL is required")
	}
	email := strings.ToLower(strings.TrimSpace(os.Getenv("API_ADMIN_EMAIL")))
	password := os.Getenv("API_ADMIN_PASSWORD")
	if (email == "") != (password == "") {
		return errors.New("provide both API_ADMIN_EMAIL and API_ADMIN_PASSWORD")
	}
	if email != "" {
		address, err := mail.ParseAddress(email)
		if err != nil || address.Address != email || len(password) < 12 || len(password) > 72 {
			return errors.New("bootstrap requires valid admin email and password of 12 to 72 bytes")
		}
	}
	address := envOr("API_ADDR", "127.0.0.1:8080")
	gatewayAddress := envOr("API_GATEWAY_ADDR", "0.0.0.0:8443")
	webAddress := os.Getenv("API_WEB_ADDR")
	dataDir, err := dataDirectory()
	if err != nil {
		return err
	}
	certificate, fingerprint, err := secure.LoadOrCreateCertificate(dataDir)
	if err != nil {
		return err
	}
	box, err := secure.LoadOrCreateBox(dataDir)
	if err != nil {
		return err
	}
	startup, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	db, err := store.Open(startup, url)
	if err != nil {
		return err
	}
	defer db.Close()
	db.UseSealer(box)
	if email != "" {
		if err = db.BootstrapAdmin(startup, email, password); err != nil {
			return errors.New("could not bootstrap admin")
		}
	}
	logger := slog.Default()
	server := api.UserServer(address, api.NewWithOptions(db, logger, api.Options{TLSFingerprint: fingerprint}))
	gatewayServer := api.GatewayServer(gatewayAddress, api.NewGateway(db, logger), certificate)
	instances := []*http.Server{gatewayServer, server}
	// Both sockets open before anything is served or announced, so a taken port fails the
	// start instead of leaving half of the API running.
	listener, err := net.Listen("tcp", address)
	if err != nil {
		return err
	}
	gatewayListener, err := net.Listen("tcp", gatewayAddress)
	if err != nil {
		listener.Close()
		return err
	}
	var webServer *http.Server
	var webListener net.Listener
	if webAddress != "" {
		webCertificate, err := secure.LoadOrCreateWebCertificate(dataDir)
		if err != nil {
			listener.Close()
			gatewayListener.Close()
			return err
		}
		if webListener, err = net.Listen("tcp", webAddress); err != nil {
			listener.Close()
			gatewayListener.Close()
			return err
		}
		webServer = api.WebServer(webAddress, api.NewWeb(db, logger), webCertificate)
		instances = append(instances, webServer)
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	done := make(chan error, 3)
	go func() {
		slog.Info("API listening", "address", address)
		done <- server.Serve(listener)
	}()
	go func() {
		slog.Info("gateway API listening", "address", gatewayAddress, "tls_sha256", fingerprint)
		done <- gatewayServer.ServeTLS(gatewayListener, "", "")
	}()
	if webServer != nil {
		go func() {
			slog.Info("collection page listening", "address", webAddress)
			done <- webServer.ServeTLS(webListener, "", "")
		}()
	}
	advertising, stopAdvertising := context.WithCancel(ctx)
	defer stopAdvertising()
	advertised := make(chan struct{})
	go func() {
		defer close(advertised)
		_, port, err := net.SplitHostPort(gatewayListener.Addr().String())
		if err != nil {
			return
		}
		if number, err := strconv.Atoi(port); err == nil {
			mdns.Advertise(advertising, number, logger)
		}
	}()
	var failure error
	select {
	case failure = <-done:
		if errors.Is(failure, http.ErrServerClosed) {
			failure = nil
		}
	case <-ctx.Done():
	}
	stopAdvertising()
	<-advertised
	shutdown, cancelShutdown := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancelShutdown()
	for _, instance := range instances {
		if err = instance.Shutdown(shutdown); err != nil {
			_ = instance.Close()
			if failure == nil {
				failure = err
			}
		}
	}
	return failure
}
