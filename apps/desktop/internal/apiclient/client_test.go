package apiclient

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestURLPolicy(t *testing.T) {
	for _, value := range []string{"https://api.example.com", "http://127.0.0.1:8080", "http://192.168.18.167:8080/", "http://[::1]:8080"} {
		if _, err := NormalizeURL(value); err != nil {
			t.Fatalf("valid %s: %v", value, err)
		}
	}
	for _, value := range []string{"http://api.example.com", "http://8.8.8.8", "ftp://localhost", "https://user:secret@example.com", "https://example.com/api/v1", "https://example.com?token=secret"} {
		if _, err := NormalizeURL(value); err == nil {
			t.Fatalf("accepted %s", value)
		}
	}
}
func TestUnauthorizedAndRedirect(t *testing.T) {
	reached := false
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { reached = true }))
	defer target.Close()
	source := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer local-session" {
			t.Error("missing credential")
		}
		if r.URL.Path == "/unauthorized" {
			w.WriteHeader(401)
			return
		}
		http.Redirect(w, r, target.URL, 302)
	}))
	defer source.Close()
	cl, _ := New(source.URL, "local-session")
	if err := cl.Request(context.Background(), "GET", "/unauthorized", nil, nil); !errors.Is(err, ErrUnauthorized) {
		t.Fatalf("unauthorized: %v", err)
	}
	if err := cl.Request(context.Background(), "GET", "/redirect", nil, nil); err == nil {
		t.Fatal("redirect accepted")
	}
	if reached {
		t.Fatal("credential forwarded through redirect")
	}
}
func TestCancellation(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { <-r.Context().Done() }))
	defer server.Close()
	cl, _ := New(server.URL, "")
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := cl.Request(ctx, "GET", "/", nil, nil); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancellation: %v", err)
	}
}
