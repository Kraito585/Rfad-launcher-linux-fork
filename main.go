package main

import (
	"embed"
	"log"
	"os"

	"github.com/wailsapp/wails/v3/pkg/application"
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
	// Создаем экземпляр вашей структуры (из app.go)
	appInstance := NewApp()

	// 1. Инициализируем само приложение Wails
	app := application.New(application.Options{
		Name:        "RFAD Launcher",
		Description: "Launcher for RFAD",
		Icon:        appIcon,
		Assets: application.AssetOptions{
			Handler: application.AssetFileServerFS(assets),
		},
		Services: []application.Service{
			application.NewService(appInstance),
		},
	})

	app.On(application.EventAppReady, func() {
		appInstance.startup()
	})
	app.On(application.EventAppShutdown, func() {
		appInstance.shutdown()
	})

	window := app.NewWebviewWindowWithOptions(application.WebviewWindowOptions{
		Title:            "RFAD Launcher",
		Width:            1240,
		Height:           768,
		Frameless:        true,
		BackgroundColour: application.NewRGBA(0, 0, 0, 0),
	})

	window.Show()

	err := app.Run()
	if err != nil {
		log.Fatal(err)
	}
}
