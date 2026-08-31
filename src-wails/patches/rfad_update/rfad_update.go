package rfad_update

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"rfad-launcher-linux/src-wails/downloader"
	"rfad-launcher-linux/src-wails/utils"
	"strings"
)

// ProcessUpdate перемещает архив, распаковывает его и заменяет EngineFixes.dll
func InstallUpdate(ctx context.Context, gameRoot string, unpackCb func(float64, string)) error {
	destDir := filepath.Join(gameRoot, "download")
	statusFile := filepath.Join(destDir, "download_status.txt")

	// Читаем все строки статуса
	data, err := os.ReadFile(statusFile)
	if err != nil {
		return fmt.Errorf("не удалось прочитать файл статуса: %w", err)
	}
	lines := strings.Split(string(data), "\n")

	// 1. Собираем все пути для обновления
	var updatePaths []string
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "complete update ") {
			updatePaths = append(updatePaths, strings.TrimPrefix(trimmed, "complete update "))
		}
	}

	var updatePath string

	// 2. Логика выбора файла в зависимости от источника
	if len(updatePaths) == 1 {
		// Если файл только один (например, с CDN), берем его без проверок имени
		updatePath = updatePaths[0]
	} else if len(updatePaths) > 1 {
		// Если файлов несколько (Google Drive), ищем конкретный архив
		for _, path := range updatePaths {
			if strings.HasSuffix(path, ".zip") && strings.Contains(path, "RFAD_PATCH") {
				updatePath = path
				break
			}
		}
	}

	if updatePath == "" {
		return fmt.Errorf("архив обновления не найден в статусе загрузки (найдено записей: %d)", len(updatePaths))
	}
	slog.Info("update path:", "path", updatePath)

	targetDir := filepath.Join(gameRoot, "MO2", "mods", "RFAD_PATCH")
	if err := os.RemoveAll(targetDir); err != nil {
		return fmt.Errorf("не удалось очистить папку RFAD_PATCH: %w", err)
	}
	if err := os.MkdirAll(targetDir, 0755); err != nil {
		return fmt.Errorf("не удалось создать папку RFAD_PATCH: %w", err)
	}

	// Передаем коллбэк для отображения прогресса во Vue
	if err := utils.ExtractArchive(ctx, updatePath, targetDir, func(p float64, msg string) {
		if unpackCb != nil {
			// Добавляем префикс "Обновление:"
			unpackCb(p, fmt.Sprintf("Обновление: %s", msg))
		}
	}); err != nil {
		return fmt.Errorf("ошибка распаковки обновления: %w", err)
	}

	// Если архив содержит общую корневую папку, перемещаем содержимое на уровень выше
	entries, err := os.ReadDir(targetDir)
	if err != nil {
		return err
	}
	if len(entries) == 1 && entries[0].IsDir() {
		subDir := filepath.Join(targetDir, entries[0].Name())
		subEntries, err := os.ReadDir(subDir)
		if err != nil {
			return err
		}
		for _, e := range subEntries {
			src := filepath.Join(subDir, e.Name())
			dst := filepath.Join(targetDir, e.Name())
			if err := os.Rename(src, dst); err != nil {
				return err
			}
		}
		os.Remove(subDir)
	}

	slog.Info("Обновление успешно установлено", "target", targetDir)

	// Финальное сообщение для закрытия прогресс-бара этого этапа
	if unpackCb != nil {
		unpackCb(1.0, "Обновление успешно установлено")
	}

	return nil
}

func DownloadUpdate(ctx context.Context, gameRoot string, creds []byte, useCDN bool, progressCb func(float64, float64, string)) error {
	downloadType := "gdrive"
	if useCDN {
		downloadType = "cdn"
	}

	err := downloader.DownloadUpdate(ctx, gameRoot, downloadType, creds, true, progressCb)
	if err != nil {
		return fmt.Errorf("ошибка загрузки обновления: %w", err)
	}

	return nil
}
