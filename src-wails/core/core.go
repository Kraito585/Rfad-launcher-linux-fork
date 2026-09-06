package core

import (
	"bufio"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"rfad-launcher-linux/src-wails/downloader"
	core "rfad-launcher-linux/src-wails/loger"
	config_patcher "rfad-launcher-linux/src-wails/patches/patch_configs"
	"rfad-launcher-linux/src-wails/patches/prefix_install"
	"rfad-launcher-linux/src-wails/patches/proton_install"
	"rfad-launcher-linux/src-wails/patches/rfad_update"
	unpacksteamfix "rfad-launcher-linux/src-wails/patches/unpack_steam_fix"
	"rfad-launcher-linux/src-wails/utils"
	"rfad-launcher-linux/src-wails/utils/steam_drm_switch"
	"runtime"
	"strconv"
	"strings"
	"time"

	wailsRuntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

type LauncherConfig struct {
	LinuxPatchComplete bool   `json:"linuxPatchComplete"`
	MangoHud           bool   `json:"mangoHud"`
	FSR                bool   `json:"fsr"`
	ShaderCache        bool   `json:"shaderCache"`
	HDR                bool   `json:"hdr"`
	SteamFix           bool   `json:"steamFix"`
	FpsLimit           string `json:"fpsLimit"`
	CDN                bool   `json:"cdn"`
	WineDllOverrides   string `json:"wineDllOverrides"`
	GrafikMod          string `json:"grafikMod"`
	FsrLvl             string `json:"fsrLvl"`
}

func parseBool(val string) bool {
	return strings.ToLower(val) == "true"
}

func GetLauncherConfig(gameRoot string) (*LauncherConfig, error) {
	configPath := filepath.Join(gameRoot, "launcher_config.txt")

	file, err := os.Open(configPath)
	if err != nil {
		// Если файла нет, возвращаем пустой конфиг (со значениями по умолчанию)
		return &LauncherConfig{}, err
	}
	defer file.Close()

	cfg := &LauncherConfig{}
	scanner := bufio.NewScanner(file)

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		if line == "" {
			continue
		}

		parts := strings.SplitN(line, ":", 2)
		if len(parts) != 2 {
			continue
		}

		key := strings.TrimSpace(parts[0])

		val := strings.TrimSpace(parts[1])
		val = strings.Trim(val, `"'`)

		switch key {
		case "linux-patch-complite":
			cfg.LinuxPatchComplete = parseBool(val)
		case "MangoHud":
			cfg.MangoHud = parseBool(val)
		case "FSR":
			cfg.FSR = parseBool(val)
		case "ShaderCache":
			cfg.ShaderCache = parseBool(val)
		case "HDR":
			cfg.HDR = parseBool(val)
		case "SteamFix":
			cfg.SteamFix = parseBool(val)
		case "FpsLimit":
			cfg.FpsLimit = val
		case "CDN":
			cfg.CDN = parseBool(val)
		case "WineDllOverrides":
			cfg.WineDllOverrides = val
		case "GrafikMod":
			cfg.GrafikMod = val
		case "FsrLvl":
			cfg.FsrLvl = val
		}
	}

	if err := scanner.Err(); err != nil {
		return cfg, err
	}

	return cfg, nil
}

func useNvapi() bool {
	if _, err := os.Stat("/proc/driver/nvidia"); err != nil {
		core.LogInfo("NVAPI не требуется или драйвер NVIDIA не установлен")
		return false
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "nvidia-smi", "--query-gpu=name", "--format=csv,noheader")
	out, err := cmd.Output()
	if err != nil {
		core.LogInfo("NVAPI: ошибка выполнения nvidia-smi")
		return false
	}

	hasNVAPI := false

	lines := strings.Split(strings.TrimSpace(string(out)), "\n")

	for _, line := range lines {
		gpuName := strings.ToUpper(strings.TrimSpace(line))

		if strings.Contains(gpuName, "RTX") {
			hasNVAPI = true
			core.LogInfo("Найдена видеокарта с поддержкой NVAPI: %s", gpuName)
			break
		}
	}

	if !hasNVAPI {
		core.LogInfo("NVAPI не требуется: подходящая видеокарта NVIDIA не найдена")
	}

	return hasNVAPI
}

func StartMO2(ctx context.Context, gameRoot string, scriptContent, mo2Args string, isGameLaunch bool) error {
	// 1. Считываем настройки лаунчера
	cfg, err := GetLauncherConfig(gameRoot)
	if err != nil {
		core.LogInfo("Не удалось прочитать launcher_config.txt, используются настройки по умолчанию: %v", err)
		if cfg == nil {
			// На всякий случай задаем дефолтные dll overrides, если файла вообще нет
			cfg = &LauncherConfig{
				WineDllOverrides: "concrt140=n;xaudio2_7=n,b;d3d11=n,b;dxgi=n,b;d3dx9_42=n,b;d3dcompiler_47=n,b;dinput8=n,b;mscoree=n;d3d12=n,b;d3d12core=n,b",
			}
		}
	}

	// 2. Проверяем наличие NVAPI
	hasNvapi := useNvapi()

	useGamemode := true // Можно тоже вынести в конфиг позже при желании

	// 3. Логика включения Gamescope
	enableGamescope := false

	// Включаем Gamescope ТОЛЬКО если это запуск ИГРЫ (явный флаг), включен Wine FSR и это НЕ CommunityShader
	if isGameLaunch && cfg.FSR && cfg.GrafikMod != "CommunityShader" {
		if _, err := exec.LookPath("gamescope"); err != nil {
			// Если бинарник не найден, отправляем сигнал на фронтенд для показа уведомления
			wailsRuntime.EventsEmit(ctx, "gamescope-missing")
			enableGamescope = false // Запускаем без него
		} else {
			// Gamescope найден, даем добро на его использование
			enableGamescope = true
		}
	}

	wineBin := filepath.Join(gameRoot, "wine", "proton", "files", "bin", "wine")
	exePath := filepath.Join(gameRoot, "MO2", "ModOrganizer.exe")
	prefixPath := filepath.Join(gameRoot, "wine", "prefix", "pfx")

	wineLibDir := filepath.Join(gameRoot, "wine", "proton", "files", "lib")
	wineLib64Dir := filepath.Join(gameRoot, "wine", "proton", "files", "lib64")
	wineBinDir := filepath.Join(gameRoot, "wine", "proton", "files", "bin")

	if _, err := os.Stat(wineBin); os.IsNotExist(err) {
		return fmt.Errorf("wine не найден в %s", wineBin)
	}
	if _, err := os.Stat(exePath); os.IsNotExist(err) {
		return fmt.Errorf("исполняемый файл не найден: %s", exePath)
	}

	// Формируем правильные пути для поиска библиотек
	wineDllPath := filepath.Join(wineLibDir, "wine") + ":" + filepath.Join(wineLib64Dir, "wine") + ":" + filepath.Dir(exePath)
	ldLibraryPath := wineLibDir + ":" + wineLib64Dir + ":" +
		filepath.Join(wineLibDir, "x86_64-linux-gnu") + ":" +
		filepath.Join(wineLibDir, "i386-linux-gnu") + ":" + os.Getenv("LD_LIBRARY_PATH")

	env := os.Environ()
	env = append(env,
		"GAME_ROOT="+gameRoot,
		"WINE_BIN="+wineBin,
		"EXE_PATH="+exePath,
		"PREFIX_PATH="+prefixPath,
		"MO2_ARGS="+mo2Args,
		"WINEDLLPATH="+wineDllPath,
		"LD_LIBRARY_PATH="+ldLibraryPath,
		"PATH="+wineBinDir+":"+os.Getenv("PATH"),

		// === ПЕРЕДАЕМ НАСТРОЙКИ ИЗ GO В BASH ===
		"WINEDLLOVERRIDES_PARAM="+cfg.WineDllOverrides,
		"ENABLE_NVAPI="+strconv.FormatBool(hasNvapi),
		"ENABLE_HDR="+strconv.FormatBool(cfg.HDR),
		"ENABLE_FSR="+strconv.FormatBool(cfg.FSR),
		"ENABLE_GAMESCOPE="+strconv.FormatBool(enableGamescope), // Передаем вычисленный флаг
		"ENABLE_MANGOHUD="+strconv.FormatBool(cfg.MangoHud),
		"ENABLE_SHADER_CACHE="+strconv.FormatBool(cfg.ShaderCache),
		"USE_GAMEMODE="+strconv.FormatBool(useGamemode),
		"STEAM_FIX_ENABLED="+strconv.FormatBool(cfg.SteamFix),

		"QT_OPENGL=software",
	)

	cmd := exec.Command("bash", "-c", scriptContent)
	cmd.Env = env

	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	go func() {
		err := cmd.Run()
		if err != nil {
			wailsRuntime.EventsEmit(ctx, "game-error", err.Error())
		} else {
			wailsRuntime.EventsEmit(ctx, "game-exit", nil)
		}
	}()

	return nil
}

func FirstInstall(ctx context.Context, gameRoot string, creds []byte, libs []byte, progressCb func(float64, string)) error {
	destDir := filepath.Join(gameRoot, "download")
	statusFile := filepath.Join(destDir, "install_status.txt")

	if err := os.MkdirAll(destDir, 0755); err != nil {
		return fmt.Errorf("не удалось создать папку download: %w", err)
	}

	statusF, err := os.OpenFile(statusFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("не удалось открыть файл статуса: %w", err)
	}
	defer statusF.Close()

	// Исправлено: теперь writeStatus корректно записывает ключ
	writeStatus := func(key string) {
		_, _ = statusF.WriteString(fmt.Sprintf("complete %s\n", key))
	}

	alreadyInstalled := func(key string) bool {
		// Убедитесь, что CheckInsatllStatus корректно объявлена в вашем проекте
		ok, _ := CheckInsatllStatus(destDir, key)
		return ok
	}

	// Чистый коллбэк для распаковки (принимает только проценты и сообщение)
	unpackCb := func(p float64, msg string) {
		if progressCb != nil {
			progressCb(p, "Распаковка: "+msg)
		}
	}

	if !alreadyInstalled("DisableGameFiles") {
		saveDisabledGameFiles(gameRoot)
	}

	if !alreadyInstalled("InstallDllOverrides") {
		err := PrepareEmbeddedLibs(gameRoot, libs, unpackCb)
		if err != nil {
			return fmt.Errorf("Не удалось подготовить встроенные библиотеки: %w", err)
		}
		writeStatus("InstallDllOverrides")
	}

	if !alreadyInstalled("InstallGEProton") {
		if err := proton_install.InstallGEProton(ctx, gameRoot, unpackCb); err != nil {
			return fmt.Errorf("Ошибка распаковки протона: %w", err)
		}
		writeStatus("InstallGEProton")
	}

	if !alreadyInstalled("UnpackPrefix") {
		if err := prefix_install.UnpackPrefix(ctx, gameRoot, unpackCb); err != nil {
			return fmt.Errorf("Ошибка распаковки префикса: %w", err)
		}
		writeStatus("UnpackPrefix")
	}

	if !alreadyInstalled("InstallUpdate") {
		if err := rfad_update.InstallUpdate(ctx, gameRoot, unpackCb); err != nil {
			return fmt.Errorf("Ошибка обновления игры: %w", err)
		}
		writeStatus("InstallUpdate")
	}

	if !alreadyInstalled("ApplyConfigPatches") {
		total, err := ApplyConfigPatches(ctx, gameRoot, unpackCb)
		if err != nil {
			return fmt.Errorf("Не удалось применить патчи конфигурации игры: %w", err)
		}
		slog.Info("применено патчей", "total", total)
		writeStatus("ApplyConfigPatches")
	}

	if !alreadyInstalled("RestoreDisabledSteamDrm") {
		err := steam_drm_switch.ToggleSteamDRM(ctx, gameRoot, false, unpackCb)
		if err != nil {
			return fmt.Errorf("Не удалось восстановить файлы запуска игры: %w", err)
		}
		slog.Info("Восстановление файлов запуска закончено")
		writeStatus("RestoreDisabledSteamDrm")
	}

	if !alreadyInstalled("InstallDrmSwitch") {
		err := unpacksteamfix.UnpackSteamFix(ctx, gameRoot, unpackCb)
		if err != nil {
			return fmt.Errorf("Не удалось установить переключатель Steam DRM: %w", err)
		}
		writeStatus("InstallDrmSwitch")
	}

	utils.SetOneSetting(gameRoot, "linux-patch-complite:", true)

	return nil
}

func FirstDownload(ctx context.Context, gameRoot string, creds []byte, offlineConfig []byte, progressCb func(float64, float64, string)) error {
	slog.Info("FirstDownload: gameRoot = " + gameRoot)

	cfg, err := GetLauncherConfig(gameRoot)
	if err != nil {
		slog.Warn("Не удалось прочитать конфиг для firstDownload, используем загрузку по умолчанию (GDrive)", "err", err)
		if cfg == nil {
			cfg = &LauncherConfig{CDN: false}
		}
	}

	downloadType := "gdrive"
	if cfg.CDN {
		downloadType = "cdn"
	}

	if err := downloader.DownloadUpdate(ctx, gameRoot, downloadType, creds, false, progressCb); err != nil {
		return err
	}

	if err := downloader.DownloadPrefix(ctx, gameRoot, downloadType, creds, false, progressCb); err != nil {
		return err
	}

	if err := downloader.DownloadSteamfix(ctx, gameRoot, downloadType, creds, false, progressCb); err != nil {
		return err
	}

	if cfg.CDN {
		if err := downloader.DownloadCommunityShaders(ctx, gameRoot, false, progressCb); err != nil {
			return err
		}
	}

	if err := downloader.DownloadGEProton(ctx, gameRoot, false, progressCb); err != nil {
		return err
	}

	if err := downloader.DownloadConfig(ctx, gameRoot, false); err != nil {
		slog.Warn("Не удалось загрузить конфигурацию из сети. Применяем встроенный offlineConfig", "err", err)

		configPath := filepath.Join(gameRoot, "download", "config.json")

		if writeErr := os.WriteFile(configPath, offlineConfig, 0644); writeErr != nil {
			slog.Error("Критическая ошибка: не удалось записать offlineConfig", "err", writeErr)
			return fmt.Errorf("ошибка сети (%v) и сбой записи резервного конфига: %w", err, writeErr)
		}

		slog.Info("Встроенная офлайн-конфигурация успешно применена")
	}

	return nil
}

func IsPathExist(ctx context.Context, gameRoot string) (bool, error) {
	filePath := filepath.Join(gameRoot, "MO2", "ModOrganizer.exe")
	_, err := os.Stat(filePath)
	if err == nil {
		return true, nil
	}
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	return false, err
}

func CheckDownloadStatus(destDir, keyword string) (bool, error) {
	statusFile := filepath.Join(destDir, "download_status.txt")
	f, err := os.Open(statusFile)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, err
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.Contains(line, keyword) {
			return true, nil
		}
	}
	return false, scanner.Err()
}

func CheckInsatllStatus(destDir, keyword string) (bool, error) {
	statusFile := filepath.Join(destDir, "install_status.txt")
	f, err := os.Open(statusFile)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, err
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.Contains(line, keyword) {
			return true, nil
		}
	}
	return false, scanner.Err()
}

func ApplyConfigPatches(ctx context.Context, gameRoot string, progressCb func(float64, string)) (int, error) {
	destDir := filepath.Join(gameRoot, "download")
	configPath, err := utils.GetDownloadedPath(destDir, "config")
	if err != nil {
		return 0, fmt.Errorf("не удалось найти путь к конфигу патчей: %w", err)
	}
	slog.Info("update path: %s", configPath)
	if configPath == "" {
		return 0, fmt.Errorf("конфиг патчей не найден в статусе загрузки")
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		return 0, fmt.Errorf("не удалось прочитать конфиг патчей: %w", err)
	}

	var patches []config_patcher.ConfigPatch
	if err := json.Unmarshal(data, &patches); err != nil {
		return 0, fmt.Errorf("ошибка парсинга конфига патчей: %w", err)
	}

	slog.Debug("загружено патчей из конфига", "count", len(patches))
	return config_patcher.ApplyPatchesFromJSON(gameRoot, patches, progressCb)
}

func InstallGame(ctx context.Context, installerPath, installPath string, cacheDir string, innoextractBytes []byte, progressCb func(float64, string)) error {
	installerPath = strings.Trim(installerPath, "\"' ")
	installPath = strings.Trim(installPath, "\"' ")

	slog.Info("InstallGame: paths", "installerPath", installerPath, "installPath", installPath)

	if installerPath == "" || installPath == "" {
		return fmt.Errorf("путь к установщику или папке установки пустой")
	}

	if _, err := os.Stat(installerPath); os.IsNotExist(err) {
		return fmt.Errorf("установщик не найден: %s", installerPath)
	}

	if err := os.MkdirAll(installPath, 0755); err != nil {
		return fmt.Errorf("не удалось создать папку назначения: %v", err)
	}

	// Определяем, какой innoextract использовать: сначала системный, потом вшитый
	innoBinPath := ""
	sysInno, err := exec.LookPath("innoextract")
	if err == nil {
		// Проверяем системный
		testCmd := exec.Command(sysInno, "--version")
		testOut, testErr := testCmd.CombinedOutput()
		if testErr == nil && strings.Contains(string(testOut), "innoextract") {
			innoBinPath = sysInno
			slog.Info("Using system innoextract", "path", innoBinPath)
		} else {
			slog.Warn("System innoextract not working", "error", testErr, "output", string(testOut))
		}
	}

	// Если системный не работает, используем вшитый
	if innoBinPath == "" {
		if err := os.MkdirAll(cacheDir, 0755); err != nil {
			return fmt.Errorf("не удалось создать временную папку: %v", err)
		}

		innoBinPath = filepath.Join(cacheDir, "innoextract")
		if err := os.WriteFile(innoBinPath, innoextractBytes, 0755); err != nil {
			return fmt.Errorf("не удалось сохранить innoextract: %v", err)
		}

		// Проверяем права
		if info, err := os.Stat(innoBinPath); err == nil {
			if info.Mode()&0111 == 0 {
				slog.Warn("innoextract not executable, setting permissions")
				if err := os.Chmod(innoBinPath, 0755); err != nil {
					slog.Warn("Failed to chmod innoextract", "error", err)
				}
			}
		}

		// Проверяем, что это бинарник (ELF на Linux)
		if runtime.GOOS == "linux" {
			if data, err := os.ReadFile(innoBinPath); err == nil && len(data) > 4 {
				if string(data[1:4]) == "ELF" {
					slog.Info("innoextract is ELF file")
				} else {
					slog.Warn("innoextract is not ELF, might be corrupted")
				}
			}
		}

		// Проверяем вшитый
		fullCmd := fmt.Sprintf("%s --version", innoBinPath)
		slog.Info("Testing embedded innoextract", "command", fullCmd)
		testCmd := exec.Command(innoBinPath, "--version")
		testOut, testErr := testCmd.CombinedOutput()
		if testErr != nil || !strings.Contains(string(testOut), "innoextract") {
			return fmt.Errorf("встроенный innoextract не работает: %v, output: %s", testErr, string(testOut))
		}
		slog.Info("Using embedded innoextract", "path", innoBinPath)
	}

	// Запускаем основную команду через bash -c, чтобы точно воспроизвести ручной запуск
	cmdStr := fmt.Sprintf("%s -d %q %q", innoBinPath, installPath, installerPath)
	slog.Info("Running innoextract", "full_command", cmdStr)
	cmd := exec.Command("bash", "-c", cmdStr)
	cmd.Dir = filepath.Dir(installerPath)

	var stdoutBuf, stderrBuf bytes.Buffer
	cmd.Stdout = &stdoutBuf
	cmd.Stderr = &stderrBuf

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("innoextract не запущен: %v", err)
	}

	done := make(chan error, 1)
	go func() {
		done <- cmd.Wait()
	}()

	// Ожидаемый размер (заглушка)
	expectedSize := int64(77737979510)
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

Loop:
	for {
		select {
		case err := <-done:
			if err != nil {
				return fmt.Errorf("innoextract failed: %v\nSTDOUT:\n%s\nSTDERR:\n%s",
					err, stdoutBuf.String(), stderrBuf.String())
			}
			break Loop
		case <-ticker.C:
			if progressCb != nil {
				currentSize, _ := DirSize(installPath)
				percent := float64(currentSize) / float64(expectedSize)
				if percent > 0.99 {
					percent = 0.99
				}
				gbCurrent := float64(currentSize) / (1024 * 1024 * 1024)
				gbTotal := float64(expectedSize) / (1024 * 1024 * 1024)
				msg := fmt.Sprintf("Распаковка: %.1f ГБ / %.1f ГБ", gbCurrent, gbTotal)
				progressCb(percent, msg)
			}
		}
	}

	// Перенос файлов из app/
	appDir := filepath.Join(installPath, "app")
	entries, err := os.ReadDir(appDir)
	if err == nil {
		for _, entry := range entries {
			oldPath := filepath.Join(appDir, entry.Name())
			newPath := filepath.Join(installPath, entry.Name())
			_ = os.Rename(oldPath, newPath)
		}
	}
	_ = os.RemoveAll(appDir)
	_ = os.RemoveAll(filepath.Join(installPath, "tmp"))

	utils.SetOneSetting(installPath, "linux-patch-complite", "false")
	utils.SetOneSetting(installPath, "MangoHud:", "false")
	utils.SetOneSetting(installPath, "FSR:", "false")
	utils.SetOneSetting(installPath, "ShaderCache:", "true")
	utils.SetOneSetting(installPath, "HDR:", "false")
	utils.SetOneSetting(installPath, "SteamFix:", "false")
	utils.SetOneSetting(installPath, "FpsLimit:", " ")
	utils.SetOneSetting(installPath, "CDN:", "false")
	utils.SetOneSetting(installPath, "WineDllOverrides:", "concrt140=n;xaudio2_7=n,b;d3d11=n,b;dxgi=n,b;d3dx9_42=n,b;d3dcompiler_47=n,b;dinput8=n,b;mscoree=n;d3d12=n,b;d3d12core=n,b")
	utils.SetOneSetting(installPath, "GrafikMod:", "Нету")
	utils.SetOneSetting(installPath, "FsrLvl:", "95")

	if progressCb != nil {
		progressCb(1.0, "Установка завершена!")
	}

	return nil
}

func hashFile(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return fmt.Sprintf("%x", h.Sum(nil)), nil
}

// copySingleFile аккуратно копирует один файл с принудительным сбросом буферов
func copySingleFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()

	if _, err := io.Copy(out, in); err != nil {
		return err
	}
	return out.Sync() // Принудительно записываем данные на диск
}

// safeCopyAndVerify копирует файл или директорию целиком и сверяет хэши каждого файла
func safeCopyAndVerify(src, dst string) error {
	info, err := os.Stat(src)
	if err != nil {
		return err
	}

	// Если это директория (например, enbseries) - рекурсивно обходим её
	if info.IsDir() {
		return filepath.Walk(src, func(path string, fInfo os.FileInfo, err error) error {
			if err != nil {
				return err
			}

			relPath, err := filepath.Rel(src, path)
			if err != nil {
				return err
			}
			targetPath := filepath.Join(dst, relPath)

			if fInfo.IsDir() {
				return os.MkdirAll(targetPath, 0755)
			}

			// Копируем файл внутри директории
			if err := copySingleFile(path, targetPath); err != nil {
				return fmt.Errorf("ошибка копирования %s: %w", path, err)
			}

			// Сверяем хэши
			hSrc, err := hashFile(path)
			if err != nil {
				return fmt.Errorf("ошибка хэширования исходника %s: %w", path, err)
			}
			hDst, err := hashFile(targetPath)
			if err != nil {
				return fmt.Errorf("ошибка хэширования копии %s: %w", targetPath, err)
			}

			if hSrc != hDst {
				return fmt.Errorf("хэши не совпадают для файла %s", path)
			}

			return nil
		})
	}

	// Если это одиночный файл
	if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
		return err
	}
	if err := copySingleFile(src, dst); err != nil {
		return fmt.Errorf("ошибка копирования %s: %w", src, err)
	}

	// Сверка хэшей для одиночного файла
	hSrc, err := hashFile(src)
	if err != nil {
		return fmt.Errorf("ошибка хэширования %s: %w", src, err)
	}
	hDst, err := hashFile(dst)
	if err != nil {
		return fmt.Errorf("ошибка хэширования копии %s: %w", dst, err)
	}
	if hSrc != hDst {
		return fmt.Errorf("хэши не совпадают для %s", src)
	}

	return nil
}

func saveDisabledGameFiles(installPath string) {
	enbFiles := []string{
		"d3d11.dll",
		"enblocal.ini",
		"enbseries",
		"_weatherlist.ini",
		"_locationweather.ini",
	}

	reshadeFiles := []string{
		"dxgi.dll",
		"ReShade.ini",
		"reshade-shaders",
	}

	drmFiles := []string{
		"SkyrimSE.exe",
		"steam_api64.dll",
		"steam_api64.cdx",
	}

	moveItem := func(itemName, typeFiles, target string) {
		src := filepath.Join(installPath, itemName)
		targetDir := filepath.Join(installPath, "disabledGameFiles", typeFiles, target)
		dst := filepath.Join(targetDir, itemName)

		// Проверяем, существует ли исходный файл или папка
		if _, err := os.Stat(src); err == nil {
			slog.Info("Начало резервного копирования", "target", itemName, "to", dst)

			// Удаляем старые остатки в папке назначения, чтобы избежать конфликтов
			os.RemoveAll(dst)

			// Шаг 1: Копируем и проверяем целостность хэшей
			if err := safeCopyAndVerify(src, dst); err != nil {
				slog.Error("КРИТИЧЕСКАЯ ОШИБКА: Сбой при копировании/верификации. Оригинал НЕ удален",
					"item", itemName,
					"err", err,
				)
				return // Прерываем операцию, не трогаем оригинал
			}

			// Шаг 2: Если дошли сюда, копии идентичны. Можно безопасно удалять оригинал
			slog.Info("Верификация пройдена успешно. Удаляем оригинал", "item", itemName)
			if err := os.RemoveAll(src); err != nil {
				slog.Warn("Файлы скопированы, но оригинал не удалось удалить",
					"item", itemName,
					"err", err,
				)
			} else {
				slog.Info("Файл успешно отключен", "item", itemName, "category", typeFiles)
			}
		}
	}

	// Выполняем отключение модов
	for _, f := range enbFiles {
		moveItem(f, "GraphicFiles", "ENB")
	}

	for _, f := range reshadeFiles {
		moveItem(f, "GraphicFiles", "ReShade")
	}

	for _, f := range drmFiles {
		moveItem(f, "SteamDRM", "off")
	}
}

// DirSize рекурсивно вычисляет размер директории в байтах.
func DirSize(path string) (int64, error) {
	var size int64
	err := filepath.Walk(path, func(_ string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() {
			size += info.Size()
		}
		return nil
	})
	return size, err
}

func PrepareEmbeddedLibs(gameRoot string, libsArchive []byte, progressCb func(float64, string)) error {
	if len(libsArchive) == 0 {
		return fmt.Errorf("массив библиотек пуст, пропускаем")
	}

	slog.Info("Извлечение переданных библиотек (libs.zip)")
	if progressCb != nil {
		progressCb(0.1, "Извлечение встроенных библиотек DXVK...")
	}

	downloadDir := filepath.Join(gameRoot, "download")
	if err := os.MkdirAll(downloadDir, 0755); err != nil {
		return fmt.Errorf("не удалось создать папку download: %w", err)
	}

	libsZipPath := filepath.Join(downloadDir, "libs.zip")

	// 1. Записываем переданные байты на диск
	if err := os.WriteFile(libsZipPath, libsArchive, 0644); err != nil {
		return fmt.Errorf("ошибка записи архива libs.zip: %w", err)
	}

	if progressCb != nil {
		progressCb(0.5, "Регистрация библиотек в системе...")
	}

	// 2. Обновляем download_status.txt
	statusFilePath := filepath.Join(downloadDir, "download_status.txt")
	var lines []string
	if content, err := os.ReadFile(statusFilePath); err == nil {
		lines = strings.Split(strings.TrimSpace(string(content)), "\n")
	}

	var newLines []string
	for _, line := range lines {
		if line == "" || strings.Contains(line, " libs ") {
			continue
		}
		newLines = append(newLines, line)
	}

	statusLine := fmt.Sprintf("complete libs %s", libsZipPath)
	newLines = append(newLines, statusLine)

	finalContent := strings.Join(newLines, "\n") + "\n"
	if err := os.WriteFile(statusFilePath, []byte(finalContent), 0644); err != nil {
		return fmt.Errorf("ошибка обновления download_status.txt: %w", err)
	}

	slog.Info("Библиотеки успешно подготовлены", "path", libsZipPath)
	if progressCb != nil {
		progressCb(1.0, "Встроенные библиотеки готовы")
	}

	return nil
}
