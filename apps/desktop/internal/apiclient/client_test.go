package apiclient

import (
	"context"
	"errors"
	"io"
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

func TestValidationReasonsReachTheUser(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(422)
		_, _ = w.Write([]byte(`{"schema_version":1,"error":{"code":"INVALID_REQUEST","message":"failed","retryable":false,"details":[{"path":"cycle_interval_s","reason":"O intervalo mínimo é de 9 s","minimum":9},{"path":"dlpf","reason":"Filtro inválido"}]}}`))
	}))
	defer server.Close()
	cl, _ := New(server.URL, "")
	err := cl.Request(context.Background(), "PUT", "/x", map[string]int{"a": 1}, nil)
	if err == nil || err.Error() != "O intervalo mínimo é de 9 s; Filtro inválido" {
		t.Fatalf("reasons: %v", err)
	}
}
func TestReachableAndUpload(t *testing.T) {
	var contentType string
	var size int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/upload" {
			contentType = r.Header.Get("Content-Type")
			body, _ := io.ReadAll(r.Body)
			size = len(body)
			w.WriteHeader(201)
			_, _ = w.Write([]byte(`{"size":4}`))
			return
		}
		w.WriteHeader(200)
	}))
	if !Reachable(context.Background(), server.URL) {
		t.Fatal("running API not reachable")
	}
	cl, _ := New(server.URL, "token")
	var out struct {
		Size int `json:"size"`
	}
	if err := cl.Upload(context.Background(), "/upload", "application/octet-stream", []byte{1, 2, 3, 4}, &out); err != nil {
		t.Fatal(err)
	}
	if contentType != "application/octet-stream" || size != 4 || out.Size != 4 {
		t.Fatalf("upload: %s %d %+v", contentType, size, out)
	}
	base := server.URL
	server.Close()
	if Reachable(context.Background(), base) {
		t.Fatal("stopped API reachable")
	}
	if Reachable(context.Background(), "not a url") {
		t.Fatal("invalid address reachable")
	}
}
