package main

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	c "kaelix/contracts"
	"kaelix/internal/apiclient"
	"kaelix/internal/store"
)

type ConnectionInfo struct {
	Endpoint      string `json:"endpoint"`
	Email         string `json:"email"`
	Authenticated bool   `json:"authenticated"`
	ExpiresAt     string `json:"expiresAt"`
}
type Workspace struct {
	Motors         []c.Motor       `json:"motors"`
	Devices        []c.Device      `json:"devices"`
	Gateways       []c.Gateway     `json:"gateways"`
	RecentReadings []c.Measurement `json:"recentReadings"`
	FetchedAt      string          `json:"fetchedAt"`
}

func (a *App) restoreAPI() {
	saved, err := a.store.APISession(a.ctx)
	if err != nil {
		return
	}
	a.connection = ConnectionInfo{Endpoint: saved.Endpoint, Email: saved.Email, ExpiresAt: saved.ExpiresAt}
	expiry, err := time.Parse(time.RFC3339Nano, saved.ExpiresAt)
	if saved.Token == "" || err != nil || !expiry.After(time.Now()) {
		return
	}
	cl, err := apiclient.New(saved.Endpoint, saved.Token)
	if err != nil {
		return
	}
	a.client = cl
	a.apiCtx, a.apiCancel = context.WithCancel(a.ctx)
	a.connection.Authenticated = true
}
func (a *App) GetConnection() ConnectionInfo {
	a.apiMu.RLock()
	defer a.apiMu.RUnlock()
	return a.connection
}
func (a *App) ConnectAPI(endpoint, email, password string) (ConnectionInfo, error) {
	a.connectionMu.Lock()
	defer a.connectionMu.Unlock()
	if a.store == nil {
		return ConnectionInfo{}, errors.New("banco local indisponível")
	}
	cl, err := apiclient.New(endpoint, "")
	if err != nil {
		return ConnectionInfo{}, err
	}
	email = strings.ToLower(strings.TrimSpace(email))
	login, err := cl.Login(a.ctx, email, password)
	if err != nil {
		return ConnectionInfo{}, err
	}
	cl.Token = login.Token
	expiry := login.ExpiresAt.UTC().Format(time.RFC3339Nano)
	if err = a.store.SaveAPISession(a.ctx, store.APISession{Endpoint: cl.BaseURL, Email: email, Token: cl.Token, ExpiresAt: expiry}); err != nil {
		_ = cl.Request(a.ctx, "POST", "/api/v1/auth/logout", nil, nil)
		return ConnectionInfo{}, errors.New("não foi possível salvar a sessão local")
	}
	a.apiMu.Lock()
	old := a.client
	if a.apiCancel != nil {
		a.apiCancel()
	}
	a.client = cl
	a.apiCtx, a.apiCancel = context.WithCancel(a.ctx)
	a.connection = ConnectionInfo{Endpoint: cl.BaseURL, Email: email, Authenticated: true, ExpiresAt: expiry}
	info := a.connection
	a.apiMu.Unlock()
	// Previous requests are canceled; revocation gets a fresh bounded context.
	if old != nil && old.Token != cl.Token {
		ctx, cancel := context.WithTimeout(a.ctx, 2*time.Second)
		_ = old.Request(ctx, "POST", "/api/v1/auth/logout", nil, nil)
		cancel()
	}
	return info, nil
}
func (a *App) DisconnectAPI() error {
	a.connectionMu.Lock()
	defer a.connectionMu.Unlock()
	a.apiMu.Lock()
	cl := a.client
	if a.apiCancel != nil {
		a.apiCancel()
	}
	a.client = nil
	a.connection.Authenticated = false
	a.connection.ExpiresAt = ""
	info := a.connection
	a.apiMu.Unlock()
	if a.store != nil {
		if err := a.store.SaveAPISession(a.ctx, store.APISession{Endpoint: info.Endpoint, Email: info.Email}); err != nil {
			return errors.New("não foi possível remover a sessão local")
		}
	}
	if cl != nil {
		ctx, cancel := context.WithTimeout(a.ctx, 2*time.Second)
		defer cancel()
		_ = cl.Request(ctx, "POST", "/api/v1/auth/logout", nil, nil)
	}
	return nil
}
func (a *App) remote() (*apiclient.Client, context.Context, error) {
	a.apiMu.RLock()
	defer a.apiMu.RUnlock()
	if a.client == nil {
		return nil, nil, errors.New("conecte-se à API para carregar os dados")
	}
	return a.client, a.apiCtx, nil
}
func (a *App) apiFailure(cl *apiclient.Client, err error) error {
	if errors.Is(err, apiclient.ErrUnauthorized) {
		a.connectionMu.Lock()
		defer a.connectionMu.Unlock()
		a.apiMu.Lock()
		if a.client == cl {
			a.client = nil
			a.connection.Authenticated = false
			a.connection.ExpiresAt = ""
			if a.apiCancel != nil {
				a.apiCancel()
			}
			if a.store != nil {
				_ = a.store.SaveAPISession(a.ctx, store.APISession{Endpoint: a.connection.Endpoint, Email: a.connection.Email})
			}
		}
		a.apiMu.Unlock()
	}
	return err
}
func (a *App) GetWorkspace() (Workspace, error) {
	out := Workspace{Motors: []c.Motor{}, Devices: []c.Device{}, Gateways: []c.Gateway{}, RecentReadings: []c.Measurement{}}
	cl, ctx, err := a.remote()
	if err != nil {
		return out, err
	}
	var motors struct {
		Items []c.Motor `json:"items"`
	}
	var devices struct {
		Items []c.Device `json:"items"`
	}
	var gateways struct {
		Items []c.Gateway `json:"items"`
	}
	var recent struct {
		Items []c.Measurement `json:"items"`
	}
	var wg sync.WaitGroup
	failures := make(chan error, 4)
	for _, request := range []struct {
		path   string
		target any
	}{{"/api/v1/motors", &motors}, {"/api/v1/devices", &devices}, {"/api/v1/gateways", &gateways}, {"/api/v1/readings/recent", &recent}} {
		wg.Add(1)
		go func(path string, target any) {
			defer wg.Done()
			if err := cl.Request(ctx, "GET", path, nil, target); err != nil {
				failures <- err
			}
		}(request.path, request.target)
	}
	wg.Wait()
	close(failures)
	var failure error
	for requestErr := range failures {
		if failure == nil || errors.Is(requestErr, apiclient.ErrUnauthorized) {
			failure = requestErr
		}
	}
	if failure != nil {
		return out, a.apiFailure(cl, failure)
	}
	out.Motors = motors.Items
	out.Devices = devices.Items
	out.Gateways = gateways.Items
	out.RecentReadings = recent.Items
	out.FetchedAt = time.Now().UTC().Format(time.RFC3339Nano)
	return out, nil
}
func (a *App) GetDeviceHistory(device, from, to, cursor string) (c.HistoryPage, error) {
	cl, ctx, err := a.remote()
	if err != nil {
		return c.HistoryPage{}, err
	}
	page, err := cl.History(ctx, device, from, to, cursor, 1000)
	return page, a.apiFailure(cl, err)
}
func (a *App) SaveMotor(input c.MotorInput, update bool) (c.Motor, error) {
	cl, ctx, err := a.remote()
	if err != nil {
		return c.Motor{}, err
	}
	var result c.Motor
	path := "/api/v1/motors"
	method := http.MethodPost
	if update {
		method = http.MethodPut
		path += "/" + url.PathEscape(input.ID)
	}
	err = cl.Request(ctx, method, path, input, &result)
	return result, a.apiFailure(cl, err)
}
