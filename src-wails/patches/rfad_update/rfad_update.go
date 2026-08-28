package rfad_update

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
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
	var updatePath string
	for _, line := range lines {
		if strings.HasPrefix(line, "complete update ") {
			path := strings.TrimPrefix(line, "complete update ")
			// Выбираем файл, который заканчивается на .zip и содержит "RFAD_PATCH"
			if strings.HasSuffix(path, ".zip") && strings.Contains(path, "RFAD_PATCH") {
				updatePath = path
				break
			}
		}
	}

	if updatePath == "" {
		return fmt.Errorf("архив обновления не найден в статусе загрузки")
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

// func EnablePlugin(gamePath string, pluginName string) error {
// 	// Путь к файлу плагинов внутри профиля
// 	pluginTxtPath := filepath.Join(gamePath, "MO2/profiles/RFAD_SE/plugins.txt")

// 	// 1. Читаем существующий файл
// 	file, err := os.OpenFile(pluginTxtPath, os.O_RDWR|os.O_CREATE, 0644)
// 	if err != nil {
// 		return err
// 	}
// 	defer file.Close()

// 	// 2. Проверяем, нет ли уже такого плагина (ищем как с *, так и без)
// 	found := false
// 	scanner := bufio.NewScanner(file)
// 	for scanner.Scan() {
// 		line := scanner.Text()
// 		if line == "*"+pluginName || line == pluginName {
// 			found = true
// 			break
// 		}
// 	}

// 	if err := scanner.Err(); err != nil {
// 		fmt.Errorf("Ошибка при чтении потока сканером: %v", err)
// 	}

// 	// 3. Если не нашли, дописываем плагин со звездочкой
// 	if !found {
// 		// Переходим в конец файла для записи
// 		_, err := file.Seek(0, 2)
// 		if err != nil {
// 			return err
// 		}

// 		// Добавляем новую строку со звездочкой
// 		if _, err := file.WriteString("\n*" + pluginName); err != nil {
// 			return err
// 		}
// 		fmt.Sprintf("Плагин %s успешно активирован в plugins.txt", pluginName)
// 	} else {
// 		fmt.Sprintf("Плагин %s уже активен", pluginName)
// 	}

// 	return nil
// }
