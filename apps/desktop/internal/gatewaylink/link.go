// Package gatewaylink talks to a gateway on its own Wi-Fi network, during pairing.
package gatewaylink

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"
)

// DefaultBaseURL is the fixed address of a gateway on its own network.
const DefaultBaseURL = "http://192.168.4.1"

// Timeouts stay short so the assistant answers quickly while the computer is not yet on
// the gateway's network. A Wi-Fi scan blocks the gateway for a few seconds.
const (
	infoTimeout      = 3 * time.Second
	scanTimeout      = 10 * time.Second
	provisionTimeout = 6 * time.Second
)

// ErrUnreachable means no gateway answered at the address.
var ErrUnreachable = errors.New("o gateway não respondeu em 192.168.4.1; confira se este computador está na rede Kaelix-XXXX")

// ErrAdminKey means the gateway is paired and refused the key.
var ErrAdminKey = errors.New("o gateway já está pareado e recusou a chave deste computador; segure o botão BOOT por 10 s para apagar o pareamento e tente novamente")

type Info struct {
	MAC             string `json:"mac"`
	FirmwareVersion string `json:"firmware_version"`
	GatewayID       string `json:"gateway_id"`
	Paired          bool   `json:"paired"`
	Connected       bool   `json:"connected"`
	SSID            string `json:"ssid"`
	IP              string `json:"ip"`
	ConfigVersion   int    `json:"config_version"`
}
type Network struct {
	SSID   string `json:"ssid"`
	RSSI   int    `json:"rssi"`
	Secure bool   `json:"secure"`
}
type WifiNetwork struct {
	SSID     string `json:"ssid"`
	Password string `json:"password"`
}
type Provision struct {
	GatewayID    string        `json:"gateway_id"`
	GatewayToken string        `json:"gateway_token"`
	AdminKey     string        `json:"admin_key"`
	TLSSHA256    string        `json:"tls_sha256"`
	Wifi         []WifiNetwork `json:"wifi"`
}

// Client has its own transport: no proxy, and no connection kept across a Wi-Fi change.
type Client struct {
	BaseURL string
	http    *http.Client
}

func New(base string) *Client {
	return &Client{BaseURL: base, http: &http.Client{Transport: &http.Transport{DisableKeepAlives: true}, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}}
}

func (cl *Client) do(ctx context.Context, timeout time.Duration, method, path string, admin string, body, out any) error {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	var reader io.Reader
	if body != nil {
		raw, err := json.Marshal(body)
		if err != nil {
			return errors.New("dados de pareamento inválidos")
		}
		reader = bytes.NewReader(raw)
	}
	req, err := http.NewRequestWithContext(ctx, method, cl.BaseURL+path, reader)
	if err != nil {
		return ErrUnreachable
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if admin != "" {
		req.Header.Set("X-Kaelix-Admin", admin)
	}
	resp, err := cl.http.Do(req)
	if err != nil {
		return ErrUnreachable
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return ErrUnreachable
	}
	if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
		return ErrAdminKey
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("o gateway respondeu HTTP %d", resp.StatusCode)
	}
	if out == nil {
		return nil
	}
	if err = json.Unmarshal(raw, out); err != nil {
		return errors.New("resposta do gateway incompatível com a especificação")
	}
	return nil
}

func (cl *Client) Info(ctx context.Context) (Info, error) {
	var out Info
	err := cl.do(ctx, infoTimeout, http.MethodGet, "/info", "", nil, &out)
	return out, err
}

func (cl *Client) Scan(ctx context.Context) ([]Network, error) {
	var out struct {
		Networks []Network `json:"networks"`
	}
	if err := cl.do(ctx, scanTimeout, http.MethodGet, "/scan", "", nil, &out); err != nil {
		return nil, err
	}
	if out.Networks == nil {
		out.Networks = []Network{}
	}
	return out.Networks, nil
}

// Send delivers the pairing. admin goes in the header when the gateway is already paired.
func (cl *Client) Send(ctx context.Context, request Provision, admin string) error {
	return cl.do(ctx, provisionTimeout, http.MethodPost, "/provision", admin, request, nil)
}
