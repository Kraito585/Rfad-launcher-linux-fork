package prefix_install

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"rfad-launcher-linux/src-wails/utils"
	"rfad-launcher-linux/src-wails/utils/recovery"
	"strings"
)

func UnpackPrefix(gameRoot string, unpackCb func(float64, string)) error {
	destDir := filepath.Join(gameRoot, "download")

	// ==========================================
	// 1. РАСПАКОВКА ПРЕФИКСА (rfad_clean_pfx -> wine/prefix/pfx)
	// ==========================================
	archivePath, err := utils.GetDownloadedPath(destDir, "prefix")
	if err != nil || archivePath == "" {
		return fmt.Errorf("архив префикса не найден: %w", err)
	}

	slog.Info("prefix archive path:", "path", archivePath)
	tempDir := filepath.Join(gameRoot, "temp_prefix_extract")
	os.RemoveAll(tempDir)
	if err := os.MkdirAll(tempDir, 0755); err != nil {
		return fmt.Errorf("не удалось создать временную папку префикса: %w", err)
	}
	defer os.RemoveAll(tempDir)

	if err := utils.ExtractArchive(archivePath, tempDir, func(p float64, msg string) {
		if unpackCb != nil {
			unpackCb(p*0.8, fmt.Sprintf("Префикс: %s", msg))
		}
	}); err != nil {
		return fmt.Errorf("ошибка распаковки префикса: %w", err)
	}

	entries, err := os.ReadDir(tempDir)
	if err != nil {
		return err
	}

	var sourceDir string
	for _, entry := range entries {
		if entry.IsDir() {
			name := strings.ToLower(entry.Name())
			if name == "rfad_clean_pfx" || name == "pfx" {
				sourceDir = filepath.Join(tempDir, entry.Name())
				break
			}
		}
	}
	if sourceDir == "" {
		return fmt.Errorf("в архиве префикса не найдена папка rfad_clean_pfx")
	}

	prefixParent := filepath.Join(gameRoot, "wine", "prefix")
	if err := os.MkdirAll(prefixParent, 0755); err != nil {
		return fmt.Errorf("не удалось создать папку %s: %w", prefixParent, err)
	}

	pfxTarget := filepath.Join(prefixParent, "pfx")
	if err := os.RemoveAll(pfxTarget); err != nil {
		return fmt.Errorf("не удалось удалить старый префикс %s: %w", pfxTarget, err)
	}

	if err := os.Rename(sourceDir, pfxTarget); err != nil {
		return fmt.Errorf("не удалось перенести префикс в %s: %w", pfxTarget, err)
	}

	// ==========================================
	// 2. РАСПАКОВКА ДОП. БИБЛИОТЕК В TMP/LIBS (если есть)
	// ==========================================
	if unpackCb != nil {
		unpackCb(0.85, "Распаковка библиотек окружения...")
	}

	libsArchivePath, err := utils.GetDownloadedPath(destDir, "libs")
	if err == nil && libsArchivePath != "" {
		libsTargetDir := filepath.Join(gameRoot, "wine", "tmp", "libs")
		os.RemoveAll(libsTargetDir)
		os.MkdirAll(libsTargetDir, 0755)

		if err := utils.ExtractArchive(libsArchivePath, libsTargetDir, nil); err != nil {
			slog.Warn("Ошибка распаковки библиотек окружения", "err", err)
		}
	}

	// ==========================================
	// 3. ЛЕЧЕНИЕ И ОБНОВЛЕНИЕ ПРЕФИКСА (wineboot -u)
	// ==========================================
	if unpackCb != nil {
		unpackCb(0.95, "Инициализация префикса...")
	}

	if err := recovery.RecoverPrefix(gameRoot); err != nil {
		return fmt.Errorf("ошибка при лечении префикса (wineboot): %w", err)
	}

	slog.Info("Префикс успешно распакован и настроен", "target", pfxTarget)
	if unpackCb != nil {
		unpackCb(1.0, "Установка префикса завершена")
	}

	return nil
}
