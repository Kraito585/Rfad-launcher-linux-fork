package prefix_install

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"rfad-launcher-linux/src-wails/utils"
	"rfad-launcher-linux/src-wails/utils/recovery"
)

func UnpackPrefix(ctx context.Context, gameRoot string, unpackCb func(float64, string)) error {
	destDir := filepath.Join(gameRoot, "download")

	// ==========================================
	// 1. РАСПАКОВКА ПРЕФИКСА
	// ==========================================
	archivePath, err := utils.GetDownloadedPath(destDir, "prefix")
	if err != nil || archivePath == "" {
		return fmt.Errorf("архив префикса не найден: %w", err)
	}

	slog.Info("prefix archive path:", "path", archivePath)
	tempDir := filepath.Join(gameRoot, "temp_prefix_extract")
	os.MkdirAll(tempDir, 0755)
	defer os.RemoveAll(tempDir)

	if err := utils.ExtractArchive(ctx, archivePath, tempDir, func(p float64, msg string) {
		if unpackCb != nil {
			unpackCb(p*0.8, fmt.Sprintf("Префикс: %s", msg))
		} // Выделяем 80% на префикс
	}); err != nil {
		return fmt.Errorf("ошибка распаковки префикса: %w", err)
	}

	entries, err := os.ReadDir(tempDir)
	if err != nil {
		return err
	}

	var sourceDir string
	for _, entry := range entries {
		if entry.IsDir() && (entry.Name() == "RFAD_SE" || entry.Name() == "pfx dotnet") {
			sourceDir = filepath.Join(tempDir, entry.Name())
			break
		}
	}
	if sourceDir == "" {
		return fmt.Errorf("в архиве не найдена ожидаемая папка")
	}

	wineDir := filepath.Join(gameRoot, "wine")
	os.MkdirAll(wineDir, 0755)
	prefixTarget := filepath.Join(wineDir, "prefix")
	os.RemoveAll(prefixTarget)
	if err := os.Rename(sourceDir, prefixTarget); err != nil {
		return err
	}

	// ==========================================
	// 2. РАСПАКОВКА БИБЛИОТЕК В TMP/LIBS
	// ==========================================
	if unpackCb != nil {
		unpackCb(0.85, "Распаковка библиотек окружения (DXVK)...")
	}

	libsArchivePath, err := utils.GetDownloadedPath(destDir, "libs")
	if err == nil && libsArchivePath != "" {
		libsTargetDir := filepath.Join(gameRoot, "wine", "tmp", "libs")

		// Чистим старую папку перед распаковкой новых библиотек
		os.RemoveAll(libsTargetDir)
		os.MkdirAll(libsTargetDir, 0755)

		if err := utils.ExtractArchive(ctx, libsArchivePath, libsTargetDir, nil); err != nil {
			slog.Warn("Ошибка распаковки библиотек окружения", "err", err)
		}
	} else {
		slog.Warn("Архив с библиотеками (libs) не найден, симлинки созданы не будут")
	}

	// ==========================================
	// 3. ЛЕЧЕНИЕ И СОЗДАНИЕ СИМЛИНКОВ
	// ==========================================
	if unpackCb != nil {
		unpackCb(0.95, "Интеграция библиотек в систему...")
	}

	if err := recovery.RecoverPrefix(ctx, gameRoot); err != nil {
		return fmt.Errorf("ошибка при лечении префикса (wineboot): %w", err)
	}

	slog.Info("Префикс успешно распакован и настроен")
	if unpackCb != nil {
		unpackCb(1.0, "Установка префикса завершена")
	}

	return nil
}
