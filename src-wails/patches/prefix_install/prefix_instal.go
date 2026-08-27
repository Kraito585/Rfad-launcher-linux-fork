package prefix_install

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"rfad-launcher-linux/src-wails/utils"
)

func UnpackPrefix(ctx context.Context, gameRoot string) error {
	destDir := filepath.Join(gameRoot, "download")
	archivePath, err := utils.GetDownloadedPath(destDir, "prefix")
	if err != nil {
		return fmt.Errorf("не удалось найти путь к архиву префикса: %w", err)
	}
	if archivePath == "" {
		return fmt.Errorf("архив префикса не найден в статусе загрузки")
	}

	slog.Info("prefix archive path: %s", archivePath)

	tempDir := filepath.Join(gameRoot, "temp_prefix_extract")
	if err := os.MkdirAll(tempDir, 0755); err != nil {
		return fmt.Errorf("не удалось создать временную папку: %w", err)
	}
	defer os.RemoveAll(tempDir)

	if err := utils.ExtractArchive(ctx, archivePath, tempDir, func(p float64, msg string) {
		fmt.Printf("Распаковка префикса: %.0f%% %s\n", p*100, msg)
	}); err != nil {
		return fmt.Errorf("ошибка распаковки префикса: %w", err)
	}

	entries, err := os.ReadDir(tempDir)
	if err != nil {
		return fmt.Errorf("не удалось прочитать содержимое временной папки: %w", err)
	}

	var sourceDir string
	for _, entry := range entries {
		if entry.IsDir() && (entry.Name() == "RFAD_SE" || entry.Name() == "pfx dotnet") {
			sourceDir = filepath.Join(tempDir, entry.Name())
			break
		}
	}
	if sourceDir == "" {
		return fmt.Errorf("в архиве не найдена ожидаемая папка (RFAD_SE или pfx dotnet)")
	}

	wineDir := filepath.Join(gameRoot, "wine")
	if err := os.MkdirAll(wineDir, 0755); err != nil {
		return fmt.Errorf("не удалось создать папку wine: %w", err)
	}
	prefixTarget := filepath.Join(wineDir, "prefix")
	if err := os.RemoveAll(prefixTarget); err != nil {
		return fmt.Errorf("не удалось удалить старую папку prefix: %w", err)
	}
	if err := os.Rename(sourceDir, prefixTarget); err != nil {
		return fmt.Errorf("не удалось переименовать папку в prefix: %w", err)
	}

	fmt.Printf("Префикс успешно распакован и переименован в %s\n", prefixTarget)

	// ==========================================
	// ЛЕЧЕНИЕ ПРЕФИКСА ПОСЛЕ РАСПАКОВКИ
	// ==========================================

	prefixPath := filepath.Join(prefixTarget, "pfx")
	slog.Info("Запуск лечения префикса", "prefixPath", prefixPath)

	// 1. Удаляем битые симлинки (основная причина STATUS_DLL_NOT_FOUND c0000135)
	dosdevicesPath := filepath.Join(prefixPath, "dosdevices")
	if err := os.RemoveAll(dosdevicesPath); err != nil {
		slog.Warn("Не удалось очистить dosdevices (не критично, продолжаем)", "error", err)
	}

	// 2. Формируем пути к встроенным библиотекам и бинарнику Proton
	wineBin := filepath.Join(gameRoot, "wine", "proton", "files", "bin", "wine")
	wineBinDir := filepath.Join(gameRoot, "wine", "proton", "files", "bin")
	wineLibDir := filepath.Join(gameRoot, "wine", "proton", "files", "lib")
	wineLib64Dir := filepath.Join(gameRoot, "wine", "proton", "files", "lib64")

	if _, err := os.Stat(wineBin); os.IsNotExist(err) {
		return fmt.Errorf("wine не найден, невозможно инициализировать префикс: %s", wineBin)
	}

	wineDllPath := filepath.Join(wineLibDir, "wine") + ":" + filepath.Join(wineLib64Dir, "wine")
	ldLibraryPath := wineLibDir + ":" + wineLib64Dir + ":" +
		filepath.Join(wineLibDir, "x86_64-linux-gnu") + ":" +
		filepath.Join(wineLibDir, "i386-linux-gnu") + ":" + os.Getenv("LD_LIBRARY_PATH")

	// 3. Формируем окружение для wineboot
	env := os.Environ()
	env = append(env,
		"WINEPREFIX="+prefixPath,
		"WINEDLLPATH="+wineDllPath,
		"LD_LIBRARY_PATH="+ldLibraryPath,
		"PATH="+wineBinDir+":"+os.Getenv("PATH"),
		"WINEDEBUG=-all", // Отключаем лишний спам в консоль
	)

	// 4. Выполняем wineboot -u для пересоздания структуры
	cmd := exec.Command(wineBin, "wineboot", "-u")
	cmd.Env = env

	// Ждем завершения обновления префикса
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("ошибка при лечении префикса (wineboot): %w", err)
	}

	slog.Info("Префикс успешно инициализирован и готов к запуску")
	return nil
}
