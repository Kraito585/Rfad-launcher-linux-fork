package unpacksteamfix

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"rfad-launcher-linux/src-wails/utils"
	"strings"
)

func UnpackSteamFix(ctx context.Context, gameRoot string, unpackCb func(float64, string)) error {
	destDir := filepath.Join(gameRoot, "download")
	steamFixPath, err := utils.GetDownloadedPath(destDir, "steamfix")
	if err != nil || steamFixPath == "" {
		return fmt.Errorf("архив SteamFix не найден: %w", err)
	}
	slog.Info("steamfix archive path:", "path", steamFixPath)

	tempDir := filepath.Join(gameRoot, "temp_steamfix_extract")
	os.MkdirAll(tempDir, 0755)
	defer os.RemoveAll(tempDir) // Гарантированно удалит мусор после завершения

	nestedTempDir := filepath.Join(gameRoot, "temp_steamfix_nested")
	defer os.RemoveAll(nestedTempDir)

	if err := utils.ExtractArchive(ctx, steamFixPath, tempDir, func(p float64, msg string) {
		if unpackCb != nil {
			unpackCb(p*50, fmt.Sprintf("SteamFix: %s", msg)) // Выделяем 50% на первый этап
		}
	}); err != nil {
		return fmt.Errorf("ошибка распаковки SteamFix: %w", err)
	}

	// --- 1. ГЛУБОКИЙ ПОИСК ВЛОЖЕННОГО АРХИВА ---
	var nestedArchivePath string
	filepath.Walk(tempDir, func(path string, info os.FileInfo, err error) error {
		if err == nil && !info.IsDir() {
			ext := strings.ToLower(filepath.Ext(info.Name()))
			if ext == ".zip" || ext == ".7z" || ext == ".rar" {
				nestedArchivePath = path
				return fmt.Errorf("found_archive") // Искусственно прерываем поиск
			}
		}
		return nil
	})

	if nestedArchivePath != "" {
		slog.Info("Обнаружен вложенный архив SteamFix. Распаковываем...", "file", nestedArchivePath)
		os.MkdirAll(nestedTempDir, 0755)
		err := utils.ExtractArchive(ctx, nestedArchivePath, nestedTempDir, func(p float64, msg string) {
			if unpackCb != nil {
				unpackCb(50+(p*50), fmt.Sprintf("Вложенный архив: %s", msg))
			}
		})
		if err == nil {
			tempDir = nestedTempDir // Переключаемся на папку с извлеченным вложенным архивом
		} else {
			slog.Warn("Не удалось распаковать вложенный архив, продолжаем с исходными", "err", err)
		}
	}

	// --- 2. УМНЫЙ ПОИСК ПАПКИ С ФАЙЛАМИ СКАЙРИМА ---
	sourceDir := tempDir
	filepath.Walk(tempDir, func(path string, info os.FileInfo, err error) error {
		if err == nil && !info.IsDir() {
			name := strings.ToLower(info.Name())
			// Ищем главные файлы кряка/фикса
			if name == "steam_api64.dll" || name == "skyrimse.exe" {
				sourceDir = filepath.Dir(path) // Берем родительскую папку найденного файла
				slog.Info("Найдена целевая папка SteamFix", "dir", sourceDir)
				return fmt.Errorf("found_payload") // Прерываем поиск
			}
		}
		return nil
	})

	// --- 3. ПЕРЕМЕЩЕНИЕ В ЦЕЛЕВУЮ ПАПКУ ---
	targetParentDir := filepath.Join(gameRoot, "disabledGameFiles", "SteamDRM")
	os.MkdirAll(targetParentDir, 0755)

	steamFixTarget := filepath.Join(targetParentDir, "on")

	// Чистим старую директорию on
	os.RemoveAll(steamFixTarget)

	// Перемещаем найденную папку с файлами
	if err := os.Rename(sourceDir, steamFixTarget); err != nil {
		return fmt.Errorf("не удалось переместить файлы SteamFix: %w", err)
	}

	slog.Info("Файлы восстановления Steam DRM успешно установлены", "target", steamFixTarget)
	if unpackCb != nil {
		unpackCb(100, "SteamFix установлен")
	}
	return nil
}
