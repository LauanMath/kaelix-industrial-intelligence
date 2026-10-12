package main

import (
	"context"
	"errors"
	"kaelix/internal/store"
	"os"
	"path/filepath"
)

type App struct {
	ctx        context.Context
	store      *store.Store
	storageErr error
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
}
func (a *App) shutdown(ctx context.Context) {
	if a.store != nil {
		a.store.Close()
	}
}
func (a *App) GetAppInfo() map[string]string {
	return map[string]string{"name": "Kaelix", "version": "0.1.0", "mode": "demo"}
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
