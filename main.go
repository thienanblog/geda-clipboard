package main

import (
	"embed"
	"log"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/mac"
	"github.com/wailsapp/wails/v2/pkg/options/windows"
)

//go:embed all:frontend/dist
var assets embed.FS

// appVersion is reported in the About panel. Override at build time with
// -ldflags "-X main.appVersion=1.2.3".
var appVersion = "0.12.1"

func main() {
	app := NewApp(trayIcon)

	err := wails.Run(&options.App{
		Title: "Geda Clipboard",
		// The window spans the list panel, the transparent preview gutter and
		// the margins the panel's shadow falls into.
		Width:  app.PopupWidth() + app.PopupGutter() + 2*shadowSide,
		Height: app.PopupHeight() + shadowTop + shadowBottom,

		// The popup is a chromeless panel that floats above other windows and
		// is shown/hidden rather than opened/closed.
		Frameless:         true,
		AlwaysOnTop:       true,
		StartHidden:       true,
		HideWindowOnClose: true,
		DisableResize:     true,

		AssetServer:      &assetserver.Options{Assets: assets},
		BackgroundColour: &options.RGBA{R: 0, G: 0, B: 0, A: 0},

		OnStartup:  app.OnStartup,
		OnShutdown: app.OnShutdown,
		SingleInstanceLock: &options.SingleInstanceLock{
			UniqueId: "com.geda.clipboard",
			OnSecondInstanceLaunch: func(options.SecondInstanceData) {
				app.onSecondInstanceLaunch()
			},
		},

		Mac: &mac.Options{
			WebviewIsTransparent: true,
			WindowIsTranslucent:  true,
			Appearance:           mac.NSAppearanceNameDarkAqua,
		},
		Windows: &windows.Options{
			WebviewIsTransparent:              true,
			WindowIsTranslucent:               true,
			DisableFramelessWindowDecorations: true,
		},

		Bind: []interface{}{app},
	})
	if err != nil {
		log.Println("error:", err)
	}
}
