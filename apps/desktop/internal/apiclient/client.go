package apiclient

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	c "kaelix/contracts"
)

type Client struct {
	BaseURL string
	Token   string
	HTTP    *http.Client
}

// DefaultEndpoint is the API on this computer, reachable on any Wi-Fi network, including
// while the computer is on the gateway's own network.
const DefaultEndpoint = "http://127.0.0.1:8080"

const uploadTimeout = 2 * time.Minute

// Reachable tells whether an API answers at this address, without credentials.
func Reachable(ctx context.Context, endpoint string) bool {
	base, err := NormalizeURL(endpoint)
	if err != nil {
		return false
	}
	ctx, cancel := context.WithTimeout(ctx, 1500*time.Millisecond)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, base+"/healthz", nil)
	if err != nil {
		return false
	}
	client := http.Client{CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	resp, err := client.Do(req)
	if err != nil {
		return false
	}
	resp.Body.Close()
	return resp.StatusCode == http.StatusOK
}

var ErrUnauthorized = errors.New("sessão expirada ou credencial inválida; conecte-se novamente")

func NormalizeURL(value string) (string, error) {
	u, err := url.Parse(strings.TrimSpace(value))
	if err != nil || u.Host == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" || (u.Path != "" && u.Path != "/") {
		return "", errors.New("informe a URL base da API, sem /api/v1, credenciais ou parâmetros")
	}
	if u.Scheme != "https" && u.Scheme != "http" {
		return "", errors.New("a API precisa usar HTTPS ou HTTP de bancada local")
	}
	if u.Scheme == "http" {
		ip := net.ParseIP(u.Hostname())
		if u.Hostname() != "localhost" && (ip == nil || (!ip.IsLoopback() && !ip.IsPrivate())) {
			return "", errors.New("HTTP permitido apenas em localhost ou IP privado da LAN; use HTTPS pela internet")
		}
	}
	return strings.TrimRight(u.String(), "/"), nil
}
func New(endpoint, token string) (*Client, error) {
	base, err := NormalizeURL(endpoint)
	if err != nil {
		return nil, err
	}
	return &Client{BaseURL: base, Token: token, HTTP: &http.Client{Timeout: 8 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}}, nil
}
func (cl *Client) Request(ctx context.Context, method, path string, body, out any) error {
	var reader io.Reader
	if body != nil {
		raw, err := json.Marshal(body)
		if err != nil {
			return err
		}
		reader = bytes.NewReader(raw)
	}
	req, err := http.NewRequestWithContext(ctx, method, cl.BaseURL+path, reader)
	if err != nil {
		return errors.New("requisição inválida")
	}
	req.Header.Set("Accept", "application/json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if cl.Token != "" {
		req.Header.Set("Authorization", "Bearer "+cl.Token)
	}
	return cl.finish(ctx, cl.HTTP, req, out)
}

// Upload sends a raw body. It has its own, longer limit: a firmware image takes longer than
// any JSON request.
func (cl *Client) Upload(ctx context.Context, path, contentType string, body []byte, out any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, cl.BaseURL+path, bytes.NewReader(body))
	if err != nil {
		return errors.New("requisição inválida")
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", contentType)
	if cl.Token != "" {
		req.Header.Set("Authorization", "Bearer "+cl.Token)
	}
	client := *cl.HTTP
	client.Timeout = uploadTimeout
	return cl.finish(ctx, &client, req, out)
}

func (cl *Client) finish(ctx context.Context, client *http.Client, req *http.Request, out any) error {
	resp, err := client.Do(req)
	if err != nil {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		return errors.New("não foi possível alcançar a API; confira o endereço e a rede")
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, (8<<20)+1))
	if err != nil || len(raw) > 8<<20 {
		return errors.New("resposta da API excede o limite ou está incompleta")
	}
	if resp.StatusCode == 401 {
		return ErrUnauthorized
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		var failure c.APIError
		_ = json.Unmarshal(raw, &failure)
		// Field reasons say exactly what to fix, such as the minimum cycle interval.
		if reasons := detailReasons(failure.Error.Details); reasons != "" {
			return errors.New(reasons)
		}
		switch failure.Error.Code {
		case "EVENT_CONFLICT":
			return errors.New("identificador já cadastrado ou dispositivo associado a outro motor")
		case "INVALID_REQUEST":
			return errors.New("dados inválidos; confira os campos e tente novamente")
		case "RATE_LIMITED":
			return errors.New("muitas tentativas; aguarde um minuto")
		}
		return fmt.Errorf("a API respondeu HTTP %d; tente novamente", resp.StatusCode)
	}
	if out == nil || resp.StatusCode == http.StatusNoContent {
		return nil
	}
	if err = json.Unmarshal(raw, out); err != nil {
		return errors.New("resposta da API incompatível com o contrato")
	}
	return nil
}

func detailReasons(details []c.ErrorDetail) string {
	reasons := make([]string, 0, len(details))
	for _, detail := range details {
		if detail.Reason != "" {
			reasons = append(reasons, detail.Reason)
		}
	}
	return strings.Join(reasons, "; ")
}

type LoginResult struct {
	Token     string    `json:"access_token"`
	ExpiresAt time.Time `json:"expires_at"`
}

func (cl *Client) Login(ctx context.Context, email, password string) (LoginResult, error) {
	var result LoginResult
	err := cl.Request(ctx, http.MethodPost, "/api/v1/auth/login", map[string]string{"email": email, "password": password}, &result)
	if err == nil && (len(result.Token) != 43 || !result.ExpiresAt.After(time.Now())) {
		err = errors.New("sessão inválida recebida da API")
	}
	return result, err
}
func (cl *Client) History(ctx context.Context, device, from, to, cursor string, limit int) (c.HistoryPage, error) {
	q := url.Values{}
	if from != "" {
		q.Set("from", from)
	}
	if to != "" {
		q.Set("to", to)
	}
	if cursor != "" {
		q.Set("cursor", cursor)
	}
	q.Set("limit", fmt.Sprint(limit))
	var page c.HistoryPage
	err := cl.Request(ctx, "GET", "/api/v1/devices/"+url.PathEscape(device)+"/readings?"+q.Encode(), nil, &page)
	return page, err
}

// Waveforms lists one page of windows, without samples.
func (cl *Client) Waveforms(ctx context.Context, device, from, to, cursor string, limit int) (c.WaveformPage, error) {
	q := url.Values{}
	if from != "" {
		q.Set("from", from)
	}
	if to != "" {
		q.Set("to", to)
	}
	if cursor != "" {
		q.Set("cursor", cursor)
	}
	q.Set("limit", fmt.Sprint(limit))
	var page c.WaveformPage
	err := cl.Request(ctx, "GET", "/api/v1/devices/"+url.PathEscape(device)+"/waveforms?"+q.Encode(), nil, &page)
	return page, err
}

// Waveform returns a window with its samples.
func (cl *Client) Waveform(ctx context.Context, id int64) (c.Waveform, error) {
	var out c.Waveform
	err := cl.Request(ctx, "GET", fmt.Sprintf("/api/v1/waveforms/%d", id), nil, &out)
	return out, err
}
