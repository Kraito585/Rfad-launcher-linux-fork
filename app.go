package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"rfad-launcher-linux/src-wails/core"
	"rfad-launcher-linux/src-wails/downloader"
	config_patcher "rfad-launcher-linux/src-wails/patches/patch_configs"
	"rfad-launcher-linux/src-wails/patches/prefix_install"
	"rfad-launcher-linux/src-wails/patches/proton_install"
	"rfad-launcher-linux/src-wails/patches/rfad_update"
	unpacksteamfix "rfad-launcher-linux/src-wails/patches/unpack_steam_fix"
	"rfad-launcher-linux/src-wails/utils"
	fsrswitch "rfad-launcher-linux/src-wails/utils/fsr_switch"
	graficswitch "rfad-launcher-linux/src-wails/utils/grafic_switch"
	"rfad-launcher-linux/src-wails/utils/steam_drm_switch"
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

	gameRoot := GetGameRoot()
	if gameRoot == "" {
		slog.Warn("GetLocalVersion: путь к игре не найден")
		return "0.0"
	}

	versionPath := filepath.Join(gameRoot, "MO2", "mods", "RFAD_PATCH", "version.txt")

	data, err := os.ReadFile(versionPath)
	if err != nil {
		slog.Warn("Не удалось прочитать файл версии", "path", versionPath, "err", err)
		return "0.0"
	}

	// ВАЖНО: Удаляем невидимый символ BOM, если он есть (часто оставляет блокнот Windows)
	data = bytes.TrimPrefix(data, []byte("\xef\xbb\xbf"))

	// Теперь очищаем строку от переносов (\n, \r) и пробелов
	version := strings.TrimSpace(string(data))

	if version == "" {
		return "0.0"
	}

	return version
}

func (a *App) GetRemoteVersion() string {
	slog.Info("GetRemoteVersion called")

	cfg, err := core.GetLauncherConfig(GetGameRoot())
	if err != nil {
		slog.Warn("Не удалось прочитать конфиг, используем загрузку по умолчанию (GDrive)", "err", err)
		if cfg == nil {
			cfg = &core.LauncherConfig{CDN: false}
		}
	}

	client := &http.Client{
		Timeout: 10 * time.Second,
	}

	downloadType := "gdrive"
	if cfg.CDN {
		downloadType = "cdn"
	}

	if downloadType == "cdn" {

		resp, err := client.Get("https://api.kraito.ru/api/v1/updates/latest")
		if err != nil {
			slog.Warn("Не удалось подключиться к серверу API", "err", err)
			return "NetError"
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			slog.Warn("Сервер API вернул статус-код", "code", resp.StatusCode)
			return "NetError"
		}

		var apiResult struct {
			Success bool `json:"success"`
			Data    struct {
				Version string `json:"version"`
			} `json:"data"`
		}

		if err := json.NewDecoder(resp.Body).Decode(&apiResult); err != nil {
			slog.Warn("Ошибка парсинга ответа от сервера API", "err", err)
			return "NetError"
		}

		if !apiResult.Success || apiResult.Data.Version == "" {
			slog.Warn("Сервер API вернул success: false или пустую версию")
			return "0.0"
		}
		slog.Info("Получена актуальная версия с сервера", "version", apiResult.Data.Version)
		return apiResult.Data.Version

	}

	docID := "17qsV5xDeJZyGZNFbxm3eZ50DYYm1URAvvhx588fAiSo"
	exportURL := fmt.Sprintf("https://docs.google.com/document/d/%s/export?format=txt", docID)

	resp, err := client.Get(exportURL)
	if err != nil {
		slog.Warn("Не удалось подключиться к Google Docs", "err", err)
		return "DriveError"
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		slog.Warn("Google Docs вернул статус-код", "code", resp.StatusCode)
		return "DriveError"
	}

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		slog.Warn("Ошибка чтения ответа от Google Docs", "err", err)
		return "DriveError"
	}

	version := strings.TrimSpace(string(bodyBytes))

	if version == "" {
		slog.Warn("Google Docs вернул пустую строку")
		return "0.0"
	}

	slog.Info("Получена актуальная версия из Google Docs", "version", version)
	return version
}

func (a *App) LoadPatches() string {
	slog.Info("LoadPatches called")

	// Структура для формирования JSON, которую ждёт фронтенд (PatchComponentProps)
	type PatchInfo struct {
		Version     string `json:"version"`
		Date        string `json:"date"`
		Author      string `json:"author"`
		Name        string `json:"name"`
		Description string `json:"description"`
		URL         string `json:"url"`
	}

	// Ссылка на экспорт Google Docs в формате обычного текста (без HTML)
	docURL := "https://docs.google.com/document/export?format=txt&id=1W2fnMXCORWJJu157EwMQP1xAC-qN0_Ke0LS4Hvo58FQ"

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Get(docURL)
	if err != nil {
		slog.Warn("Не удалось загрузить патчноуты", "err", err)
		return "[]"
	}
	defer resp.Body.Close()

	var patches []PatchInfo
	var currentPatch *PatchInfo
	var descBuilder strings.Builder

	skipCurrent := true // Пропускаем весь начальный текст до первого тега [...]
	scanner := bufio.NewScanner(resp.Body)

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		if strings.HasPrefix(line, "[") {
			endIdx := strings.Index(line, "]")
			if endIdx > 3 && endIdx < 50 {
				if currentPatch != nil && !skipCurrent {
					currentPatch.Description = strings.TrimSpace(descBuilder.String())
					patches = append(patches, *currentPatch)
				}

				descBuilder.Reset()

				lineLower := strings.ToLower(line)
				if strings.Contains(lineLower, "в разработке") || strings.Contains(lineLower, "beta") {
					skipCurrent = true
					currentPatch = nil
					continue
				}

				versionStr := line[1:endIdx]

				nameStr := strings.TrimSpace(line[endIdx+1:])
				if nameStr == "" {
					nameStr = "Обновление " + versionStr
				}

				skipCurrent = false
				currentPatch = &PatchInfo{
					Version: versionStr,
					Date:    versionStr,
					Author:  "RFAD Team",
					Name:    nameStr,
					URL:     "https://docs.google.com/document/d/1W2fnMXCORWJJu157EwMQP1xAC-qN0_Ke0LS4Hvo58FQ/edit?tab=t.0",
				}
			}
		}

		if !skipCurrent && currentPatch != nil {
			descBuilder.WriteString(line + "\n")
		}
	}

	if currentPatch != nil && !skipCurrent {
		currentPatch.Description = strings.TrimSpace(descBuilder.String())
		patches = append(patches, *currentPatch)
	}

	if len(patches) == 0 {
		return "[]"
	}

	resultJSON, err := json.Marshal(patches)
	if err != nil {
		slog.Warn("Ошибка сериализации патчноутов", "err", err)
		return "[]"
	}

	return string(resultJSON)
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
	slog.Info("Начало процесса обновления игры")

	gameRoot := GetGameRoot()
	creds := getCreds()

	// Читаем настройки, чтобы узнать, включен ли CDN
	cfg := a.GetGameSettings()

	// ==========================================
	// 1. ЭТАП ЗАГРУЗКИ ОБНОВЛЕНИЯ
	// ==========================================
	wailsRuntime.EventsEmit(a.ctx, "update-status", map[string]string{"status": "download-started"})

	downloadCb := func(p float64, speed float64, msg string) {
		speedMB := speed / 1024 / 1024
		wailsRuntime.EventsEmit(a.ctx, "download-progress", map[string]interface{}{
			"fileName":         msg,
			"percentage":       p * 100,
			"speedBytesPerSec": fmt.Sprintf("%.1f", speedMB),
		})
	}

	// Передаем cfg.CDN в функцию
	if err := rfad_update.DownloadUpdate(a.ctx, gameRoot, creds, cfg.CDN, downloadCb); err != nil {
		slog.Error("Ошибка при скачивании обновления", "error", err)
		return err
	}

	wailsRuntime.EventsEmit(a.ctx, "update-status", map[string]string{"status": "download-finished"})

	wailsRuntime.EventsEmit(a.ctx, "update-status", map[string]string{"status": "unpack-started"})

	unpackCb := func(p float64, msg string) {
		wailsRuntime.EventsEmit(a.ctx, "unpack-progress", map[string]interface{}{
			"percentage": p * 100,
		})
	}

	if err := rfad_update.InstallUpdate(a.ctx, gameRoot, unpackCb); err != nil {
		slog.Error("Ошибка при распаковке обновления", "error", err)
		return err
	}

	wailsRuntime.EventsEmit(a.ctx, "update-status", map[string]string{"status": "unpack-finished"})
	slog.Info("Обновление успешно завершено!")

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

	if err := core.StartMO2(a.ctx, gameRoot, scriptContent, mo2Args, false); err != nil {
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
	enableGamescope := false

	// Читаем конфиг, чтобы понять, нужен ли нам Gamescope для FSR
	cfg, err := core.GetLauncherConfig(gameRoot)
	if err == nil && cfg != nil {
		// Проверяем условия: включен Wine FSR и это НЕ CommunityShader
		if cfg.FSR && cfg.GrafikMod != "CommunityShader" {
			if _, err := exec.LookPath("gamescope"); err != nil {
				slog.Warn("Gamescope требуется для FSR, но не найден в системе. Отправляем уведомление в UI.")
				// Отправляем сигнал во Vue для отображения компонента GamescopeErrorMessage
				wailsRuntime.EventsEmit(a.ctx, "gamescope-missing")
				enableGamescope = false
			} else {
				slog.Info("Gamescope найден в системе, активируем.")
				enableGamescope = true
			}
		}
	} else {
		slog.Warn("Не удалось прочитать конфиг перед запуском игры", "error", err)
	}

	// Передаем вычисленный флаг enableGamescope 5-м аргументом
	if err := core.StartMO2(a.ctx, gameRoot, scriptContent, mo2Args, enableGamescope); err != nil {
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
	// Если GetGameRoot() находится в другом пакете, поправьте вызов (например, core.GetGameRoot())
	gameRoot := GetGameRoot()

	cfg, err := core.GetLauncherConfig(gameRoot)
	if err != nil {
		if os.IsNotExist(err) {
			return true
		}
		return true
	}
	return !cfg.LinuxPatchComplete
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

func (a *App) FirstInstall() error { ///Патчи совместимости для Linux одноразовая установка
	slog.Info("Начало полного процесса установки (Загрузка + Распаковка)")
	gameRoot := GetGameRoot()
	creds := getCreds()
	offlineConfig := getOfflineConfig()

	// 1. Сигнализируем фронтенду, что началась загрузка
	wailsRuntime.EventsEmit(a.ctx, "update-status", map[string]string{"status": "download-started"})

	// ЭТАП ЗАГРУЗКИ
	downloadCb := func(p float64, speed float64, msg string) {
		wailsRuntime.EventsEmit(a.ctx, "download-progress", map[string]interface{}{
			"fileName":         msg,
			"percentage":       p * 100, // Убедитесь, что здесь приходит число от 0 до 100 (или от 0 до 1, умноженное на 100)
			"speedBytesPerSec": speed,
		})
	}

	if err := core.FirstDownload(a.ctx, gameRoot, creds, offlineConfig, downloadCb); err != nil {
		slog.Error("Ошибка при скачивании", "error", err)
		return err
	}

	// Сигнализируем, что загрузка окончена
	wailsRuntime.EventsEmit(a.ctx, "update-status", map[string]string{"status": "download-finished"})

	// 2. ЭТАП РАСПАКОВКИ
	wailsRuntime.EventsEmit(a.ctx, "update-status", map[string]string{"status": "unpack-started"})

	unpackCb := func(p float64, msg string) {
		wailsRuntime.EventsEmit(a.ctx, "unpack-progress", map[string]interface{}{
			"percentage": p * 100,
		})
	}

	if err := core.FirstInstall(a.ctx, gameRoot, creds, getLibs(), unpackCb); err != nil {
		slog.Error("Ошибка при распаковке", "error", err)
		return err
	}

	// Сигнализируем, что распаковка завершена
	wailsRuntime.EventsEmit(a.ctx, "update-status", map[string]string{"status": "unpack-finished"})

	slog.Info("Установка успешно завершена!")
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
				WineDllOverrides: "concrt140=n;xaudio2_7=n,b;d3d11=n,b;dxgi=n,b;d3dx9_42=n,b;d3dcompiler_47=n,b;dinput8=n,b;mscoree=n;d3d12=n,b;d3d12core=n,b",
				GrafikMod:        "Нету",
				FsrLvl:           "95",
			}
		}
	}

	return *cfg
}

// UpdateSetting сохраняет измененную настройку
func (a *App) UpdateSetting(key string, value interface{}) error {
	slog.Info("UpdateSetting called", "key", key, "value", value)
	gameRoot := GetGameRoot()

	// --- 1. ГОТОВИМ КОЛЛБЭКИ ДЛЯ FRONTEND ---
	unpackCb := func(p float64, msg string) {
		wailsRuntime.EventsEmit(a.ctx, "unpack-progress", map[string]interface{}{
			"percentage": p,
			"message":    msg,
		})
	}

	downloadCb := func(p float64, speed float64, msg string) {
		// Принудительно переключаем UI в режим скачивания (если вызван загрузчик)
		wailsRuntime.EventsEmit(a.ctx, "update-status", map[string]string{"status": "download-started"})
		wailsRuntime.EventsEmit(a.ctx, "download-progress", map[string]interface{}{
			"percentage":       p,
			"speedBytesPerSec": speed,
			"fileName":         msg,
		})
	}

	// --- 2. МАРШРУТИЗАЦИЯ НАСТРОЕК ---
	switch key {
	case "mangoHud":
		utils.SetOneSetting(gameRoot, "MangoHud:", value)

	case "fsr":
		isFSR := false
		if valBool, ok := value.(bool); ok {
			isFSR = valBool
		} else if valStr, ok := value.(string); ok {
			isFSR = (strings.TrimSpace(valStr) == "true")
		}

		wailsRuntime.EventsEmit(a.ctx, "update-status", map[string]string{"status": "unpack-started"})
		if err := fsrswitch.ApplyFSR(a.ctx, gameRoot, isFSR); err != nil {
			wailsRuntime.EventsEmit(a.ctx, "update-status", map[string]string{"status": "process-error"})
			return err
		}
		wailsRuntime.EventsEmit(a.ctx, "update-status", map[string]string{"status": "process-finished"})
		return utils.SetOneSetting(gameRoot, "FSR:", value)

	case "shaderCache":
		utils.SetOneSetting(gameRoot, "ShaderCache:", value)

	case "hdr":
		utils.SetOneSetting(gameRoot, "HDR:", value)

	case "steamFix":
		isSteamFix := false
		if valBool, ok := value.(bool); ok {
			isSteamFix = valBool
		} else if valStr, ok := value.(string); ok {
			isSteamFix = (strings.TrimSpace(valStr) == "true")
		}

		wailsRuntime.EventsEmit(a.ctx, "update-status", map[string]string{"status": "unpack-started"})
		if err := steam_drm_switch.ToggleSteamDRM(a.ctx, gameRoot, isSteamFix, unpackCb); err != nil {
			wailsRuntime.EventsEmit(a.ctx, "update-status", map[string]string{"status": "process-error"})
			return err
		}
		wailsRuntime.EventsEmit(a.ctx, "update-status", map[string]string{"status": "process-finished"})

	case "cdn":
		utils.SetOneSetting(gameRoot, "CDN:", value)

	case "fpsLimit":
		fpsStr := fmt.Sprintf("%v", value)

		// Блокируем интерфейс на долю секунды, чтобы избежать двойных кликов
		wailsRuntime.EventsEmit(a.ctx, "update-status", map[string]string{"status": "unpack-started"})

		patches := []config_patcher.ConfigPatch{
			{
				TargetFile: "MO2/mods/SSE Display Tweaks/SKSE/Plugins/SSEDisplayTweaks.ini",
				ReplacePrefix: map[string]string{
					// Ищем начало строки и заменяем её целиком вместе с новым значением
					"FramerateLimit =": fmt.Sprintf("FramerateLimit = %s", fpsStr),
				},
			},
		}

		// Применяем патч
		if _, err := config_patcher.ApplyPatchesFromJSON(gameRoot, patches, nil); err != nil {
			slog.Error("Ошибка при установке лимита кадров", "error", err)
			wailsRuntime.EventsEmit(a.ctx, "update-status", map[string]string{"status": "process-finished"})
			return fmt.Errorf("ошибка обновления SSEDisplayTweaks.ini: %w", err)
		}

		wailsRuntime.EventsEmit(a.ctx, "update-status", map[string]string{"status": "process-finished"})

		// Сохраняем настройку, чтобы лаунчер запомнил выбор при следующем запуске
		return utils.SetOneSetting(gameRoot, "FPSLimit:", fpsStr)

	case "wineDllOverrides":
		utils.SetOneSetting(gameRoot, "WineDllOverrides:", value)

	case "grafikMod":
		newMod := fmt.Sprintf("%v", value)

		// Блокируем кнопку "Играть" и показываем полосу загрузки
		wailsRuntime.EventsEmit(a.ctx, "update-status", map[string]string{"status": "unpack-started"})

		// Вызываем SwitchGrafikMod
		if err := graficswitch.SwitchGrafikMod(a.ctx, gameRoot, newMod, unpackCb, downloadCb); err != nil {
			slog.Error("Критическая ошибка при смене графического мода", "error", err)
			// Отправляем process-finished, чтобы интерфейс В ЛЮБОМ СЛУЧАЕ снял блокировку (скрыл прогресс-бар)
			wailsRuntime.EventsEmit(a.ctx, "update-status", map[string]string{"status": "process-finished"})
			return err
		}

		if err := fsrswitch.SyncFSRSettings(a.ctx, gameRoot, newMod); err != nil {
			slog.Error("Ошибка подготовки FSR для графического мода", "error", err)
			wailsRuntime.EventsEmit(a.ctx, "update-status", map[string]string{"status": "process-finished"})
			return fmt.Errorf("ошибка подготовки FSR для мода: %w", err)
		}

		// Снимаем блокировку интерфейса при успехе
		wailsRuntime.EventsEmit(a.ctx, "update-status", map[string]string{"status": "process-finished"})
		return utils.SetOneSetting(gameRoot, "GrafikMod:", newMod)

	case "fsrLvl":
		fsrLvlStr := fmt.Sprintf("%v", value)

		utils.SetOneSetting(gameRoot, "FsrLvl:", value)
		err := fsrswitch.SyncFSRSettings(a.ctx, gameRoot, fsrLvlStr)
		if err != nil {
			utils.SetOneSetting(gameRoot, "FsrLvl:", true)
			return err
		}

		return nil

	default:
		slog.Warn("Unknown setting key received", "key", key)
		return nil
	}

	return nil
}

func (a *App) OpenProtonTrics() error {
	slog.Info("OpenWinetricks called")

	// 1. Ищем классический winetricks, так как он умеет работать с любыми папками
	binPath, err := exec.LookPath("winetricks")
	if err != nil {
		slog.Error("winetricks не найден в системе", "error", err)
		return fmt.Errorf("winetricks не установлен: %w", err)
	}

	gameRoot := GetGameRoot()
	if gameRoot == "" {
		return fmt.Errorf("не удалось определить путь к игре")
	}

	// 2. Формируем пути к нашему префиксу и нашему бинарнику Wine из Proton
	prefixPath := filepath.Join(gameRoot, "wine", "prefix", "pfx")
	wineBin := filepath.Join(gameRoot, "wine", "proton", "files", "bin", "wine")

	// 3. Запускаем GUI
	cmd := exec.Command(binPath, "--gui")

	// 4. ВАЖНО: Передаем не только префикс, но и путь к кастомному Wine,
	// чтобы winetricks не пытался использовать системный Wine
	cmd.Env = append(os.Environ(),
		"WINEPREFIX="+prefixPath,
		"WINE="+wineBin,
	)

	if err := cmd.Start(); err != nil {
		slog.Error("Ошибка при запуске winetricks", "error", err)
		return fmt.Errorf("ошибка запуска: %w", err)
	}

	slog.Info("Winetricks успешно запущен для кастомного префикса", "prefix", prefixPath)
	return nil
}

func (a *App) RecoverComponent(key string, force bool) error {
	slog.Info("RecoverComponent called", "key", key, "force", force)

	gameRoot := GetGameRoot()

	switch key {
	case "proton":
		slog.Info("Начата полная переустановка Proton/Wine и префикса")

		wailsRuntime.EventsEmit(a.ctx, "update-status", map[string]string{"status": "download-started"})

		cfg, err := core.GetLauncherConfig(gameRoot)
		if err != nil {
			slog.Warn("Не удалось прочитать конфиг, используем загрузку по умолчанию (GDrive)", "err", err)
			if cfg == nil {
				cfg = &core.LauncherConfig{CDN: false}
			}
		}

		downloadType := "gdrive"
		if cfg.CDN {
			downloadType = "cdn"
		}

		downloadCb := func(p float64, speed float64, msg string) {
			wailsRuntime.EventsEmit(a.ctx, "download-progress", map[string]interface{}{
				"fileName":         msg,
				"percentage":       p * 100,
				"speedBytesPerSec": speed,
			})
		}

		// 1. Скачиваем Proton
		err = downloader.DownloadGEProton(a.ctx, gameRoot, true, downloadCb)
		if err != nil {
			wailsRuntime.EventsEmit(a.ctx, "update-status", map[string]string{"status": "process-error"})
			return fmt.Errorf("ошибка загрузки proton: %w", err)
		}

		// 2. Скачиваем Prefix (так как мы его тоже будем сносить)
		err = downloader.DownloadPrefix(a.ctx, gameRoot, downloadType, getCreds(), true, downloadCb)
		if err != nil {
			wailsRuntime.EventsEmit(a.ctx, "update-status", map[string]string{"status": "process-error"})
			return fmt.Errorf("ошибка загрузки префикса: %w", err)
		}

		wailsRuntime.EventsEmit(a.ctx, "update-status", map[string]string{"status": "unpack-started"})

		unpackCb := func(p float64, msg string) {
			wailsRuntime.EventsEmit(a.ctx, "unpack-progress", map[string]interface{}{
				"percentage": p * 100,
				"message":    msg,
			})
		}

		// 3. Удаляем старые папки
		protonTarget := filepath.Join(gameRoot, "wine", "proton")
		prefixTarget := filepath.Join(gameRoot, "wine", "prefix")

		slog.Info("Удаление старых директорий proton и prefix")
		os.RemoveAll(protonTarget)
		os.RemoveAll(prefixTarget)

		// 4. Распаковываем Proton
		slog.Info("Распаковка Proton")
		// Предполагается, что у вас есть функция proton_install.UnpackProton (по аналогии с prefix_install)
		err = proton_install.InstallGEProton(a.ctx, gameRoot, unpackCb)
		if err != nil {
			wailsRuntime.EventsEmit(a.ctx, "update-status", map[string]string{"status": "process-error"})
			return fmt.Errorf("ошибка распаковки proton: %w", err)
		}

		// 5. Подготавливаем библиотеки и распаковываем Префикс
		slog.Info("Распаковка Prefix")
		if err := core.PrepareEmbeddedLibs(gameRoot, getLibs(), unpackCb); err != nil {
			slog.Warn("Не удалось подготовить встроенные библиотеки при переустановке", "err", err)
		}

		err = prefix_install.UnpackPrefix(a.ctx, gameRoot, unpackCb)
		if err != nil {
			wailsRuntime.EventsEmit(a.ctx, "update-status", map[string]string{"status": "process-error"})
			return fmt.Errorf("ошибка распаковки префикса: %w", err)
		}

		wailsRuntime.EventsEmit(a.ctx, "update-status", map[string]string{"status": "process-finished"})
		slog.Info("Proton и Prefix успешно переустановлены")

	case "prefix":
		if force {
			slog.Info("Начата полная переустановка префикса")

			wailsRuntime.EventsEmit(a.ctx, "update-status", map[string]string{"status": "download-started"})

			cfg, err := core.GetLauncherConfig(gameRoot)
			if err != nil {
				slog.Warn("Не удалось прочитать конфиг, используем загрузку по умолчанию", "err", err)
				if cfg == nil {
					cfg = &core.LauncherConfig{CDN: false}
				}
			}

			downloadType := "gdrive"
			if cfg.CDN {
				downloadType = "cdn"
			}

			downloadCb := func(p float64, speed float64, msg string) {
				wailsRuntime.EventsEmit(a.ctx, "download-progress", map[string]interface{}{
					"fileName":         msg,
					"percentage":       p * 100,
					"speedBytesPerSec": speed,
				})
			}

			err = downloader.DownloadPrefix(a.ctx, gameRoot, downloadType, getCreds(), true, downloadCb)
			if err != nil {
				wailsRuntime.EventsEmit(a.ctx, "update-status", map[string]string{"status": "process-error"})
				return fmt.Errorf("ошибка загрузки префикса: %w", err)
			}

			wailsRuntime.EventsEmit(a.ctx, "update-status", map[string]string{"status": "unpack-started"})

			// 1. Удаляем старый префикс
			prefixTarget := filepath.Join(gameRoot, "wine", "prefix")
			if err := os.RemoveAll(prefixTarget); err != nil {
				slog.Warn("Не удалось удалить старую папку prefix (возможно, её и не было)", "err", err)
			}

			unpackCb := func(p float64, msg string) {
				wailsRuntime.EventsEmit(a.ctx, "unpack-progress", map[string]interface{}{
					"percentage": p * 100,
					"message":    msg,
				})
			}

			// 2. ВАЖНО: Подготавливаем свежие библиотеки из embed перед распаковкой
			if err := core.PrepareEmbeddedLibs(gameRoot, getLibs(), unpackCb); err != nil {
				slog.Warn("Не удалось подготовить встроенные библиотеки при переустановке", "err", err)
			}

			// 3. Распаковываем (UnpackPrefix сам очистит wine/tmp/libs и создаст симлинки)
			err = prefix_install.UnpackPrefix(a.ctx, gameRoot, unpackCb)
			if err != nil {
				wailsRuntime.EventsEmit(a.ctx, "update-status", map[string]string{"status": "process-error"})
				return fmt.Errorf("ошибка распаковки префикса: %w", err)
			}

			wailsRuntime.EventsEmit(a.ctx, "update-status", map[string]string{"status": "process-finished"})
			slog.Info("Префикс успешно переустановлен")

		} else {
			slog.Info("Начато лечение префикса (перераспаковка и настройка)")

			wailsRuntime.EventsEmit(a.ctx, "update-status", map[string]string{"status": "unpack-started"})

			prefixDir := filepath.Join(gameRoot, "wine", "prefix")

			// 1. Сносим старый сломанный префикс
			slog.Info("Удаление старой директории префикса", "dir", prefixDir)
			if err := os.RemoveAll(prefixDir); err != nil {
				slog.Warn("Не удалось полностью удалить старый префикс, возможны конфликты", "error", err)
			}

			// 2. Запускаем единый конвейер установки
			err := prefix_install.UnpackPrefix(a.ctx, gameRoot, func(p float64, msg string) {
				wailsRuntime.EventsEmit(a.ctx, "unpack-progress", map[string]interface{}{
					"percentage": p,
					"message":    msg,
				})
			})

			if err != nil {
				slog.Error("Ошибка при лечении префикса", "error", err)
				wailsRuntime.EventsEmit(a.ctx, "update-status", map[string]string{"status": "process-error"})
				return err
			}

			wailsRuntime.EventsEmit(a.ctx, "update-status", map[string]string{"status": "process-finished"})
			slog.Info("Лечение префикса успешно завершено")
		}

	case "steamfix":
		slog.Info("Начато восстановление Steam Fix")

		wailsRuntime.EventsEmit(a.ctx, "update-status", map[string]string{"status": "unpack-started"})

		steamFixOnDir := filepath.Join(gameRoot, "disabledGameFiles", "SteamDRM", "on")

		slog.Info("Очистка директории SteamFix", "dir", steamFixOnDir)
		if err := os.RemoveAll(steamFixOnDir); err != nil {
			slog.Warn("Не удалось удалить старую директорию SteamFix", "error", err)
		}

		err := unpacksteamfix.UnpackSteamFix(a.ctx, gameRoot, func(p float64, msg string) {
			wailsRuntime.EventsEmit(a.ctx, "unpack-progress", map[string]interface{}{
				"percentage": p,
				"message":    msg,
			})
		})

		if err != nil {
			slog.Error("Ошибка при распаковке SteamFix", "error", err)
			wailsRuntime.EventsEmit(a.ctx, "update-status", map[string]string{"status": "process-error"})
			return err
		}

		// Применяем SteamFix к игре
		if err := steam_drm_switch.ToggleSteamDRM(a.ctx, gameRoot, false, func(p float64, msg string) {
			wailsRuntime.EventsEmit(a.ctx, "unpack-progress", map[string]interface{}{
				"percentage": p,
				"message":    msg,
			})
		}); err != nil {
			slog.Error("Ошибка при переключении Steam DRM", "error", err)
			wailsRuntime.EventsEmit(a.ctx, "update-status", map[string]string{"status": "process-error"})
			return err
		}

		wailsRuntime.EventsEmit(a.ctx, "update-status", map[string]string{"status": "process-finished"})
		slog.Info("Восстановление Steam Fix успешно завершено")

	default:
		slog.Warn("Попытка восстановить неизвестный компонент", "key", key)
		return fmt.Errorf("неизвестный компонент для восстановления: %s", key)
	}

	return nil
}
func (a *App) CheckCSFilesExist() bool {
	gameRoot := GetGameRoot()
	downloadDir := filepath.Join(gameRoot, "download")

	hasCS := false
	hasUp := false

	entries, err := os.ReadDir(downloadDir)
	if err != nil {
		return false
	}

	for _, e := range entries {
		if !e.IsDir() {
			name := strings.ToLower(e.Name())
			// Проверяем все поддерживаемые форматы архивов
			if strings.HasSuffix(name, ".zip") || strings.HasSuffix(name, ".7z") || strings.HasSuffix(name, ".rar") || strings.HasSuffix(name, ".tar.gz") || strings.HasSuffix(name, ".gz") || strings.HasSuffix(name, ".tar") {
				if strings.Contains(name, "community") {
					hasCS = true
				}
				if strings.Contains(name, "upscal") {
					hasUp = true
				}
			}
		}
	}

	return hasCS && hasUp
}

func (a *App) Quit() {
	slog.Info("Quit called")
	wailsRuntime.Quit(a.ctx)
}

// Minimize сворачивает окно
func (a *App) Minimize() {
	slog.Info("Minimize called")
	wailsRuntime.WindowMinimise(a.ctx)
}
