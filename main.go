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

var version = "dev"

func init() {
	os.Setenv("WEBKIT_DISABLE_DMABUF_RENDERER", "1")
	os.Setenv("GDK_BACKEND", "x11")
}

func main() {
	// Создаем инстанс бэкенда
	appInstance := NewApp()

	// Автоинтеграция AppImage (если запущено из Загрузок — перенесет себя в ~/Applications и перезапустится)
	appInstance.AutoIntegrateAppImage()

	// Инициализируем приложение Wails v3
	app := application.New(application.Options{
		Name:        "RFAD Launcher",
		Description: "Launcher for RFAD SE",
		Icon:        appIcon,
		Assets: application.AssetOptions{
			Handler: application.AssetFileServerFS(assets),
		},
		Services: []application.Service{
			application.NewService(appInstance),
		},
		Mac: application.MacOptions{
			ApplicationShouldTerminateAfterLastWindowClosed: true,
		},
	})

	// Создаем главное окно
	app.Window.NewWithOptions(application.WebviewWindowOptions{
		Name:             "main",
		Title:            "RFAD Launcher",
		Width:            1240,
		Height:           768,
		Frameless:        true,
		BackgroundColour: application.NewRGBA(0, 0, 0, 0),
	})

	// Запускаем приложение
	err := app.Run()
	if err != nil {
		log.Fatal(err)
	}
}
