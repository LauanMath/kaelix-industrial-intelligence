package main

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"

	wruntime "github.com/wailsapp/wails/v2/pkg/runtime"
	c "kaelix/contracts"
	"kaelix/internal/apiclient"
	"kaelix/internal/gatewaylink"
	"kaelix/internal/store"
)

// call runs one authenticated request and handles an expired session.
func (a *App) call(method, path string, body, out any) error {
	cl, ctx, err := a.remote()
	if err != nil {
		return err
	}
	return a.apiFailure(cl, cl.Request(ctx, method, path, body, out))
}

func devicePath(device, suffix string) string {
	return "/api/v1/devices/" + url.PathEscape(device) + suffix
}
func gatewayPath(gateway, suffix string) string {
	return "/api/v1/gateways/" + url.PathEscape(gateway) + suffix
}

// Transmitters.

func (a *App) GetDeviceConfig(device string) (c.DeviceConfigState, error) {
	var out c.DeviceConfigState
	err := a.call(http.MethodGet, devicePath(device, "/config"), nil, &out)
	return out, err
}
func (a *App) SaveDeviceConfig(device string, cfg c.DeviceConfig) (c.DeviceConfigState, error) {
	var out c.DeviceConfigState
	err := a.call(http.MethodPut, devicePath(device, "/config"), cfg, &out)
	return out, err
}

// CheckDeviceConfig gives the form the same answer the API would, without saving.
func (a *App) CheckDeviceConfig(cfg c.DeviceConfig) c.DeviceConfigCheck {
	return c.CheckDeviceConfig(cfg)
}
func (a *App) GetDeviceCommands(device string) ([]c.DeviceCommand, error) {
	var out c.DeviceCommandList
	if err := a.call(http.MethodGet, devicePath(device, "/commands"), nil, &out); err != nil {
		return []c.DeviceCommand{}, err
	}
	return out.Items, nil
}
func (a *App) SendDeviceCommand(device, kind string) (c.DeviceCommand, error) {
	var out c.DeviceCommand
	err := a.call(http.MethodPost, devicePath(device, "/commands"), c.DeviceCommandInput{Kind: kind}, &out)
	return out, err
}

// Gateways.

func (a *App) GetGatewayConfig(gateway string) (c.GatewayConfigState, error) {
	var out c.GatewayConfigState
	err := a.call(http.MethodGet, gatewayPath(gateway, "/config"), nil, &out)
	return out, err
}
func (a *App) SaveGatewayConfig(gateway string, input c.GatewayConfigInput) (c.GatewayConfigState, error) {
	var out c.GatewayConfigState
	err := a.call(http.MethodPut, gatewayPath(gateway, "/config"), input, &out)
	return out, err
}
func (a *App) GetGatewayFirmware(gateway string) (c.GatewayFirmwareStatus, error) {
	var out c.GatewayFirmwareStatus
	err := a.call(http.MethodGet, gatewayPath(gateway, "/firmware"), nil, &out)
	return out, err
}

const maxFirmwareBytes = 8 << 20

func (a *App) uploadFirmware(gateway, version, path string) (c.GatewayFirmwareStatus, error) {
	info, err := os.Stat(path)
	if err != nil {
		return c.GatewayFirmwareStatus{}, errors.New("não foi possível ler o arquivo de firmware")
	}
	if info.Size() > maxFirmwareBytes {
		return c.GatewayFirmwareStatus{}, errors.New("o firmware excede o limite de 8 MiB")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return c.GatewayFirmwareStatus{}, errors.New("não foi possível ler o arquivo de firmware")
	}
	cl, ctx, err := a.remote()
	if err != nil {
		return c.GatewayFirmwareStatus{}, err
	}
	query := url.Values{"version": {version}}
	if err = a.apiFailure(cl, cl.Upload(ctx, gatewayPath(gateway, "/firmware?"+query.Encode()), "application/octet-stream", data, nil)); err != nil {
		return c.GatewayFirmwareStatus{}, err
	}
	return a.GetGatewayFirmware(gateway)
}

// UploadGatewayFirmware asks for the binary in the native dialog. Closing the dialog sends
// nothing and returns the current state.
func (a *App) UploadGatewayFirmware(gateway, version string) (c.GatewayFirmwareStatus, error) {
	if strings.TrimSpace(version) == "" {
		return c.GatewayFirmwareStatus{}, errors.New("informe a versão do firmware")
	}
	path, err := wruntime.OpenFileDialog(a.ctx, wruntime.OpenDialogOptions{Title: "Selecionar o firmware do gateway", Filters: []wruntime.FileFilter{{DisplayName: "Imagem de firmware (.bin)", Pattern: "*.bin"}}})
	if err != nil {
		return c.GatewayFirmwareStatus{}, err
	}
	if path == "" {
		return a.GetGatewayFirmware(gateway)
	}
	return a.uploadFirmware(gateway, strings.TrimSpace(version), path)
}

// Windows.

func (a *App) GetDeviceWaveforms(device, from, to, cursor string) (c.WaveformPage, error) {
	cl, ctx, err := a.remote()
	if err != nil {
		return c.WaveformPage{Items: []c.Waveform{}}, err
	}
	page, err := cl.Waveforms(ctx, device, from, to, cursor, 100)
	return page, a.apiFailure(cl, err)
}
func (a *App) GetWaveform(id int64) (c.Waveform, error) {
	cl, ctx, err := a.remote()
	if err != nil {
		return c.Waveform{}, err
	}
	out, err := cl.Waveform(ctx, id)
	return out, a.apiFailure(cl, err)
}

// Collections.

func (a *App) GetCollections() ([]c.Collection, error) {
	var out c.CollectionList
	if err := a.call(http.MethodGet, "/api/v1/collections", nil, &out); err != nil {
		return []c.Collection{}, err
	}
	return out.Items, nil
}
func (a *App) StartCollection(input c.CollectionInput) (c.Collection, error) {
	var out c.Collection
	err := a.call(http.MethodPost, "/api/v1/collections", input, &out)
	return out, err
}

// StopCollection ends a collection now, keeping its other fields.
func (a *App) StopCollection(collection c.Collection) (c.Collection, error) {
	var out c.Collection
	end := time.Now().UTC()
	err := a.call(http.MethodPut, fmt.Sprintf("/api/v1/collections/%d", collection.ID), c.CollectionInput{Name: collection.Name, Condition: collection.Condition, Notes: collection.Notes, EndedAt: &end}, &out)
	return out, err
}
func (a *App) GetCollectionSummary(id int64) (c.CollectionSummary, error) {
	var out c.CollectionSummary
	err := a.call(http.MethodGet, fmt.Sprintf("/api/v1/collections/%d/summary", id), nil, &out)
	return out, err
}

// ExportCollection asks for a folder in the native dialog and writes the whole collection
// into a new folder inside it. Closing the dialog exports nothing.
func (a *App) ExportCollection(id int64) (ExportResult, error) {
	cl, ctx, err := a.remote()
	if err != nil {
		return ExportResult{}, err
	}
	parent, err := wruntime.OpenDirectoryDialog(a.ctx, wruntime.OpenDialogOptions{Title: "Escolher a pasta da exportação", CanCreateDirectories: true})
	if err != nil {
		return ExportResult{}, err
	}
	if parent == "" {
		return ExportResult{}, nil
	}
	out, err := exportCollection(ctx, cl, id, parent, func(p ExportProgress) { wruntime.EventsEmit(a.ctx, "export:progress", p) })
	return out, a.apiFailure(cl, err)
}

// Stale address of the API.

// EndpointProbe tells the interface whether to offer the API on this computer.
type EndpointProbe struct {
	Endpoint       string `json:"endpoint"`
	Reachable      bool   `json:"reachable"`
	LocalEndpoint  string `json:"localEndpoint"`
	LocalReachable bool   `json:"localReachable"`
	SuggestLocal   bool   `json:"suggestLocal"`
}

// ProbeConnection checks the saved address and the local one.
func (a *App) ProbeConnection() EndpointProbe {
	endpoint := a.GetConnection().Endpoint
	out := EndpointProbe{Endpoint: endpoint, LocalEndpoint: apiclient.DefaultEndpoint}
	var wg sync.WaitGroup
	wg.Add(2)
	go func() { defer wg.Done(); out.Reachable = apiclient.Reachable(a.ctx, endpoint) }()
	go func() { defer wg.Done(); out.LocalReachable = apiclient.Reachable(a.ctx, apiclient.DefaultEndpoint) }()
	wg.Wait()
	out.SuggestLocal = endpoint != apiclient.DefaultEndpoint && !out.Reachable && out.LocalReachable
	return out
}

// UseLocalEndpoint points the application at the API on this computer. The saved session
// belongs to the previous server and is never sent to another one, so a new login follows.
func (a *App) UseLocalEndpoint() (ConnectionInfo, error) {
	a.connectionMu.Lock()
	defer a.connectionMu.Unlock()
	a.apiMu.Lock()
	if a.apiCancel != nil {
		a.apiCancel()
	}
	a.client = nil
	a.connection = ConnectionInfo{Endpoint: apiclient.DefaultEndpoint, Email: a.connection.Email}
	info := a.connection
	a.apiMu.Unlock()
	if a.store != nil {
		if err := a.store.SaveAPISession(a.ctx, store.APISession{Endpoint: info.Endpoint, Email: info.Email}); err != nil {
			return info, errors.New("não foi possível salvar o endereço local")
		}
	}
	return info, nil
}

// Pairing assistant. The token and the keys stay in this process: the interface only ever
// sees the gateway identifier and the state of each step.

type provisionState struct {
	gatewayID string
	token     string
	existing  bool
}

// ProvisionStatus is what the assistant shows about the pairing in progress.
type ProvisionStatus struct {
	GatewayID string `json:"gatewayId"`
	Existing  bool   `json:"existing"`
}

func (a *App) link() *gatewaylink.Client {
	base := a.linkBaseURL
	if base == "" {
		base = gatewaylink.DefaultBaseURL
	}
	return gatewaylink.New(base)
}

// StartGatewayProvision creates the gateway in the API, or selects an existing one to pair
// again. The credential of an existing gateway is only replaced when the pairing is sent, so
// giving up here leaves that gateway working.
func (a *App) StartGatewayProvision(name, location, existing string) (ProvisionStatus, error) {
	a.provisionMu.Lock()
	defer a.provisionMu.Unlock()
	if existing != "" {
		a.provision = &provisionState{gatewayID: existing, existing: true}
		return ProvisionStatus{GatewayID: existing, Existing: true}, nil
	}
	var created struct {
		Gateway c.Gateway `json:"gateway"`
		Token   string    `json:"gateway_token"`
	}
	if err := a.call(http.MethodPost, "/api/v1/gateways", c.GatewayInput{Name: name, Location: location}, &created); err != nil {
		return ProvisionStatus{}, err
	}
	a.provision = &provisionState{gatewayID: created.Gateway.ID, token: created.Token}
	return ProvisionStatus{GatewayID: created.Gateway.ID}, nil
}

// CancelGatewayProvision forgets the pairing in progress. A gateway already created stays
// in the list and can be paired later.
func (a *App) CancelGatewayProvision() {
	a.provisionMu.Lock()
	defer a.provisionMu.Unlock()
	a.provision = nil
}

// ProvisionInfo reads GET /info on the gateway's own network.
func (a *App) ProvisionInfo() (gatewaylink.Info, error) { return a.link().Info(a.ctx) }

// ProvisionScan reads GET /scan on the gateway's own network.
func (a *App) ProvisionScan() ([]gatewaylink.Network, error) { return a.link().Scan(a.ctx) }

func newAdminKey() (string, error) {
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		return "", err
	}
	return hex.EncodeToString(key), nil
}

// SendGatewayProvision saves the chosen networks in the API and sends the pairing to the
// gateway. Passwords pass through and are not kept here.
func (a *App) SendGatewayProvision(networks []c.WifiCredential) error {
	a.provisionMu.Lock()
	defer a.provisionMu.Unlock()
	if a.provision == nil {
		return errors.New("inicie o assistente novamente")
	}
	if a.store == nil {
		return errors.New("banco local indisponível")
	}
	if len(networks) == 0 {
		return errors.New("escolha ao menos uma rede Wi-Fi")
	}
	gateway := a.provision.gatewayID
	link := a.link()
	info, err := link.Info(a.ctx)
	if err != nil {
		return err
	}
	admin, err := a.store.GatewayAdminKey(a.ctx, gateway)
	if err != nil {
		return errors.New("não foi possível ler o banco local")
	}
	if info.Paired && admin == "" {
		return gatewaylink.ErrAdminKey
	}
	// The API keeps the same networks, so a later edit has their passwords to keep.
	current, err := a.GetGatewayConfig(gateway)
	if err != nil {
		return err
	}
	input := c.DefaultGatewayConfig()
	if current.Config != nil {
		input.HeartbeatIntervalS, input.SyncIntervalS, input.APWhenOffline, input.TxPowerDBm = current.Config.HeartbeatIntervalS, current.Config.SyncIntervalS, current.Config.APWhenOffline, current.Config.TxPowerDBm
	}
	input.Wifi = networks
	wifi := make([]gatewaylink.WifiNetwork, 0, len(networks))
	for _, network := range networks {
		password := ""
		if network.Password != nil {
			password = *network.Password
		}
		wifi = append(wifi, gatewaylink.WifiNetwork{SSID: network.SSID, Password: password})
	}
	if issues := c.ValidateGatewayConfig(input); len(issues) > 0 {
		reasons := make([]string, 0, len(issues))
		for _, issue := range issues {
			reasons = append(reasons, issue.Reason)
		}
		return errors.New(strings.Join(reasons, "; "))
	}
	if _, err = a.SaveGatewayConfig(gateway, input); err != nil {
		return err
	}
	var fingerprint c.TLSFingerprint
	if err = a.call(http.MethodGet, "/api/v1/tls/fingerprint", nil, &fingerprint); err != nil {
		return err
	}
	if a.provision.token == "" {
		var rotated struct {
			Token string `json:"gateway_token"`
		}
		if err = a.call(http.MethodPost, gatewayPath(gateway, "/rotate-token"), nil, &rotated); err != nil {
			return err
		}
		a.provision.token = rotated.Token
	}
	// The header proves the old key; the body installs the key of this pairing. It reaches
	// the local database first, or a failure here would lose the only copy.
	replacement := admin
	if replacement == "" {
		if replacement, err = newAdminKey(); err != nil {
			return err
		}
		if err = a.store.SaveGatewayAdminKey(a.ctx, gateway, replacement); err != nil {
			return errors.New("não foi possível salvar a chave no banco local")
		}
	}
	header := ""
	if info.Paired {
		header = admin
	}
	if err = link.Send(a.ctx, gatewaylink.Provision{GatewayID: gateway, GatewayToken: a.provision.token, AdminKey: replacement, TLSSHA256: fingerprint.SHA256, Wifi: wifi}, header); err != nil {
		return err
	}
	a.provision.token = ""
	a.provision.existing = true
	return nil
}
