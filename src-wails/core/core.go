package core

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"rfad-launcher-linux/src-wails/downloader"
	config_patcher "rfad-launcher-linux/src-wails/patches/patch_configs"
	"rfad-launcher-linux/src-wails/patches/prefix_install"
	"rfad-launcher-linux/src-wails/patches/proton_install"
	"rfad-launcher-linux/src-wails/patches/rfad_update"
	"rfad-launcher-linux/src-wails/utils"
	"rfad-launcher-linux/src-wails/utils/steam_drm_switch"
	"runtime"
	"strconv"
	"strings"
	"time"

	wailsRuntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

const (
	UpdateFolderID   string = "1JUOctbsugh2IIEUCWcBkupXYVYoJMg4G"
	YandexPrefixURL  string = "https://disk.yandex.ru/d/y3mx1DXn83CbgQ"
	SteamFixID       string = "17BUNJj1akU-ktCOK7iDVErFMZJK6_sfQ"
	PrefixFileCDNURL string = "https://mirror.kraito.ru/rfad/prefix/prefix.tar.gz"
	SteamFixCDNURL   string = "https://mirror.kraito.ru/rfad/SteamFix/SteamFix.zip"
	// InnoExtract      string = "https://github.com/dscharrer/innoextract/releases/download/1.9/innoextract-1.9-linux.tar.xz" unused
	GEProtonUrl    string = "https://github.com/GloriousEggroll/proton-ge-custom/releases/download/GE-Proton11-5/GE-Proton11-5-x86_64.tar.gz"
	ConfigPatchURL string = "https://api.kraito.ru/api/v1/config"
)

func StartMO2(ctx context.Context, gameRoot string, scriptContent, mo2Args string) error {
	useGamemode := true

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
		"USE_GAMEMODE="+strconv.FormatBool(useGamemode),
		"WINEDLLPATH="+wineDllPath,
		"LD_LIBRARY_PATH="+ldLibraryPath,
		"PATH="+wineBinDir+":"+os.Getenv("PATH"),

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

func FirstInstall(ctx context.Context, source string, gameRoot string, creds []byte, progressCb func(float64, string)) error {
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

	writeStatus := func(key string) {
		_, _ = statusF.WriteString(fmt.Sprintf("complete %s %s\n", key))
	}

	alreadyInstalled := func(key string) bool {
		ok, _ := CheckInsatllStatus(destDir, key)
		return ok
	}

	if err := FirstDownload(ctx, source, gameRoot, creds, progressCb); err != nil {
		return fmt.Errorf("Ошибка загрузки файлов: %w", err)
	}

	if !alreadyInstalled("InstallGEProton") {
		if err := proton_install.InstallGEProton(ctx, gameRoot); err != nil {
			return fmt.Errorf("Ошибка распаковки протона: %w", err)
		}
		writeStatus("InstallGEProton")
	}

	if !alreadyInstalled("UnpackPrefix") {
		if err := prefix_install.UnpackPrefix(ctx, gameRoot); err != nil {
			return fmt.Errorf("Ошибка распаковки префикса: %w", err)
		}
		writeStatus("UnpackPrefix")
	}

	if !alreadyInstalled("InstallUpdate") {
		if err := rfad_update.InstallUpdate(ctx, gameRoot); err != nil {
			return fmt.Errorf("Ошибка обновления игры: %w", err)
		}
		writeStatus("InstallUpdate")
	}

	if !alreadyInstalled("ApplyConfigPatches") {
		total, err := ApplyConfigPatches(ctx, gameRoot, progressCb)
		if err != nil {
			return fmt.Errorf("Не удалось применить патчи конфигурации игры: %w", err)
		}
		slog.Info("применено патчей", "total", total)
		writeStatus("ApplyConfigPatches")
	}

	if !alreadyInstalled("RestoreDisabledSteamDrm") {
		err := steam_drm_switch.ToggleSteamDRM(ctx, gameRoot, false, progressCb)
		if err != nil {
			return fmt.Errorf("Не удалось востоновить файлы запуска игры: %w", err)
		}
		slog.Info("Востоновления файлов запуска закончено")
		writeStatus("RestoreDisabledSteamDrm")
	}

	configFile := filepath.Join(gameRoot, "launcher_config.txt")
	configF, err := os.OpenFile(configFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		slog.Warn("не удалось открыть файл статуса: %w", err)
	}
	defer configF.Close()

	_, _ = configF.WriteString("linux-patch-complite: true/n")

	return nil
}

func FirstDownload(ctx context.Context, source string, gameRoot string, creds []byte, progressCb func(float64, string)) error {
	slog.Info("FirstDownload: gameRoot = " + gameRoot)
	type ConfigData struct {
		Data []json.RawMessage `json:"data"`
	}

	type ConfigResponse struct {
		Success bool       `json:"success"`
		Data    ConfigData `json:"data"`
	}

	downloadCb := func(p float64) {
		progressCb(p, "Загрузка файлов...")
	}

	destDir := filepath.Join(gameRoot, "download")
	if err := os.MkdirAll(destDir, 0755); err != nil {
		return fmt.Errorf("не удалось создать папку download: %w", err)
	}
	statusFile := filepath.Join(destDir, "download_status.txt")
	configFile := filepath.Join(destDir, "config.json")

	statusF, err := os.OpenFile(statusFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("не удалось открыть файл статуса: %w", err)
	}
	defer statusF.Close()

	writeStatus := func(key, value string) {
		_, _ = statusF.WriteString(fmt.Sprintf("complete %s %s\n", key, value))
	}

	alreadyDownloaded := func(key string) bool {
		ok, _ := CheckDownloadStatus(destDir, key)
		return ok
	}

	switch source {
	case "cdn":
		var path string

		if !alreadyDownloaded("update") {
			url, err := LastUpdateFromCDN(ctx)
			if err != nil {
				return fmt.Errorf("ошибка получения обновления попробуйте использовать google drive %s", err)
			}

			path, err = downloader.DownloadURL(ctx, url, destDir, downloadCb)
			if err != nil {
				return fmt.Errorf("ошибка получения обновления попробуйте использовать google drive %s", err)
			}
			writeStatus("update", path)
		}

		if !alreadyDownloaded("prefix") {
			path, err = downloader.DownloadURL(ctx, PrefixFileCDNURL, destDir, downloadCb)
			if err != nil {
				return fmt.Errorf("ошибка получения префикса wine попробуйте использовать google drive %s", err)
			}
			writeStatus("prefix", path)
		}

		if !alreadyDownloaded("steamfix") {
			path, err = downloader.DownloadURL(ctx, SteamFixCDNURL, destDir, downloadCb)
			if err != nil {
				return fmt.Errorf("ошибка получения префикса wine попробуйте использовать google drive %s", err)
			}
			writeStatus("steamfix", path)
		}

	case "gdrive":
		var paths []string

		if !alreadyDownloaded("update") {
			paths, err = downloader.DownloadDriveFolder(ctx, creds, UpdateFolderID, destDir, downloadCb)
			if err != nil {
				return fmt.Errorf("ошибка получения обновления попробуйте использовать cdn(mirror) %s", err)
			}
			for _, p := range paths {
				writeStatus("update", p)
			}
		}

		if !alreadyDownloaded("prefix") {
			path, err := downloader.DownloadYandex(ctx, YandexPrefixURL, destDir, "pfx dotnet.7z", downloadCb)
			if err != nil {
				return fmt.Errorf("ошибка загрузки префикса с Яндекс.Диска: %w", err)
			}
			writeStatus("prefix", path)
		}

		if !alreadyDownloaded("steamfix") {
			paths, err = downloader.DownloadDriveFolder(ctx, creds, SteamFixID, destDir, downloadCb)
			if err != nil {
				return fmt.Errorf("ошибка получения префикса wine попробуйте использовать cdn(mirror) %s", err)
			}
			for _, p := range paths {
				writeStatus("steamfix", p)
			}
		}

	default:
		return fmt.Errorf("неизвестный источник: %s", source)
	}
	var path string

	if !alreadyDownloaded("GE-Proton") {
		path, err = downloader.DownloadURL(ctx, GEProtonUrl, destDir, downloadCb)
		if err != nil {
			return fmt.Errorf("Ошибка загрузки GE-Proton повторите попытку позже %s", err)
		}
		writeStatus("GE-Proton", path)
	}

	if !alreadyDownloaded("config") {
		req, err := http.NewRequestWithContext(ctx, "GET", ConfigPatchURL, nil)
		if err != nil {
			return err
		}

		client := &http.Client{Timeout: 10 * time.Second}
		resp, err := client.Do(req)
		if err != nil {
			return fmt.Errorf("ошибка соединения с сервером: %w", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			return fmt.Errorf("сервер вернул статус: %d", resp.StatusCode)
		}

		var apiResult ConfigResponse
		if err := json.NewDecoder(resp.Body).Decode(&apiResult); err != nil {
			return fmt.Errorf("ошибка чтения ответа API: %w", err)
		}

		if !apiResult.Success {
			return fmt.Errorf("API вернул success: false")
		}

		cfgFile, err := os.Create(configFile)
		jsonData, err := json.MarshalIndent(apiResult.Data.Data, "", "    ")
		if err != nil {
			return fmt.Errorf("неудалось пропарсить конфиг с сервера: %w", err)
		}
		_, err = cfgFile.Write(jsonData)
		writeStatus("config", configFile)
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

func LastUpdateFromCDN(ctx context.Context) (string, error) {
	type UpdateData struct {
		ID        string `json:"id"`
		Version   string `json:"version"`
		URL       string `json:"url"`
		CreatedAt int64  `json:"created_at"`
	}

	type UpdateResponse struct {
		Success bool       `json:"success"`
		Data    UpdateData `json:"data"`
	}

	apiUrl := "https://api.kraito.ru/api/v1/updates/latest"

	req, err := http.NewRequestWithContext(ctx, "GET", apiUrl, nil)
	if err != nil {
		return "", err
	}

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("ошибка соединения с сервером: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("сервер вернул статус: %d", resp.StatusCode)
	}

	var apiResult UpdateResponse
	if err := json.NewDecoder(resp.Body).Decode(&apiResult); err != nil {
		return "", fmt.Errorf("ошибка чтения ответа API: %w", err)
	}

	if !apiResult.Success {
		return "", fmt.Errorf("API вернул success: false")
	}
	s3BaseURL := "https://mirror.kraito.ru/"
	return s3BaseURL + apiResult.Data.URL, nil
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

	saveDisabledGameFiles(installPath)

	if progressCb != nil {
		progressCb(1.0, "Установка завершена!")
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

	// Функция для аккуратного перемещения файлов и папок в бэкап
	moveItem := func(itemName, typeFiles, target string) {
		src := filepath.Join(installPath, itemName)
		targetDir := filepath.Join(installPath, "disabledGameFiles", typeFiles, target)
		dst := filepath.Join(targetDir, itemName)

		// Проверяем, существует ли исходный файл или папка
		if _, err := os.Stat(src); err == nil {
			// Создаем всю структуру папок (например: disabledGameFiles/SteamDRM/off)
			os.MkdirAll(targetDir, 0755)

			// Удаляем старый файл в папке назначения, чтобы избежать ошибки перезаписи
			os.RemoveAll(dst)

			// Перемещаем
			if err := os.Rename(src, dst); err == nil {
				fmt.Printf("Перемещен [%s/%s]: %s\n", typeFiles, target, itemName)
			} else {
				fmt.Printf("Ошибка перемещения [%s/%s] %s: %v\n", typeFiles, target, itemName, err)
			}
		}
	}

	// Перемещаем файлы ENB
	for _, f := range enbFiles {
		moveItem(f, "GraphicFiles", "ENB")
	}

	// Перемещаем файлы ReShade
	for _, f := range reshadeFiles {
		moveItem(f, "GraphicFiles", "ReShade")
	}

	// Перемещаем DRM файлы Steam
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
