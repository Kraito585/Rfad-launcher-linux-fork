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
	"syscall"

	"github.com/wailsapp/wails/v3/pkg/application"
)

//go:embed all:frontend/dist
var assets embed.FS

//go:embed build/appicon.png
var appIcon []byte

var version = "dev"

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
	configDir := getHostConfigDir() // Убедитесь, что эта функция использует os.UserConfigDir(), а не Wails API

	appDir := filepath.Join(configDir, "rfad-launcher")
	os.MkdirAll(appDir, 0755)

	logPath := filepath.Join(appDir, fileName)

	// Умная генерация имени для бэкапа (например: "task.log" -> "task-prev.log")
	ext := filepath.Ext(fileName)
	base := strings.TrimSuffix(fileName, ext)
	prevLogPath := filepath.Join(appDir, base+"-prev"+ext)

	// Бэкап старого лога (если существует)
	if _, err := os.Stat(logPath); err == nil {
		os.Rename(logPath, prevLogPath)
	}

	file, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0666)
	if err == nil {
		// 1. Направляем стандартный логгер Go в файл
		log.SetOutput(file)

		// 2. Направляем slog в файл (чтобы slog.Info тоже писало туда)
		slog.SetDefault(slog.New(slog.NewTextHandler(file, nil)))

		// 3. Жесткий перехват stdout (1) и stderr (2) на уровне ОС
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
	// Узнаем, сколько байт сейчас в главном логе
	if info, err := os.Stat(mainLogPath); err == nil {
		startOffset = info.Size()
	}

	// Возвращаем функцию финализации
	return func() {
		srcFile, err := os.Open(mainLogPath)
		if err != nil {
			return
		}
		defer srcFile.Close()

		// Прыгаем на ту позицию, где лог был до запуска нашей задачи
		srcFile.Seek(startOffset, io.SeekStart)

		dstFile, err := os.OpenFile(targetLogPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0666)
		if err != nil {
			return
		}
		defer dstFile.Close()

		// Копируем весь новый текст в целевой файл
		io.Copy(dstFile, srcFile)
	}
}

var cliHelp = `
  -v, --version    Показать версию лаунчера
  --run-game       Тихий запуск игры без UI (не работает)
  -h, --help       Показать это сообщение`

func main() {
	versionFlag := flag.Bool("v", false, "Показать версию")
	versionFlagLong := flag.Bool("version", false, "Показать версию")
	runGameFlag := flag.Bool("run-game", false, "Тихий запуск игры без UI")
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

	if *runGameFlag {
		slog.Info("Инициирован тихий запуск игры через CLI флаг")
		logFile, _ := setupLogging("cli-run.log")
		if logFile != nil {
			defer logFile.Close()
		}

		myApp := &App{}
		err := myApp.StartGame()
		if err != nil {
			slog.Error("Критическая ошибка при тихом запуске", "error", err)
			os.Exit(1)
		}

		slog.Info("Тихий запуск успешно выполнен, завершение процесса лаунчера")
		os.Exit(0)
	}

	logFile, logPath := setupLogging("launcher.log")
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
