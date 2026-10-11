package main

import "context"

// App owns the desktop lifecycle and the context for future API requests.
type App struct{ ctx context.Context }

func NewApp() *App                         { return &App{} }
func (a *App) startup(ctx context.Context) { a.ctx = ctx }
func (a *App) GetAppInfo() map[string]string {
	return map[string]string{"name": "Kaelix", "version": "0.1.0", "mode": "demo"}
}
