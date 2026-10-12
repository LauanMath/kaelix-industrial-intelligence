package main

import (
	"embed"
	"github.com/wailsapp/wails/v2/pkg/menu"
	"github.com/wailsapp/wails/v2/pkg/menu/keys"
	"github.com/wailsapp/wails/v2/pkg/options/mac"
	wruntime "github.com/wailsapp/wails/v2/pkg/runtime"
	"runtime"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
)

//go:embed all:frontend/dist
var assets embed.FS

func main() {
	// Create an instance of the app structure
	app := NewApp()

	appMenu := menu.NewMenu()
	if runtime.GOOS == "darwin" {
		appMenu.Append(menu.AppMenu())
	}
	viewMenu := appMenu.AddSubmenu("Área de trabalho")
	for index, label := range []string{"Monitoramento", "Motores", "Gateways", "Eventos", "Configurações", "Dashboard"} {
		workspace := index
		viewMenu.AddText(label, keys.CmdOrCtrl(string(rune('1'+index))), func(_ *menu.CallbackData) {
			if app.ctx != nil {
				wruntime.EventsEmit(app.ctx, "workspace:select", workspace)
			}
		})
	}
	appMenu.Append(menu.EditMenu())
	// Create application with options
	err := wails.Run(&options.App{
		Title:     "Kaelix",
		Menu:      appMenu,
		Mac:       &mac.Options{TitleBar: mac.TitleBarHidden(), Appearance: mac.NSAppearanceNameDarkAqua},
		Width:     1440,
		Height:    900,
		MinWidth:  1000,
		MinHeight: 700,
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		BackgroundColour: &options.RGBA{R: 24, G: 24, B: 24, A: 1},
		OnStartup:        app.startup,
		Bind: []interface{}{
			app,
		},
	})

	if err != nil {
		println("Error:", err.Error())
	}
}
