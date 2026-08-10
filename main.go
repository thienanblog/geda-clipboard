package main

import (
	"embed"
	"log"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/mac"
)

//go:embed all:frontend/dist
var assets embed.FS

//go:embed build/trayicon.png
var trayIcon []byte

// appVersion is reported in the About panel. Override at build time with
// -ldflags "-X main.appVersion=1.2.3".
var appVersion = "0.3.0"

func main() {
	app := NewApp(trayIcon)

	err := wails.Run(&options.App{
		Title: "Geda Clipboard",
		// The window spans the list panel plus the transparent preview gutter.
		Width:  app.PopupWidth() + app.PopupGutter(),
		Height: app.PopupHeight(),

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

		Mac: &mac.Options{
			WebviewIsTransparent: true,
			WindowIsTranslucent:  true,
			Appearance:           mac.NSAppearanceNameDarkAqua,
		},

		Bind: []interface{}{app},
	})
	if err != nil {
		log.Println("error:", err)
	}
}
