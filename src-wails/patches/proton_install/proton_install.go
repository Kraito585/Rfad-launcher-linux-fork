package proton_install

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"rfad-launcher-linux/src-wails/utils"
	"strings"
)

func InstallGEProton(ctx context.Context, gameRoot string) error {
	destDir := filepath.Join(gameRoot, "download")
	protonPath, err := utils.GetDownloadedPath(destDir, "GE-Proton")
	if err != nil {
		return fmt.Errorf("не удалось найти путь к архиву Proton: %w", err)
	}
	if protonPath == "" {
		return fmt.Errorf("архив Proton не найден в статусе загрузки")
	}
	slog.Info("proton path: %s", protonPath)

	tempDir := filepath.Join(gameRoot, "temp_proton_extract")
	if err := os.MkdirAll(tempDir, 0755); err != nil {
		return fmt.Errorf("не удалось создать временную папку: %w", err)
	}
	defer os.RemoveAll(tempDir)

	if err := utils.ExtractArchive(ctx, protonPath, tempDir, func(p float64, msg string) {
		fmt.Printf("Распаковка Proton: %.0f%% %s\n", p*100, msg)
	}); err != nil {
		return fmt.Errorf("ошибка распаковки Proton: %w", err)
	}

	entries, err := os.ReadDir(tempDir)
	if err != nil {
		return fmt.Errorf("не удалось прочитать содержимое временной папки: %w", err)
	}

	var sourceDir string
	for _, entry := range entries {
		if entry.IsDir() && strings.Contains(entry.Name(), "GE-Proton") {
			sourceDir = filepath.Join(tempDir, entry.Name())
			break
		}
	}
	if sourceDir == "" {
		return fmt.Errorf("в архиве не найдена папка, содержащая 'GE-Proton'")
	}

	wineDir := filepath.Join(gameRoot, "wine")
	if err := os.MkdirAll(wineDir, 0755); err != nil {
		return fmt.Errorf("не удалось создать папку wine: %w", err)
	}
	protonTarget := filepath.Join(wineDir, "proton")
	if err := os.RemoveAll(protonTarget); err != nil {
		return fmt.Errorf("не удалось удалить старую папку proton: %w", err)
	}
	if err := os.Rename(sourceDir, protonTarget); err != nil {
		return fmt.Errorf("не удалось переименовать папку в proton: %w", err)
	}

	fmt.Printf("Proton успешно распакован и переименован в %s\n", protonTarget)
	return nil
}
