package main

import (
	"context"
	"errors"
	"kaelix/internal/apiclient"
	"kaelix/internal/store"
	"os"
	"path/filepath"
	"sync"
)

type App struct {
	ctx          context.Context
	store        *store.Store
	storageErr   error
	apiMu        sync.RWMutex
	connectionMu sync.Mutex
	client       *apiclient.Client
	apiCtx       context.Context
	apiCancel    context.CancelFunc
	connection   ConnectionInfo
}

func NewApp() *App { return &App{} }
func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	dir, err := os.UserConfigDir()
	if err != nil {
		a.storageErr = err
		return
	}
	a.store, a.storageErr = store.Open(filepath.Join(dir, "Kaelix", "kaelix.db"))
	if a.store != nil {
		a.restoreAPI()
	}
}
func (a *App) shutdown(ctx context.Context) {
	a.apiMu.Lock()
	if a.apiCancel != nil {
		a.apiCancel()
	}
	a.apiMu.Unlock()
	if a.store != nil {
		a.store.Close()
	}
}
func (a *App) GetAppInfo() map[string]string {
	return map[string]string{"name": "Kaelix", "version": "0.1.0", "mode": "live"}
}
func (a *App) GetProfile() (store.Profile, error) {
	if a.store == nil {
		return store.Profile{}, errors.New("banco local indisponível")
	}
	return a.store.Profile(a.ctx)
}
func (a *App) SaveProfile(p store.Profile) (store.Profile, error) {
	if a.store == nil {
		return store.Profile{}, errors.New("banco local indisponível")
	}
	return a.store.SaveProfile(a.ctx, p)
}
