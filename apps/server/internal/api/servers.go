package api

import (
	"crypto/tls"
	"net/http"
	"time"
)

// UserServer is the plain HTTP listener with all routes, meant for loopback.
func UserServer(address string, handler http.Handler) *http.Server {
	return &http.Server{Addr: address, Handler: handler, ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 15 * time.Second, WriteTimeout: 15 * time.Second, IdleTimeout: 60 * time.Second, MaxHeaderBytes: 16 << 10}
}

// GatewayServer is the TLS listener for the local network. A TLS handshake is expensive
// on the gateway, so the idle limit is several times the longest sync interval (60 s) and
// the same connection serves every poll. HTTP/2 is off: the firmware only speaks HTTP/1.1,
// and its multiplexing would add nothing for one client. net/http takes the handshake limit
// from the shortest of these timeouts, so ReadHeaderTimeout also bounds a slow TLS handshake.
func GatewayServer(address string, handler http.Handler, certificate tls.Certificate) *http.Server {
	return &http.Server{Addr: address, Handler: handler, ReadHeaderTimeout: 20 * time.Second, ReadTimeout: 30 * time.Second, WriteTimeout: 30 * time.Second, IdleTimeout: 3 * time.Minute, MaxHeaderBytes: 16 << 10, TLSConfig: &tls.Config{Certificates: []tls.Certificate{certificate}, MinVersion: tls.VersionTLS12, NextProtos: []string{"http/1.1"}}, TLSNextProto: map[string]func(*http.Server, *tls.Conn, http.Handler){}}
}

// WebServer is the TLS listener of the collection page. A browser holds its own connections,
// so it keeps the limits of the user listener.
func WebServer(address string, handler http.Handler, certificate tls.Certificate) *http.Server {
	return &http.Server{Addr: address, Handler: handler, ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 15 * time.Second, WriteTimeout: 15 * time.Second, IdleTimeout: 60 * time.Second, MaxHeaderBytes: 16 << 10, TLSConfig: &tls.Config{Certificates: []tls.Certificate{certificate}, MinVersion: tls.VersionTLS12}}
}
