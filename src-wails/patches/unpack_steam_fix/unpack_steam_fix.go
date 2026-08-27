package unpacksteamfix

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"rfad-launcher-linux/src-wails/utils"
)

func UnpackSteamFix(ctx context.Context, gameRoot string) error {
	destDir := filepath.Join(gameRoot, "download")
	steamFixPath, err := utils.GetDownloadedPath(destDir, "steamfix")
	if err != nil {
		return fmt.Errorf("не удалось найти путь к архиву SteamFix: %w", err)
	}
	if steamFixPath == "" {
		return fmt.Errorf("архив SteamFix не найден в статусе загрузки")
	}
	slog.Info("steamfix archive path:", "path", steamFixPath)

	tempDir := filepath.Join(gameRoot, "temp_steamfix_extract")
	if err := os.MkdirAll(tempDir, 0755); err != nil {
		return fmt.Errorf("не удалось создать временную папку: %w", err)
	}
	// Обязательно убираем за собой мусор
	defer os.RemoveAll(tempDir)

	if err := utils.ExtractArchive(ctx, steamFixPath, tempDir, func(p float64, msg string) {
		fmt.Printf("Распаковка SteamFix: %.0f%% %s\n", p*100, msg)
	}); err != nil {
		return fmt.Errorf("ошибка распаковки SteamFix: %w", err)
	}

	entries, err := os.ReadDir(tempDir)
	if err != nil {
		return fmt.Errorf("не удалось прочитать содержимое временной папки: %w", err)
	}
	if len(entries) == 0 {
		return fmt.Errorf("архив SteamFix пуст")
	}

	// Определяем источник файлов.
	// Если внутри архива лежит только одна папка, берем её содержимое.
	// Если там сразу лежат файлы, берем саму временную папку.
	sourceDir := tempDir
	if len(entries) == 1 && entries[0].IsDir() {
		sourceDir = filepath.Join(tempDir, entries[0].Name())
	}

	// Формируем целевой путь: gameRoot/disabledGameFiles/SteamDRM/on
	targetParentDir := filepath.Join(gameRoot, "disabledGameFiles", "SteamDRM")
	if err := os.MkdirAll(targetParentDir, 0755); err != nil {
		return fmt.Errorf("не удалось создать родительскую директорию для SteamDRM: %w", err)
	}

	steamFixTarget := filepath.Join(targetParentDir, "on")

	// Удаляем старую папку "on", если она там осталась от предыдущих манипуляций
	if err := os.RemoveAll(steamFixTarget); err != nil {
		return fmt.Errorf("не удалось очистить целевую папку steamfix: %w", err)
	}

	// Перемещаем распакованные файлы в целевую директорию
	if err := os.Rename(sourceDir, steamFixTarget); err != nil {
		return fmt.Errorf("не удалось переместить файлы SteamFix: %w", err)
	}

	fmt.Printf("Файлы восстановления Steam DRM успешно распакованы в %s\n", steamFixTarget)
	return nil
}
