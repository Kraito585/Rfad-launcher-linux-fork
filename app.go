package main

import (
	"context"
	"fmt"
	"log/slog"
	"math/rand/v2"
	"os"
	"os/exec"
	"path/filepath"
	"rfad-launcher-linux/src-wails/core"
	"runtime"
	"strings"
	"time"

	wailsRuntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

type GameSettings struct {
	MangoHud         bool   `json:"mangoHud"`
	Fsr              bool   `json:"fsr"`
	ShaderCache      bool   `json:"shaderCache"`
	Hdr              bool   `json:"hdr"`
	SteamFix         bool   `json:"steamFix"`
	Cdn              bool   `json:"cdn"`
	FpsLimit         string `json:"fpsLimit"`
	WineDllOverrides string `json:"wineDllOverrides"`
	GrafikMod        string `json:"grafikMod"`
	FsrLvl           string `json:"fsrLvl"`
}

type App struct {
	ctx context.Context
}

func NewApp() *App {
	return &App{}
}

func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	slog.Info("App started")
}

func (a *App) shutdown(ctx context.Context) {
	slog.Info("App shutting down")
}

// ============================================================
//  МЕТОДЫ, ВЫЗЫВАЕМЫЕ ИЗ ФРОНТЕНДА (заглушки)
// ============================================================

func (a *App) IsPathExist() bool {
	status, err := core.IsPathExist(a.ctx, GetGameRoot())
	if err != nil {
		panic("uncown error")
	}
	return status
}

func (a *App) GetLocalVersion() string {
	slog.Info("GetLocalVersion called")
	return "0.0.1"
}

func (a *App) GetRemoteVersion() string {
	slog.Info("GetRemoteVersion called")
	return "0.0.2"
}

func (a *App) LoadPatches() string {
	slog.Info("LoadPatches called")
	return `[{"version":"1.0.0","date":"2026-01-01","author":"Test","name":"Test Patch","description":"Test description","url":"https://example.com"}]`
}

func (a *App) GetFramerateLimit() int {
	slog.Info("GetFramerateLimit called")
	return 60
}

func (a *App) GetVoiceLocale() string {
	slog.Info("GetVoiceLocale called")
	return "ru"
}

func (a *App) UpdateGameSettings(framerate int, voice string) error {
	slog.Info("UpdateGameSettings called", "framerate", framerate, "voice", voice)
	return nil
}

func (a *App) Update() error {
	slog.Info("Update called")

	go func() {
		// 1. Статус: загрузка началась
		wailsRuntime.EventsEmit(a.ctx, "update-status", map[string]string{"status": "download-started"})

		// Имитация скачивания
		for i := 0; i <= 100; i += 2 {
			time.Sleep(50 * time.Millisecond) // Задержка для плавной анимации

			// Генерируем случайную скорость от 15.0 до 45.0 МБ/сек
			// rand.Float64() дает значение от 0.0 до 1.0
			randomSpeedMB := 15.0 + (rand.Float64() * 30.0)
			speedBytes := randomSpeedMB * 1024 * 1024

			wailsRuntime.EventsEmit(a.ctx, "download-progress", map[string]interface{}{
				"fileName":         "update.zip", // Обязательное поле для фронтенда
				"percentage":       float64(i),
				"speedBytesPerSec": speedBytes, // Передаем плавающую скорость
			})
		}

		// 2. Статус: загрузка завершена, начинаем распаковку
		wailsRuntime.EventsEmit(a.ctx, "update-status", map[string]string{"status": "download-finished"})
		time.Sleep(300 * time.Millisecond) // Пауза перед сменой статуса

		wailsRuntime.EventsEmit(a.ctx, "update-status", map[string]string{"status": "unpack-started"})

		// Имитация распаковки (здесь скорость обычно не показывают)
		for i := 0; i <= 100; i += 4 {
			time.Sleep(60 * time.Millisecond)

			wailsRuntime.EventsEmit(a.ctx, "unpack-progress", map[string]interface{}{
				"percentage": float64(i),
			})
		}

		// 3. Статус: распаковка завершена
		wailsRuntime.EventsEmit(a.ctx, "update-status", map[string]string{"status": "unpack-finished"})
	}()

	return nil
}

func (a *App) OpenExplorer() error {
	slog.Info("OpenExplorer called")
	cmd := exec.Command("xdg-open", GetGameRoot())

	err := cmd.Start()
	if err != nil {
		slog.Warn("Не удалось открыть директорию: %v", err)
	}
	return nil
}

func (a *App) OpenMO2() error {
	slog.Info("StartMO2 called")

	gameRoot := GetGameRoot()
	if gameRoot == "" {
		err := fmt.Errorf("не удалось определить путь к игре")
		slog.Error("OpenMO2 failed", "error", err)
		return err
	}

	scriptBytes, err := bundledAssets.ReadFile("embed/start.sh")
	if err != nil {
		slog.Error("не удалось прочитать стартовый скрипт", "error", err)
		return err
	}
	scriptContent := string(scriptBytes)

	var mo2Args string

	if err := core.StartMO2(a.ctx, gameRoot, scriptContent, mo2Args); err != nil {
		slog.Error("StartMO2 failed", "error", err)
		return err
	}
	return nil
}

func (a *App) StartGame() error {
	slog.Info("StartGame called")

	gameRoot := GetGameRoot()
	if gameRoot == "" {
		err := fmt.Errorf("не удалось определить путь к игре")
		slog.Error("StartGame failed", "error", err)
		return err
	}

	scriptBytes, err := bundledAssets.ReadFile("embed/start.sh")
	if err != nil {
		slog.Error("не удалось прочитать стартовый скрипт", "error", err)
		return err
	}
	scriptContent := string(scriptBytes)

	mo2Args := "moshortcut://:SKSE"

	if err := core.StartMO2(a.ctx, gameRoot, scriptContent, mo2Args); err != nil {
		slog.Error("StartGame failed", "error", err)
		return err
	}
	return nil
}

func (a *App) GetLauncherVersion() string {
	slog.Info("GetLauncherVersion called")
	return "0.0.1"
}

func (a *App) UpdateLauncher(downloadUrl string) error {
	slog.Info("UpdateLauncher called", "downloadUrl", downloadUrl)
	return nil
}

func (a *App) StartNewLauncher() error {
	slog.Info("StartNewLauncher called")
	return nil
}

func (a *App) SelectFile() (string, error) {
	options := wailsRuntime.OpenDialogOptions{
		Title: "Выберите установщик игры (.exe или .msi)",
		Filters: []wailsRuntime.FileFilter{
			{DisplayName: "Исполняемые файлы", Pattern: "*.exe;*.msi"},
			{DisplayName: "Все файлы", Pattern: "*.*"},
		},
	}
	return wailsRuntime.OpenFileDialog(a.ctx, options)
}

func (a *App) SelectDirectory() (string, error) {
	options := wailsRuntime.OpenDialogOptions{
		Title: "Выберите папку для установки игры",
	}
	return wailsRuntime.OpenDirectoryDialog(a.ctx, options)
}

func (a *App) ReadFile(path string) (string, error) {
	slog.Info("ReadFile called", "path", path)
	return "File content (stub)", nil
}

func (a *App) WriteFile(path string, content string) error {
	slog.Info("WriteFile called", "path", path, "content_len", len(content))
	return nil
}

func (a *App) GetHomeDir() (string, error) {
	slog.Info("GetHomeDir called")
	return os.UserHomeDir()
}

func (a *App) GetAppVersion() string {
	slog.Info("GetAppVersion called")
	return "0.0.1"
}

func (a *App) RunCommand(command string, args []string) (string, error) {
	slog.Info("RunCommand called", "command", command, "args", args)
	if runtime.GOOS == "windows" {
		return "Command output (stub for Windows)", nil
	}
	return "Command output (stub for Unix)", nil
}

func (a *App) BrowserOpenURL(url string) error {
	slog.Info("OpenBrowser called", "url", url)
	err := exec.Command("xdg-open", url).Start()
	if err != nil {
		// Обработка ошибки, если браузер не удалось открыть
		panic(err)
	}
	return nil
}

func (a *App) ShowMessageDialog(title, message string) error {
	slog.Info("ShowMessageDialog called", "title", title, "message", message)
	return nil
}

func (a *App) GetFirstInstallStatus() bool {
	configFile := filepath.Join(GetGameRoot(), "launcher_config.txt")
	_, err := os.Open(configFile)
	if err != nil {
		if os.IsNotExist(err) {
			return true
		}
		return false
	}

	return false
}

func (a *App) InstallGame(installerPath, installPath string) error {
	cacheDir := filepath.Join(GetGameRoot(), "tmp")
	err := core.InstallGame(a.ctx, installerPath, installPath, cacheDir, getInnoextract(), func(p float64, msg string) {
		wailsRuntime.EventsEmit(a.ctx, "install-progress", map[string]interface{}{
			"percentage": p,
			"message":    msg,
		})
	})
	if err != nil {
		slog.Error("InstallGame failed", "error", err)
		_ = a.ShowMessageDialog("Ошибка", fmt.Sprintf("Не удалось установить игру: %v", err))
		return err
	}

	if err := a.relaunchToGameRoot(installPath); err != nil {
		slog.Warn("Relaunch failed, but installation is complete", "error", err)
		// Не возвращаем ошибку, чтобы не сломать восприятие установки
	}

	return nil
}

func (a *App) FirstInstall() error {
	slog.Info("FirstInstall called")

	err := core.FirstInstall(a.ctx, "gdrive", GetGameRoot(), getCreds(), func(p float64, speed float64, msg string) {

		msgLower := strings.ToLower(msg)
		isUnpacking := strings.Contains(msgLower, "распаков") ||
			strings.Contains(msgLower, "unpack") ||
			strings.Contains(msgLower, "extract") ||
			strings.Contains(msgLower, "патч")

		if isUnpacking {
			wailsRuntime.EventsEmit(a.ctx, "update-status", map[string]string{"status": "download-finished"})
			wailsRuntime.EventsEmit(a.ctx, "update-status", map[string]string{"status": "unpack-started"})

			wailsRuntime.EventsEmit(a.ctx, "unpack-progress", map[string]interface{}{
				"percentage": p * 100,
			})
		} else {
			// Передаем пришедшую скорость
			wailsRuntime.EventsEmit(a.ctx, "download-progress", map[string]interface{}{
				"fileName":         msg,
				"percentage":       p * 100,
				"speedBytesPerSec": speed,
			})
		}

		// Для обратной совместимости
		wailsRuntime.EventsEmit(a.ctx, "install-progress", map[string]interface{}{
			"percentage": p,
			"message":    msg,
		})
	})

	if err != nil {
		slog.Error("FirstInstall failed", "error", err)
		return err
	}

	wailsRuntime.EventsEmit(a.ctx, "update-status", map[string]string{"status": "unpack-finished"})
	return nil
}
func GetGameRoot() string {
	if testRoot := os.Getenv("RFAD_TEST_GAME_ROOT"); testRoot != "" {
		slog.Info(testRoot)
		return testRoot
	}
	exe, err := os.Executable()
	if err != nil {
		return ""
	}
	slog.Info(exe)
	return filepath.Dir(exe)
}

func (a *App) relaunchToGameRoot(installPath string) error {
	// Получаем путь к текущему бинарнику
	exe, err := os.Executable()
	if err != nil {
		return err
	}

	// Если уже в корне игры — выходим
	if filepath.Dir(exe) == installPath {
		slog.Info("Launcher already in game root, skipping copy")
		return nil
	}

	// Копируем бинарник в корень игры
	targetExe := filepath.Join(installPath, filepath.Base(exe))
	data, err := os.ReadFile(exe)
	if err != nil {
		return err
	}
	if err := os.WriteFile(targetExe, data, 0755); err != nil {
		return err
	}

	// Запускаем новую копию
	cmd := exec.Command(targetExe)
	cmd.Env = os.Environ()
	if err := cmd.Start(); err != nil {
		return err
	}

	// Завершаем текущий процесс
	slog.Info("Relaunching from", "path", targetExe)
	go func() {
		time.Sleep(500 * time.Millisecond)
		os.Exit(0)
	}()
	return nil
}

func (a *App) GetGameSettings() core.LauncherConfig {
	gameRoot := GetGameRoot()

	cfg, err := core.GetLauncherConfig(gameRoot)
	if err != nil {
		slog.Warn("Не удалось прочитать настройки для UI, используем дефолтные", "err", err)
		if cfg == nil {
			cfg = &core.LauncherConfig{
				MangoHud:         false,
				FSR:              false,
				ShaderCache:      false,
				HDR:              false,
				SteamFix:         false,
				CDN:              false,
				FpsLimit:         "60",
				WineDllOverrides: "concrt140=n;xaudio2_7=n,b;d3d11=n,b;dxgi=n,b;d3dx9_42=n,b;d3dcompiler_47=n,b;dinput8=n,b;mscoree=n",
				GrafikMod:        "Нету",
				FsrLvl:           "95",
			}
		}
	}

	return *cfg
}

// UpdateSetting сохраняет измененную настройку
// Используем interface{} для value, так как с фронтенда могут приходить как bool, так и string
func (a *App) UpdateSetting(key string, value interface{}) error {
	err := core.UpdateSetting(a.ctx, GetGameRoot(), key, value, nil)
	if err != nil {
		slog.Error("fail to switch settings: %w")
		return err
	}
	return nil
}
