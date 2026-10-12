package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"net/mail"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"kaelix/server/internal/api"
	"kaelix/server/internal/store"
)

func main() {
	if err := run(); err != nil {
		slog.Error("API stopped", "error", err)
		os.Exit(1)
	}
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
	address := os.Getenv("API_ADDR")
	if address == "" {
		address = "127.0.0.1:8080"
	}
	startup, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	db, err := store.Open(startup, url)
	if err != nil {
		return err
	}
	defer db.Close()
	if email != "" {
		if err = db.BootstrapAdmin(startup, email, password); err != nil {
			return errors.New("could not bootstrap admin")
		}
	}
	server := &http.Server{Addr: address, Handler: api.New(db, slog.Default()), ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 15 * time.Second, WriteTimeout: 15 * time.Second, IdleTimeout: 60 * time.Second, MaxHeaderBytes: 16 << 10}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	done := make(chan error, 1)
	go func() { slog.Info("API listening", "address", address); done <- server.ListenAndServe() }()
	select {
	case err = <-done:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	case <-ctx.Done():
		shutdown, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err = server.Shutdown(shutdown); err != nil {
			_ = server.Close()
			return err
		}
		return nil
	}
}
