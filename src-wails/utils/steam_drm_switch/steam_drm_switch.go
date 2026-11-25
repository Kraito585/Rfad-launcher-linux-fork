package steam_drm_switch

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"rfad-launcher-linux/src-wails/utils"
)

// isSteamInstalled проверяет, установлен ли нативный или Flatpak-клиент Steam.
func isSteamInstalled() bool {
	if utils.HostCommandExists("steam") {
		return true
	}

	homeDir, err := os.UserHomeDir()
	if err != nil {
		return false
	}

	flatpakPaths := []string{
		"/var/lib/flatpak/app/com.valvesoftware.Steam",
		filepath.Join(homeDir, ".local", "share", "flatpak", "app", "com.valvesoftware.Steam"),
	}
	for _, p := range flatpakPaths {
		if info, err := os.Stat(p); err == nil && info.IsDir() {
			return true
		}
	}

	return false
}

// findSteamLegacyCompatDir ищет каталог legacycompat с реальным steamclient64.dll.
func findSteamLegacyCompatDir() (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("не удалось определить домашнюю директорию: %w", err)
	}

	candidates := []string{
		filepath.Join(homeDir, ".local", "share", "Steam"),
		filepath.Join(homeDir, ".steam", "steam"),
		filepath.Join(homeDir, ".steam", "root"),
		filepath.Join(homeDir, ".var", "app", "com.valvesoftware.Steam", ".local", "share", "Steam"),
	}

	for _, base := range candidates {
		legacyDir := filepath.Join(base, "legacycompat")
		if info, err := os.Stat(filepath.Join(legacyDir, "steamclient64.dll")); err == nil && !info.IsDir() {
			return legacyDir, nil
		}
	}

	return "", fmt.Errorf("не найдены файлы клиента Steam (legacycompat/steamclient64.dll). Запустите Steam хотя бы один раз для загрузки его компонентов и повторите попытку")
}

// EnsureSteamBridge проверяет наличие Steam, удаляет битые симлинки в префиксе
// и копирует реальные библиотеки моста Steam в префикс игры.
func EnsureSteamBridge(gameRoot string) error {
	if !isSteamInstalled() {
		return fmt.Errorf("Steam не установлен на устройстве. Установите Steam, запустите его хотя бы один раз и повторите попытку")
	}

	legacyDir, err := findSteamLegacyCompatDir()
	if err != nil {
		return err
	}

	pfxDir := filepath.Join(gameRoot, "wine", "prefix", "pfx")
	wineLibDir := filepath.Join(gameRoot, "wine", "lib", "wine")

	// 1. Гарантируем наличие lsteamclient.dll и steam.exe в system32 / syswow64
	bridgePairs := [][2]string{
		{filepath.Join(wineLibDir, "x86_64-windows", "lsteamclient.dll"), filepath.Join(pfxDir, "drive_c", "windows", "system32", "lsteamclient.dll")},
		{filepath.Join(wineLibDir, "x86_64-windows", "steam.exe"), filepath.Join(pfxDir, "drive_c", "windows", "system32", "steam.exe")},
		{filepath.Join(wineLibDir, "i386-windows", "lsteamclient.dll"), filepath.Join(pfxDir, "drive_c", "windows", "syswow64", "lsteamclient.dll")},
		{filepath.Join(wineLibDir, "i386-windows", "steam.exe"), filepath.Join(pfxDir, "drive_c", "windows", "syswow64", "steam.exe")},
	}

	for _, p := range bridgePairs {
		if _, err := os.Stat(p[0]); err != nil {
			continue
		}
		if err := os.MkdirAll(filepath.Dir(p[1]), 0755); err != nil {
			return fmt.Errorf("не удалось создать каталог %s: %w", filepath.Dir(p[1]), err)
		}
		_ = os.Remove(p[1])
		if err := utils.CopyFile(p[0], p[1]); err != nil {
			return fmt.Errorf("не удалось скопировать %s: %w", filepath.Base(p[0]), err)
		}
	}

	// 2. Удаляем битые симлинки и копируем реальные DLL клиента Steam из legacycompat
	dstSteamDir := filepath.Join(pfxDir, "drive_c", "Program Files (x86)", "Steam")
	if err := os.MkdirAll(dstSteamDir, 0755); err != nil {
		return fmt.Errorf("не удалось создать папку Steam в префиксе: %w", err)
	}

	steamDlls := []string{
		"steamclient64.dll",
		"steamclient.dll",
		"GameOverlayRenderer64.dll",
		"GameOverlayRenderer.dll",
	}

	for _, dll := range steamDlls {
		src := filepath.Join(legacyDir, dll)
		dst := filepath.Join(dstSteamDir, dll)

		// Обязательно удаляем старый файл/симлинк, чтобы CopyFile не писал по цели симлинка
		_ = os.Remove(dst)

		if _, err := os.Stat(src); err != nil {
			if dll == "steamclient64.dll" {
				return fmt.Errorf("отсутствует обязательный файл %s", src)
			}
			continue
		}

		if err := utils.CopyFile(src, dst); err != nil {
			return fmt.Errorf("ошибка копирования %s в префикс: %w", dll, err)
		}
	}

	slog.Info("Компоненты моста Steam успешно установлены в префикс", "legacyDir", legacyDir)
	return nil
}

func ToggleSteamDRM(gameRoot string, enable bool, unpackCb func(float64, string)) error {
	drmOnFiles := []string{"SkyrimSE.exe", "steam_api64.dll"}
	drmOffFiles := []string{"SkyrimSE.exe", "steam_api64.dll", "steam_api64"}

	steamDrmDir := filepath.Join(gameRoot, "disabledGameFiles", "SteamDRM")
	dirOn := filepath.Join(steamDrmDir, "on")
	dirOff := filepath.Join(steamDrmDir, "off")

	// 0. При включении DRM сначала проверяем Steam и лечим префикс ДО удаления файлов игры
	if enable {
		if unpackCb != nil {
			unpackCb(0.0, "Проверка Steam и подготовка префикса...")
		}
		if err := EnsureSteamBridge(gameRoot); err != nil {
			slog.Warn("Не удалось включить Steam DRM", "err", err)
			return err
		}
	}

	// 1. Очищаем корень игры от возможных старых файлов
	allFiles := []string{"SkyrimSE.exe", "steam_api64.dll", "steam_api64"}
	for _, f := range allFiles {
		os.RemoveAll(filepath.Join(gameRoot, f))
	}

	// 2. Выбираем источник для копирования
	var sourceDir string
	var filesToCopy []string

	if enable {
		if unpackCb != nil {
			unpackCb(0.2, "Включение Steam DRM...")
		}
		sourceDir = dirOn
		filesToCopy = drmOnFiles
	} else {
		if unpackCb != nil {
			unpackCb(0.0, "Отключение Steam DRM...")
		}
		sourceDir = dirOff
		filesToCopy = drmOffFiles
	}

	// 3. Копируем нужные файлы из хранилища в корень игры
	totalOperations := len(filesToCopy)

	for i, f := range filesToCopy {
		src := filepath.Join(sourceDir, f)
		dst := filepath.Join(gameRoot, f)
		percent := float64(i+1) / float64(totalOperations)

		if _, err := os.Stat(src); os.IsNotExist(err) {
			if unpackCb != nil {
				unpackCb(percent, fmt.Sprintf("Пропущен файл (не найден): %s", f))
			}
			continue
		}

		if err := utils.CopyPath(src, dst); err == nil {
			if unpackCb != nil {
				unpackCb(percent, fmt.Sprintf("Скопирован: %s", f))
			}
		} else {
			if unpackCb != nil {
				unpackCb(percent, fmt.Sprintf("Ошибка копирования: %s", f))
			}
			slog.Error("ошибка копирования DRM файла", "src", src, "err", err)
		}
	}

	// 4. Финальный вызов прогресса и запись в конфиг
	if unpackCb != nil {
		unpackCb(1.0, "Обновление конфигурации...")
	}

	valStr := "false"
	if enable {
		valStr = "true"
	}

	return utils.SetOneSetting("SteamFix", valStr)
}
