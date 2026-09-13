package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
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

	application "github.com/wailsapp/wails/v3/pkg/application"
)

type GameSettings struct {
	MangoHud         bool   `json:"mangoHud"`
	Fsr              bool   `json:"fsr"`
	ShaderCache      bool   `json:"shaderCache"`
	Hdr              bool   `json:"hdr"`
	SteamFix         bool   `json:"steamFix"`
	FpsLimit         string `json:"fpsLimit"`
	WineDllOverrides string `json:"wineDllOverrides"`
	GrafikMod        string `json:"grafikMod"`
	FsrLvl           string `json:"fsrLvl"`
}

const systemConfigPath = "~/.config/rfad-launcher/launcher.conf"

type App struct {
}

func NewApp() *App {
	return &App{}
}

func (a *App) startup() {
	slog.Info("App started")
}

func (a *App) shutdown() {
	slog.Info("App shutting down")
}

// ServiceStartup вызывается Wails при старте приложения.
func (a *App) ServiceStartup(options application.ServiceOptions) error {
	a.startup()
	return nil
}

// ServiceShutdown вызывается Wails при завершении приложения.
func (a *App) ServiceShutdown() error {
	a.shutdown()
	return nil
}

func (a *App) IsPathExist() bool {
	filePath := filepath.Join(GetGameRoot(), "MO2", "ModOrganizer.exe")

	_, err := os.Stat(filePath)
	if err == nil {
		return true
	}
	if errors.Is(err, os.ErrNotExist) {
		return false
	}

	panic("unknown error")
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

	data = bytes.TrimPrefix(data, []byte("\xef\xbb\xbf"))

	version := strings.TrimSpace(string(data))

	if version == "" {
		return "0.0"
	}

	return version
}

func (a *App) GetRemoteVersion() string {
	slog.Info("GetRemoteVersion called")

	client := &http.Client{
		Timeout: 10 * time.Second,
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

func (a *App) Update() error {
	slog.Info("Начало процесса обновления игры")

	gameRoot := GetGameRoot()
	creds := getCreds()

	// ==========================================
	// 1. ЭТАП ЗАГРУЗКИ ОБНОВЛЕНИЯ
	// ==========================================
	application.Get().Event.Emit("update-status", map[string]string{"status": "download-started"})

	downloadCb := func(p float64, speed float64, msg string) {
		speedMB := speed / 1024 / 1024
		application.Get().Event.Emit("download-progress", map[string]interface{}{
			"fileName":         msg,
			"percentage":       p * 100,
			"speedBytesPerSec": fmt.Sprintf("%.1f", speedMB),
		})
	}

	if err := rfad_update.DownloadUpdate(gameRoot, creds, downloadCb); err != nil {
		slog.Error("Ошибка при скачивании обновления", "error", err)
		return err
	}

	application.Get().Event.Emit("update-status", map[string]string{"status": "download-finished"})

	application.Get().Event.Emit("update-status", map[string]string{"status": "unpack-started"})

	unpackCb := func(p float64, msg string) {
		application.Get().Event.Emit("unpack-progress", map[string]interface{}{
			"percentage": p * 100,
		})
	}

	if err := rfad_update.InstallUpdate(gameRoot, unpackCb); err != nil {
		slog.Error("Ошибка при распаковке обновления", "error", err)
		return err
	}

	application.Get().Event.Emit("update-status", map[string]string{"status": "unpack-finished"})
	slog.Info("Обновление успешно завершено!")

	return nil
}

func (a *App) OpenExplorer() error {
	slog.Info("OpenExplorer called")
	cmd := exec.Command("xdg-open", GetGameRoot())

	err := cmd.Start()
	if err != nil {
		slog.Warn("Не удалось открыть директорию", "err", err)
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

	if err := core.StartMO2(gameRoot, scriptContent, mo2Args, false); err != nil {
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
	cfg, err := utils.GetLauncherConfig()
	if err == nil && cfg != nil {
		// Проверяем условия: включен Wine FSR и это НЕ CommunityShader
		if cfg.FSR && cfg.GrafikMod != "CommunityShader" {
			if _, err := exec.LookPath("gamescope"); err != nil {
				slog.Warn("Gamescope требуется для FSR, но не найден в системе. Отправляем уведомление в UI.")
				// Отправляем сигнал во Vue для отображения компонента GamescopeErrorMessage
				application.Get().Event.Emit("gamescope-missing")
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
	if err := core.StartMO2(gameRoot, scriptContent, mo2Args, enableGamescope); err != nil {
		slog.Error("StartGame failed", "error", err)
		return err
	}

	return nil
}

func (a *App) GetLauncherVersion() string {
    return version
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

func (a *App) GetFirstInstallStatus() bool {
	cfg, err := utils.GetLauncherConfig()
	if err != nil {
		slog.Warn("Не удалось прочитать конфиг при проверке FirstInstall", "error", err)
		return true
	}
	return !cfg.LinuxPatchComplete
}

func (a *App) InstallGame(installerPath, installPath, oldMo2Path string) error {
	cacheDir := filepath.Join(installPath, "tmp")

	err := core.InstallGame(installerPath, installPath, oldMo2Path, cacheDir, getInnoextract(), func(p float64, msg string) {
		application.Get().Event.Emit("install-progress", map[string]interface{}{
			"percentage": p,
			"message":    msg,
		})
	})

	if err != nil {
		slog.Error("InstallGame failed", "error", err)
		a.ShowMessageDialog("Ошибка", fmt.Sprintf("Не удалось установить игру: %v", err))
		return err
	}

	return nil
}

func (a *App) SaveGlobalPath(mo2ExePath string) error {
	mo2ExePath = strings.Trim(mo2ExePath, "\"' ")

	gameRoot := filepath.Dir(filepath.Dir(mo2ExePath))

	slog.Info("Попытка сохранить глобальный путь", "mo2ExePath", mo2ExePath, "gameRoot", gameRoot)

	if err := utils.SetOneSetting("gameDir", gameRoot); err != nil {
		slog.Error("Не удалось сохранить путь к игре", "error", err)
		return fmt.Errorf("ошибка сохранения пути: %w", err)
	}
	slog.Info("Путь к игре успешно сохранен", "gameDir", gameRoot)

	a.portOldConfig(gameRoot)

	return nil
}

func (a *App) portOldConfig(gameRoot string) {
	oldConfigPath := filepath.Join(gameRoot, "launcher_config.txt")

	file, err := os.Open(oldConfigPath)
	if err != nil {
		// Если файла нет (например, свежая ручная распаковка), просто игнорируем
		return
	}
	defer file.Close()

	slog.Info("Найден старый launcher_config.txt, начинаем миграцию настроек")
	scanner := bufio.NewScanner(file)

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		// Парсим старый формат "Key: value"
		parts := strings.SplitN(line, ":", 2)
		if len(parts) != 2 {
			continue
		}

		key := strings.TrimSpace(parts[0])
		val := strings.TrimSpace(parts[1])
		val = strings.Trim(val, `"'`) // Очищаем от кавычек на всякий случай

		// Обрабатываем пустые значения (например "FpsLimit:  ")
		if val == "" {
			if key == "FpsLimit" {
				val = "60" // Дефолтное значение для фронтенда
			} else {
				val = "false"
			}
		}

		// Сохраняем в новый глобальный конфиг (utils.SetOneSetting уже использует формат с пробелом)
		if err := utils.SetOneSetting(key, val); err != nil {
			slog.Warn("Не удалось портировать настройку", "key", key, "error", err)
		}
	}

	if err := scanner.Err(); err != nil {
		slog.Error("Ошибка при чтении старого конфига", "error", err)
	}

	// Закрываем файл перед переименованием
	file.Close()

	// Переименовываем старый файл, чтобы завершить миграцию
	backupPath := oldConfigPath + ".bak"
	if err := os.Rename(oldConfigPath, backupPath); err != nil {
		slog.Warn("Не удалось переименовать старый конфиг", "error", err)
	} else {
		slog.Info("Старый конфиг сохранен как бэкап", "path", backupPath)
	}
}

func (a *App) FirstInstall() error { ///Патчи совместимости для Linux одноразовая установка
	slog.Info("Начало полного процесса установки (Загрузка + Распаковка)")
	gameRoot := GetGameRoot()
	creds := getCreds()
	offlineConfig := getOfflineConfig()

	// 1. Сигнализируем фронтенду, что началась загрузка
	application.Get().Event.Emit("update-status", map[string]string{"status": "download-started"})

	// ЭТАП ЗАГРУЗКИ
	downloadCb := func(p float64, speed float64, msg string) {
		application.Get().Event.Emit("download-progress", map[string]interface{}{
			"fileName":         msg,
			"percentage":       p * 100, // Убедитесь, что здесь приходит число от 0 до 100 (или от 0 до 1, умноженное на 100)
			"speedBytesPerSec": speed,
		})
	}

	if err := core.FirstDownload(gameRoot, creds, offlineConfig, downloadCb); err != nil {
		slog.Error("Ошибка при скачивании", "error", err)
		return err
	}

	// Сигнализируем, что загрузка окончена
	application.Get().Event.Emit("update-status", map[string]string{"status": "download-finished"})

	// 2. ЭТАП РАСПАКОВКИ
	application.Get().Event.Emit("update-status", map[string]string{"status": "unpack-started"})

	unpackCb := func(p float64, msg string) {
		application.Get().Event.Emit("unpack-progress", map[string]interface{}{
			"percentage": p * 100,
		})
	}

	if err := core.FirstInstall(gameRoot, creds, getLibs(), unpackCb); err != nil {
		slog.Error("Ошибка при распаковке", "error", err)
		return err
	}

	// Сигнализируем, что распаковка завершена
	application.Get().Event.Emit("update-status", map[string]string{"status": "unpack-finished"})

	slog.Info("Установка успешно завершена!")
	return nil
}

func GetGameRoot() string {
	// 1. Тестовая переменная окружения (высший приоритет для отладки)
	if testRoot := os.Getenv("RF_TEST_GAME_ROOT"); testRoot != "" {
		slog.Info("Используется тестовый путь: " + testRoot)
		return testRoot
	}

	// 2. Ищем сохраненный путь в глобальном конфиге пользователя
	if savedDir, err := utils.GetOneSetting("gameDir"); err == nil && savedDir != "" {
		slog.Info("Путь к игре найден в конфиге: " + savedDir)
		return savedDir
	}

	// Если ничего не найдено — возвращаем пустую строку
	slog.Info("Путь к игре не найден, требуется ручная настройка")
	return ""
}

func (a *App) IntegrateAppImageAndRelaunch() error {
	appImagePath := os.Getenv("APPIMAGE")

	// Если переменная APPIMAGE пуста, значит это DEB, RPM или запуск из исходников.
	// Ничего не перемещаем и не перезапускаем.
	if appImagePath == "" {
		slog.Info("Запуск не из AppImage (DEB/RPM). Перемещение и создание ярлыка не требуется.")
		return nil
	}

	// === Логика только для AppImage ===

	homeDir, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("не удалось получить домашнюю директорию: %w", err)
	}

	// Безопасное место для AppImage (стандарт для Linux)
	targetDir := filepath.Join(homeDir, "Applications")
	if err := os.MkdirAll(targetDir, 0755); err != nil {
		return fmt.Errorf("не удалось создать директорию %s: %w", targetDir, err)
	}

	targetExe := filepath.Join(targetDir, "RFADLauncher.AppImage")

	// Если AppImage уже запущен из правильного места, пропускаем
	if appImagePath == targetExe {
		slog.Info("AppImage уже находится в Applications, пропускаем копирование")
		return nil
	}

	_ = os.Remove(targetExe)

	// Копируем AppImage используя нашу надежную функцию (которая попытается сделать Hardlink)
	slog.Info("Интеграция AppImage", "source", appImagePath, "target", targetExe)
	if err := utils.CopyFile(appImagePath, targetExe); err != nil {
		return fmt.Errorf("не удалось скопировать AppImage: %w", err)
	}

	// Создаем ярлык в меню приложений пользователя
	desktopDir := filepath.Join(homeDir, ".local", "share", "applications")
	if err := os.MkdirAll(desktopDir, 0755); err == nil {
		desktopFile := filepath.Join(desktopDir, "rfad-launcher.desktop")

		// Базовый .desktop файл.
		// При желании можно добавить путь к иконке (Icon=/путь/к/иконке.png),
		// если она лежит рядом или извлекается
		desktopContent := fmt.Sprintf(`[Desktop Entry]
Name=RFAD SE Launcher
Comment=Управление и запуск сборки RFAD
Exec="%s"
Icon=utilities-terminal
Terminal=false
Type=Application
Categories=Game;
`, targetExe)

		if writeErr := os.WriteFile(desktopFile, []byte(desktopContent), 0644); writeErr != nil {
			slog.Warn("Не удалось создать ярлык .desktop", "error", writeErr)
		} else {
			slog.Info("Ярлык приложения успешно создан", "path", desktopFile)
		}
	}

	// Запускаем перенесенный AppImage
	cmd := exec.Command(targetExe)
	cmd.Env = os.Environ()

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("не удалось запустить интегрированный AppImage: %w", err)
	}

	slog.Info("Перезапуск из интегрированного AppImage", "path", targetExe)

	// Даем новому процессу время на старт и "убиваем" текущий (из папки Загрузки)
	go func() {
		time.Sleep(500 * time.Millisecond)
		os.Exit(0)
	}()

	return nil
}

func (a *App) AutoIntegrateAppImage() {
	appImagePath := os.Getenv("APPIMAGE")
	
	// Если это не AppImage (запуск DEB/RPM/go run), просто выходим
	if appImagePath == "" {
		return
	}

	homeDir, err := os.UserHomeDir()
	if err != nil {
		slog.Error("AutoIntegrate: не удалось получить домашнюю директорию", "error", err)
		return
	}

	targetExe := filepath.Join(homeDir, "Applications", "RFADLauncher.AppImage")

	// Если мы уже запущены из безопасного места, ничего не делаем — можно работать
	if appImagePath == targetExe {
		slog.Info("AppImage запущен из безопасной директории, продолжаем работу")
		return
	}

	// Если пути не совпадают (например, запуск из ~/Загрузки), принудительно интегрируем и перезапускаем
	slog.Info("AppImage запущен извне безопасной директории. Начинаем интеграцию (обновление)...", 
		"currentPath", appImagePath, 
		"targetPath", targetExe)
		
	if err := a.IntegrateAppImageAndRelaunch(); err != nil {
		slog.Error("Не удалось выполнить автоинтеграцию AppImage", "error", err)
	}
}

func (a *App) GetGameSettings() utils.LauncherConfig {
	cfg, err := utils.GetLauncherConfig()
	if err != nil {
		slog.Warn("Не удалось прочитать настройки для UI, используем дефолтные", "err", err)
		if cfg == nil {
			cfg = &utils.LauncherConfig{
				MangoHud:         false,
				FSR:              false,
				ShaderCache:      false,
				HDR:              false,
				SteamFix:         false,
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
		application.Get().Event.Emit("unpack-progress", map[string]interface{}{
			"percentage": p,
			"message":    msg,
		})
	}

	downloadCb := func(p float64, speed float64, msg string) {
		// Принудительно переключаем UI в режим скачивания (если вызван загрузчик)
		application.Get().Event.Emit("update-status", map[string]string{"status": "download-started"})
		application.Get().Event.Emit("download-progress", map[string]interface{}{
			"percentage":       p,
			"speedBytesPerSec": speed,
			"fileName":         msg,
		})
	}

	// --- 2. МАРШРУТИЗАЦИЯ НАСТРОЕК ---
	switch key {
	case "mangoHud":
		utils.SetOneSetting("MangoHud:", value)

	case "fsr":
		cfg, err := utils.GetLauncherConfig()
		if err != nil {
			slog.Error("Не удалось прочитать конфиг перед сменой FSR уровня", "error", err)
			return err
		}

		application.Get().Event.Emit("update-status", map[string]string{"status": "unpack-started"})
		if err := fsrswitch.SyncFSRSettings(gameRoot, cfg.GrafikMod); err != nil {
			application.Get().Event.Emit("update-status", map[string]string{"status": "process-error"})
			return err
		}
		application.Get().Event.Emit("update-status", map[string]string{"status": "process-finished"})
		return utils.SetOneSetting("FSR:", value)

	case "shaderCache":
		utils.SetOneSetting("ShaderCache:", value)

	case "hdr":
		utils.SetOneSetting("HDR:", value)

	case "steamFix":
		isSteamFix := false
		if valBool, ok := value.(bool); ok {
			isSteamFix = valBool
		} else if valStr, ok := value.(string); ok {
			isSteamFix = (strings.TrimSpace(valStr) == "true")
		}

		application.Get().Event.Emit("update-status", map[string]string{"status": "unpack-started"})
		if err := steam_drm_switch.ToggleSteamDRM(gameRoot, isSteamFix, unpackCb); err != nil {
			application.Get().Event.Emit("update-status", map[string]string{"status": "process-error"})
			return err
		}
		application.Get().Event.Emit("update-status", map[string]string{"status": "process-finished"})

	case "fpsLimit":
		fpsStr := fmt.Sprintf("%v", value)

		// Блокируем интерфейс на долю секунды, чтобы избежать двойных кликов
		application.Get().Event.Emit("update-status", map[string]string{"status": "unpack-started"})

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
			application.Get().Event.Emit("update-status", map[string]string{"status": "process-finished"})
			return fmt.Errorf("ошибка обновления SSEDisplayTweaks.ini: %w", err)
		}

		application.Get().Event.Emit("update-status", map[string]string{"status": "process-finished"})

		// Сохраняем настройку, чтобы лаунчер запомнил выбор при следующем запуске
		return utils.SetOneSetting("FPSLimit:", fpsStr)

	case "wineDllOverrides":
		utils.SetOneSetting("WineDllOverrides:", value)

	case "grafikMod":
		newMod := fmt.Sprintf("%v", value)

		// Блокируем кнопку "Играть" и показываем полосу загрузки
		application.Get().Event.Emit("update-status", map[string]string{"status": "unpack-started"})

		// Вызываем SwitchGrafikMod
		if err := graficswitch.SwitchGrafikMod(gameRoot, newMod, unpackCb, downloadCb); err != nil {
			slog.Error("Критическая ошибка при смене графического мода", "error", err)
			// Отправляем process-finished, чтобы интерфейс В ЛЮБОМ СЛУЧАЕ снял блокировку (скрыл прогресс-бар)
			application.Get().Event.Emit("update-status", map[string]string{"status": "process-finished"})
			return err
		}

		if err := fsrswitch.SyncFSRSettings(gameRoot, newMod); err != nil {
			slog.Error("Ошибка подготовки FSR для графического мода", "error", err)
			application.Get().Event.Emit("update-status", map[string]string{"status": "process-finished"})
			return fmt.Errorf("ошибка подготовки FSR для мода: %w", err)
		}

		// Снимаем блокировку интерфейса при успехе
		application.Get().Event.Emit("update-status", map[string]string{"status": "process-finished"})
		return utils.SetOneSetting("GrafikMod:", newMod)

	case "fsrLvl":
		cfg, err := utils.GetLauncherConfig()
		if err != nil {
			slog.Error("Не удалось прочитать конфиг перед сменой FSR уровня", "error", err)
			return err
		}

		utils.SetOneSetting("FsrLvl:", value)
		err = fsrswitch.SyncFSRSettings(gameRoot, cfg.GrafikMod)
		if err != nil {
			utils.SetOneSetting("FsrLvl:", true)
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

		application.Get().Event.Emit("update-status", map[string]string{"status": "download-started"})

		downloadCb := func(p float64, speed float64, msg string) {
			application.Get().Event.Emit("download-progress", map[string]interface{}{
				"fileName":         msg,
				"percentage":       p * 100,
				"speedBytesPerSec": speed,
			})
		}

		// 1. Скачиваем Proton
		err := downloader.DownloadGEProton(gameRoot, true, downloadCb)
		if err != nil {
			application.Get().Event.Emit("update-status", map[string]string{"status": "process-error"})
			return fmt.Errorf("ошибка загрузки proton: %w", err)
		}

		// 2. Скачиваем Prefix (так как мы его тоже будем сносить)
		err = downloader.DownloadPrefix(gameRoot, getCreds(), true, downloadCb)
		if err != nil {
			application.Get().Event.Emit("update-status", map[string]string{"status": "process-error"})
			return fmt.Errorf("ошибка загрузки префикса: %w", err)
		}

		application.Get().Event.Emit("update-status", map[string]string{"status": "unpack-started"})

		unpackCb := func(p float64, msg string) {
			application.Get().Event.Emit("unpack-progress", map[string]interface{}{
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
		err = proton_install.InstallGEProton(gameRoot, unpackCb)
		if err != nil {
			application.Get().Event.Emit("update-status", map[string]string{"status": "process-error"})
			return fmt.Errorf("ошибка распаковки proton: %w", err)
		}

		// 5. Подготавливаем библиотеки и распаковываем Префикс
		slog.Info("Распаковка Prefix")
		if err := core.PrepareEmbeddedLibs(gameRoot, getLibs(), unpackCb); err != nil {
			slog.Warn("Не удалось подготовить встроенные библиотеки при переустановке", "err", err)
		}

		err = prefix_install.UnpackPrefix(gameRoot, unpackCb)
		if err != nil {
			application.Get().Event.Emit("update-status", map[string]string{"status": "process-error"})
			return fmt.Errorf("ошибка распаковки префикса: %w", err)
		}

		application.Get().Event.Emit("update-status", map[string]string{"status": "process-finished"})
		slog.Info("Proton и Prefix успешно переустановлены")

	case "prefix":
		if force {
			slog.Info("Начата полная переустановка префикса")

			application.Get().Event.Emit("update-status", map[string]string{"status": "download-started"})

			downloadCb := func(p float64, speed float64, msg string) {
				application.Get().Event.Emit("download-progress", map[string]interface{}{
					"fileName":         msg,
					"percentage":       p * 100,
					"speedBytesPerSec": speed,
				})
			}

			err := downloader.DownloadPrefix(gameRoot, getCreds(), true, downloadCb)
			if err != nil {
				application.Get().Event.Emit("update-status", map[string]string{"status": "process-error"})
				return fmt.Errorf("ошибка загрузки префикса: %w", err)
			}

			application.Get().Event.Emit("update-status", map[string]string{"status": "unpack-started"})

			// 1. Удаляем старый префикс
			prefixTarget := filepath.Join(gameRoot, "wine", "prefix")
			if err := os.RemoveAll(prefixTarget); err != nil {
				slog.Warn("Не удалось удалить старую папку prefix (возможно, её и не было)", "err", err)
			}

			unpackCb := func(p float64, msg string) {
				application.Get().Event.Emit("unpack-progress", map[string]interface{}{
					"percentage": p * 100,
					"message":    msg,
				})
			}

			// 2. ВАЖНО: Подготавливаем свежие библиотеки из embed перед распаковкой
			if err := core.PrepareEmbeddedLibs(gameRoot, getLibs(), unpackCb); err != nil {
				slog.Warn("Не удалось подготовить встроенные библиотеки при переустановке", "err", err)
			}

			// 3. Распаковываем (UnpackPrefix сам очистит wine/tmp/libs и создаст симлинки)
			err = prefix_install.UnpackPrefix(gameRoot, unpackCb)
			if err != nil {
				application.Get().Event.Emit("update-status", map[string]string{"status": "process-error"})
				return fmt.Errorf("ошибка распаковки префикса: %w", err)
			}

			application.Get().Event.Emit("update-status", map[string]string{"status": "process-finished"})
			slog.Info("Префикс успешно переустановлен")

		} else {
			slog.Info("Начато лечение префикса (перераспаковка и настройка)")

			application.Get().Event.Emit("update-status", map[string]string{"status": "unpack-started"})

			prefixDir := filepath.Join(gameRoot, "wine", "prefix")

			// 1. Сносим старый сломанный префикс
			slog.Info("Удаление старой директории префикса", "dir", prefixDir)
			if err := os.RemoveAll(prefixDir); err != nil {
				slog.Warn("Не удалось полностью удалить старый префикс, возможны конфликты", "error", err)
			}

			// 2. Запускаем единый конвейер установки
			err := prefix_install.UnpackPrefix(gameRoot, func(p float64, msg string) {
				application.Get().Event.Emit("unpack-progress", map[string]interface{}{
					"percentage": p,
					"message":    msg,
				})
			})

			if err != nil {
				slog.Error("Ошибка при лечении префикса", "error", err)
				application.Get().Event.Emit("update-status", map[string]string{"status": "process-error"})
				return err
			}

			application.Get().Event.Emit("update-status", map[string]string{"status": "process-finished"})
			slog.Info("Лечение префикса успешно завершено")
		}

	case "steamfix":
		slog.Info("Начато восстановление Steam Fix")

		application.Get().Event.Emit("update-status", map[string]string{"status": "unpack-started"})

		steamFixOnDir := filepath.Join(gameRoot, "disabledGameFiles", "SteamDRM", "on")

		slog.Info("Очистка директории SteamFix", "dir", steamFixOnDir)
		if err := os.RemoveAll(steamFixOnDir); err != nil {
			slog.Warn("Не удалось удалить старую директорию SteamFix", "error", err)
		}

		err := unpacksteamfix.UnpackSteamFix(gameRoot, func(p float64, msg string) {
			application.Get().Event.Emit("unpack-progress", map[string]interface{}{
				"percentage": p,
				"message":    msg,
			})
		})

		if err != nil {
			slog.Error("Ошибка при распаковке SteamFix", "error", err)
			application.Get().Event.Emit("update-status", map[string]string{"status": "process-error"})
			return err
		}

		// Применяем SteamFix к игре
		if err := steam_drm_switch.ToggleSteamDRM(gameRoot, false, func(p float64, msg string) {
			application.Get().Event.Emit("unpack-progress", map[string]interface{}{
				"percentage": p,
				"message":    msg,
			})
		}); err != nil {
			slog.Error("Ошибка при переключении Steam DRM", "error", err)
			application.Get().Event.Emit("update-status", map[string]string{"status": "process-error"})
			return err
		}

		application.Get().Event.Emit("update-status", map[string]string{"status": "process-finished"})
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
	application.Get().Quit()
}

func (a *App) Minimize() {
	slog.Info("Minimize called")
	if w, ok := application.Get().Window.GetByName("main"); ok {
		w.Minimise()
	}
}

// SelectFile открывает системное окно выбора файла
func (a *App) SelectFile() (string, error) {
	app := application.Get()

	path, err := app.Dialog.
		OpenFile().
		SetTitle("Выберите установщик").
		AddFilter("Установочные файлы", "*.exe;*.msi").
		AddFilter("Все файлы", "*.*").
		PromptForSingleSelection()

	if err != nil {
		return "", err
	}
	return path, nil
}

// SelectDirectory открывает системное окно выбора папки
func (a *App) SelectDirectory() (string, error) {
	app := application.Get()

	path, err := app.Dialog.
		OpenFile().
		SetTitle("Выберите папку для установки").
		CanChooseDirectories(true).
		CanChooseFiles(false).
		PromptForSingleSelection()

	if err != nil {
		return "", err
	}
	return path, nil
}

// ShowMessageDialog показывает простое информационное окно
func (a *App) ShowMessageDialog(title, message string) {
	app := application.Get()

	// Просто вызываем цепочку методов, не пытаясь вернуть результат
	app.Dialog.
		Info().
		SetTitle(title).
		SetMessage(message).
		Show()
}
