package main

import (
	"embed"
	"log"
	"os"
	"path/filepath"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/linux"
)

//go:embed all:frontend/dist
var assets embed.FS

//go:embed build/appicon.png
var appIcon []byte

func init() {
	os.Setenv("WEBKIT_DISABLE_DMABUF_RENDERER", "1")
	os.Setenv("GDK_BACKEND", "x11")
}

func main() {
	appDir := os.Getenv("APPDIR")
	if appDir != "" {
		webkitPath := filepath.Join(appDir, "usr", "lib", "x86_64-linux-gnu", "webkit2gtk-4.1")
		os.Setenv("WEBKIT_EXEC_PATH", webkitPath)
		os.Setenv("WEBKIT_INJECTED_BUNDLE_PATH", webkitPath)
	}
	// --------------------------------

	app := NewApp()

	err := wails.Run(&options.App{
		Title:            "RFAD Launcher",
		Width:            1240,
		Height:           768,
		Frameless:        true,
		BackgroundColour: &options.RGBA{R: 0, G: 0, B: 0, A: 0},
		Linux: &linux.Options{
			WindowIsTranslucent: true,
			Icon:                appIcon,
		},
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		OnStartup:  app.startup,
		OnShutdown: app.shutdown,
		Bind: []interface{}{
			app,
		},
	})

	if err != nil {
		log.Fatal(err)
	}
}
