package steam_drm_switch

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"rfad-launcher-linux/src-wails/utils"
)

func ToggleSteamDRM(ctx context.Context, gameRoot string, enable bool, progressCb func(float64, string)) error {
	// Списки файлов для разных состояний
	drmOnFiles := []string{
		"SkyrimSE.exe",
		"steam_api64.dll",
	}
	drmOffFiles := []string{
		"SkyrimSE.exe",
		"steam_api64.dll",
		"steam_api64",
	}

	steamDrmDir := filepath.Join(gameRoot, "disabledGameFiles", "SteamDRM")
	dirOn := filepath.Join(steamDrmDir, "on")
	dirOff := filepath.Join(steamDrmDir, "off")

	os.MkdirAll(dirOn, 0755)
	os.MkdirAll(dirOff, 0755)

	// Подсчитываем общее количество операций для точного прогресс-бара
	totalOperations := len(drmOnFiles) + len(drmOffFiles)
	currentOp := 0

	// Вспомогательная функция для перемещения с вызовом колбэка
	moveFiles := func(files []string, srcDir, destDir string) {
		for _, f := range files {
			// Проверка отмены контекста
			select {
			case <-ctx.Done():
				return
			default:
			}

			currentOp++
			percent := float64(currentOp) / float64(totalOperations)

			src := filepath.Join(srcDir, f)
			dst := filepath.Join(destDir, f)

			if _, err := os.Stat(src); err == nil {
				os.RemoveAll(dst)
				if err := os.Rename(src, dst); err == nil {
					if progressCb != nil {
						progressCb(percent, fmt.Sprintf("Перемещен файл: %s", f))
					}
				} else {
					if progressCb != nil {
						progressCb(percent, fmt.Sprintf("Ошибка перемещения: %s", f))
					}
				}
			} else {
				if progressCb != nil {
					progressCb(percent, fmt.Sprintf("Пропущен файл (не найден): %s", f))
				}
			}
		}
	}

	if enable {
		if progressCb != nil {
			progressCb(0.0, "Включение Steam DRM...")
		}
		moveFiles(drmOffFiles, gameRoot, dirOff)
		moveFiles(drmOnFiles, dirOn, gameRoot)
	} else {
		if progressCb != nil {
			progressCb(0.0, "Отключение Steam DRM...")
		}
		moveFiles(drmOnFiles, gameRoot, dirOn)
		moveFiles(drmOffFiles, dirOff, gameRoot)
	}

	// Финальный вызов прогресса
	if progressCb != nil {
		progressCb(1.0, "Обновление конфигурации...")
	}

	// Обновление конфигурационного файла (передаем строку)
	valStr := "false"
	if enable {
		valStr = "true"
	}
	return utils.UpdateLauncherConfig(gameRoot, "SteamDRM", valStr)
}
