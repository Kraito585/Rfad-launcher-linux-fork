package proton_install

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"rfad-launcher-linux/src-wails/utils"
	"strings"
)

func InstallGEProton(gameRoot string, unpackCb func(float64, string)) error {
	destDir := filepath.Join(gameRoot, "download")
	wineArchivePath, err := utils.GetDownloadedPath(destDir, "Wine")
	if err != nil {
		return fmt.Errorf("не удалось найти путь к архиву Wine: %w", err)
	}
	if wineArchivePath == "" {
		return fmt.Errorf("архив Wine не найден в статусе загрузки")
	}
	slog.Info("wine archive path:", "path", wineArchivePath)

	tempDir := filepath.Join(gameRoot, "temp_wine_extract")
	os.RemoveAll(tempDir)
	if err := os.MkdirAll(tempDir, 0755); err != nil {
		return fmt.Errorf("не удалось создать временную папку: %w", err)
	}
	defer os.RemoveAll(tempDir)

	if err := utils.ExtractArchive(wineArchivePath, tempDir, func(p float64, msg string) {
		if unpackCb != nil {
			unpackCb(p, fmt.Sprintf("Wine: %s", msg))
		}
	}); err != nil {
		return fmt.Errorf("ошибка распаковки Wine: %w", err)
	}

	entries, err := os.ReadDir(tempDir)
	if err != nil {
		return fmt.Errorf("не удалось прочитать содержимое временной папки: %w", err)
	}

	var sourceDir string
	for _, entry := range entries {
		if entry.IsDir() && strings.ToLower(entry.Name()) == "wine" {
			sourceDir = filepath.Join(tempDir, entry.Name())
			break
		}
	}
	if sourceDir == "" {
		return fmt.Errorf("в архиве не найдена папка 'wine'")
	}

	wineTarget := filepath.Join(gameRoot, "wine")
	if err := os.MkdirAll(wineTarget, 0755); err != nil {
		return fmt.Errorf("не удалось создать папку wine: %w", err)
	}

	// Удаляем устаревшую папку proton от прошлых версий, если она осталась
	os.RemoveAll(filepath.Join(wineTarget, "proton"))

	// Переносим содержимое wine/ (bin, lib, share, include) в gameRoot/wine/, не трогая prefix и tmp
	subEntries, err := os.ReadDir(sourceDir)
	if err != nil {
		return fmt.Errorf("не удалось прочитать содержимое папки wine: %w", err)
	}

	for _, sub := range subEntries {
		srcPath := filepath.Join(sourceDir, sub.Name())
		dstPath := filepath.Join(wineTarget, sub.Name())

		if err := os.RemoveAll(dstPath); err != nil {
			return fmt.Errorf("не удалось удалить старый %s: %w", dstPath, err)
		}
		if err := os.Rename(srcPath, dstPath); err != nil {
			return fmt.Errorf("не удалось перенести %s в %s: %w", srcPath, dstPath, err)
		}
	}

	// Контрольная проверка бинарника
	wineBin := filepath.Join(wineTarget, "bin", "wine")
	if _, err := os.Stat(wineBin); err != nil {
		return fmt.Errorf("после распаковки не найден бинарник %s: %w", wineBin, err)
	}

	slog.Info("Wine успешно распакован", "target", wineTarget)

	if unpackCb != nil {
		unpackCb(1.0, "Настройка файлов Wine завершена")
	}

	return nil
}
