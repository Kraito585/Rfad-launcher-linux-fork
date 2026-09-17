package main

import (
	"embed"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"runtime/debug"
	"strings"
	"syscall"

	"github.com/wailsapp/wails/v3/pkg/application"
)

//go:embed all:frontend/dist
var assets embed.FS

//go:embed build/appicon.png
var appIcon []byte

var version = "2.1.0"

func init() {
	os.Setenv("GDK_BACKEND", "wayland,x11")
	os.Setenv("WEBKIT_DISABLE_DMABUF_RENDERER", "1")
	os.Setenv("WEBKIT_DISABLE_SANDBOX_THIS_IS_DANGEROUS", "1")
}

// Вспомогательная функция для записи логов вне Flatpak песочницы.
func getHostConfigDir() string {
	if _, err := os.Stat("/.flatpak-info"); err == nil {
		out, err := exec.Command("flatpak-spawn", "--host", "sh", "-c", "echo ${XDG_CONFIG_HOME:-$HOME/.config}").Output()
		if err == nil {
			path := strings.TrimSpace(string(out))
			if path != "" {
				return path
			}
		}
	}

	configDir, err := os.UserConfigDir()
	if err != nil {
		return "/tmp"
	}
	return configDir
}

func setupLogging() (*os.File, string) {
	configDir := getHostConfigDir()

	appDir := filepath.Join(configDir, "rfad-launcher")
	os.MkdirAll(appDir, 0755)

	logPath := filepath.Join(appDir, "launcher.log")
	file, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0666)
	if err == nil {
		log.SetOutput(file)
		syscall.Dup2(int(file.Fd()), 1)
		syscall.Dup2(int(file.Fd()), 2)
	}
	return file, logPath
}

func main() {
	logFile, logPath := setupLogging()
	if logFile != nil {
		defer logFile.Close()
	}

	log.Println("=== ЗАПУСК RFAD LAUNCHER ===")

	defer func() {
		if r := recover(); r != nil {
			log.Printf("КРИТИЧЕСКАЯ ОШИБКА (PANIC): %v\n", r)
			log.Printf("Stack Trace:\n%s", debug.Stack())
			exec.Command("xdg-open", logPath).Start()
			os.Exit(1)
		}
	}()

	appInstance := NewApp()

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
		log.Printf("ОШИБКА ЗАПУСКА WAILS: %v\n", err)
		exec.Command("xdg-open", logPath).Start()
		log.Fatal(err)
	}

	log.Println("=== ПРИЛОЖЕНИЕ ЗАКРЫТО ШТАТНО ===")
}
