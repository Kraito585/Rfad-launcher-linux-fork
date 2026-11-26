package main

import (
	"embed"
	"flag"
	"fmt"
	"io"
	"log"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"runtime/debug"
	"strings"
	"sync/atomic"
	"syscall"

	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"
)

//go:embed all:frontend/dist
var assets embed.FS

//go:embed build/appicon.png
var appIcon []byte

var version = "dev"

const LatestMigrationID = 1

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

func setupLogging(fileName string) (*os.File, string) {
	configDir := getHostConfigDir()

	appDir := filepath.Join(configDir, "rfad-launcher")
	os.MkdirAll(appDir, 0755)

	logPath := filepath.Join(appDir, fileName)

	ext := filepath.Ext(fileName)
	base := strings.TrimSuffix(fileName, ext)
	prevLogPath := filepath.Join(appDir, base+"-prev"+ext)

	if _, err := os.Stat(logPath); err == nil {
		os.Rename(logPath, prevLogPath)
	}

	file, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0666)
	if err == nil {
		log.SetOutput(file)
		slog.SetDefault(slog.New(slog.NewTextHandler(file, nil)))
		syscall.Dup2(int(file.Fd()), 1)
		syscall.Dup2(int(file.Fd()), 2)
	}

	return file, logPath
}

func captureLogTail(mainLogName, targetLogName string) func() {
	configDir := getHostConfigDir()
	appDir := filepath.Join(configDir, "rfad-launcher")

	mainLogPath := filepath.Join(appDir, mainLogName)
	targetLogPath := filepath.Join(appDir, targetLogName)

	var startOffset int64 = 0
	if info, err := os.Stat(mainLogPath); err == nil {
		startOffset = info.Size()
	}

	return func() {
		srcFile, err := os.Open(mainLogPath)
		if err != nil {
			return
		}
		defer srcFile.Close()

		srcFile.Seek(startOffset, io.SeekStart)

		dstFile, err := os.OpenFile(targetLogPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0666)
		if err != nil {
			return
		}
		defer dstFile.Close()

		io.Copy(dstFile, srcFile)
	}
}

var cliHelp = `
  -v, --version    Показать версию лаунчера
  --run-game       Тихий запуск игры без окна лаунчера
  --steam          Умный запуск из Steam (миграции, выбор режима / run-type)
  -h, --help       Показать это сообщение`

func resolveStartupMode(a *App, runGame, steam bool) string {
	if runGame {
		return "headless-game"
	}
	if !steam {
		return "main"
	}

	// Если путь не настроен, не завершена первичная установка или нужна миграция — открываем полный лаунчер
	if GetGameRoot() == "" || !a.IsPathExist() || a.GetFirstInstallStatus() || a.GetMigrationStatus() {
		slog.Info("Режим --steam: требуется настройка или миграция, открываем основной лаунчер")
		return "main"
	}

	switch a.GetRunType() {
	case "game":
		return "headless-game"
	case "mo2":
		return "headless-mo2"
	case "launcher":
		return "main"
	default:
		return "picker"
	}
}

func main() {
	versionFlag := flag.Bool("v", false, "Показать версию")
	versionFlagLong := flag.Bool("version", false, "Показать версию")
	runGameFlag := flag.Bool("run-game", false, "Тихий запуск игры без UI")
	steamFlag := flag.Bool("steam", false, "Умный запуск из Steam")
	helpFlag := flag.Bool("h", false, "Показать справку")
	helpFlagLong := flag.Bool("help", false, "Показать справку")

	flag.Usage = func() {
		fmt.Println(cliHelp)
	}

	flag.Parse()

	if *helpFlag || *helpFlagLong {
		fmt.Println(cliHelp)
		os.Exit(0)
	}

	if *versionFlag || *versionFlagLong {
		fmt.Printf("RFAD Launcher v%s\n", version)
		os.Exit(0)
	}

	probeApp := NewApp(true)
	startupMode := resolveStartupMode(probeApp, *runGameFlag, *steamFlag)
	isHeadless := startupMode == "headless-game" || startupMode == "headless-mo2"

	logFileName := "launcher.log"
	if isHeadless {
		logFileName = "cli-run.log"
	}

	logFile, logPath := setupLogging(logFileName)
	closeLog := func() {
		if logFile != nil {
			_ = logFile.Close()
		}
	}
	defer closeLog()

	log.Printf("=== ЗАПУСК RFAD LAUNCHER (mode=%s) ===\n", startupMode)

	defer func() {
		if r := recover(); r != nil {
			log.Printf("КРИТИЧЕСКАЯ ОШИБКА (PANIC): %v\n", r)
			log.Printf("Stack Trace:\n%s", debug.Stack())
			if !isHeadless {
				exec.Command("xdg-open", logPath).Start()
			}
			closeLog()
			os.Exit(1)
		}
	}()

	appInstance := NewApp(isHeadless)

	opts := application.Options{
		Name:        "RFAD Launcher",
		Description: "Launcher for RFAD SE",
		Icon:        appIcon,
		Services: []application.Service{
			application.NewService(appInstance),
		},
		Mac: application.MacOptions{
			ApplicationShouldTerminateAfterLastWindowClosed: !isHeadless,
		},
	}

	// В тихом режиме не поднимаем даже сервер фронтенд-ассетов
	if !isHeadless {
		opts.Assets = application.AssetOptions{
			Handler: application.AssetFileServerFS(assets),
		}
	}

	app := application.New(opts)
	var exitCode atomic.Int32

	switch startupMode {
	case "headless-game", "headless-mo2":
		app.Event.OnApplicationEvent(events.Common.ApplicationStarted, func(_ *application.ApplicationEvent) {
			if GetGameRoot() == "" || !appInstance.IsPathExist() || appInstance.GetFirstInstallStatus() || appInstance.GetMigrationStatus() {
				slog.Error("Игра не готова к тихому запуску — сначала запустите лаунчер с UI")
				exitCode.Store(1)
				app.Quit()
				return
			}

			launchFn := appInstance.StartGame
			if startupMode == "headless-mo2" {
				launchFn = appInstance.OpenMO2
			}
			runSilentAndQuit(app, &exitCode, launchFn)
		})

	case "picker":
		createPickerWindow(app)

	default:
		createMainWindow(app)
	}

	if err := app.Run(); err != nil {
		log.Printf("ОШИБКА ЗАПУСКА WAILS: %v\n", err)
		if !isHeadless {
			exec.Command("xdg-open", logPath).Start()
		}
		closeLog()
		os.Exit(1)
	}

	log.Println("=== ПРИЛОЖЕНИЕ ЗАКРЫТО ШТАТНО ===")
	if code := int(exitCode.Load()); code != 0 {
		closeLog()
		os.Exit(code)
	}
}
