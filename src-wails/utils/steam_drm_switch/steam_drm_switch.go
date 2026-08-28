package steam_drm_switch

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"rfad-launcher-linux/src-wails/utils"
)

func ToggleSteamDRM(ctx context.Context, gameRoot string, enable bool, unpackCb func(float64, string)) error {
	drmOnFiles := []string{"SkyrimSE.exe", "steam_api64.dll"}
	drmOffFiles := []string{"SkyrimSE.exe", "steam_api64.dll", "steam_api64"}

	steamDrmDir := filepath.Join(gameRoot, "disabledGameFiles", "SteamDRM")
	dirOn := filepath.Join(steamDrmDir, "on")
	dirOff := filepath.Join(steamDrmDir, "off")

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
			unpackCb(0.0, "Включение Steam DRM...")
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
		// Проверка отмены контекста
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		src := filepath.Join(sourceDir, f)
		dst := filepath.Join(gameRoot, f)
		percent := float64(i+1) / float64(totalOperations)

		// Проверяем, существует ли файл в хранилище ДО копирования
		if _, err := os.Stat(src); os.IsNotExist(err) {
			// Файла нет — просто пропускаем без паники и логов ошибки
			if unpackCb != nil {
				unpackCb(percent, fmt.Sprintf("Пропущен файл (не найден): %s", f))
			}
			continue
		}

		// Если файл есть — копируем
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

	return utils.SetOneSetting(gameRoot, "SteamFix:", valStr)
}
