package recovery

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// ==========================
// PREFIX RECOVERY UTILS
// ==========================

func RecoverPrefix(ctx context.Context, gameRoot string) error {
	prefixTarget := filepath.Join(gameRoot, "wine", "prefix")
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
	// Используем CommandContext для поддержки отмены операции
	cmd := exec.CommandContext(ctx, wineBin, "wineboot", "-u")
	cmd.Env = env

	// Ждем завершения обновления префикса
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("ошибка при лечении префикса (wineboot): %w", err)
	}

	injectedDlls, err := InjectLibraries(gameRoot)
	if err != nil {
		slog.Warn("Ошибка инъекции библиотек в system32", "err", err)
	}

	// Динамически прописываем скопированные DLL в реестр (чтобы Wine использовал native,builtin)
	if len(injectedDlls) > 0 {
		if err := ApplyRegistryOverrides(ctx, env, wineBin, injectedDlls); err != nil {
			slog.Warn("Ошибка модификации реестра", "err", err)
		}
	}

	slog.Info("Префикс успешно инициализирован и готов к запуску")
	return nil
}

// InjectLibraries подменяет системные заглушки внутри самого Proton
func InjectLibraries(gameRoot string) ([]string, error) {
	slog.Info("Начало копирования библиотек в system32")

	libsDir := filepath.Join(gameRoot, "wine", "tmp", "libs")
	system32Dir := filepath.Join(gameRoot, "wine", "prefix", "pfx", "drive_c", "windows", "system32")

	if _, err := os.Stat(libsDir); os.IsNotExist(err) {
		return nil, fmt.Errorf("папка с библиотеками не найдена: %s", libsDir)
	}

	os.MkdirAll(system32Dir, 0755)

	entries, err := os.ReadDir(libsDir)
	if err != nil {
		return nil, fmt.Errorf("не удалось прочитать директорию библиотек: %w", err)
	}

	var injectedDlls []string
	count := 0

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		fileName := entry.Name()
		srcFile := filepath.Join(libsDir, fileName)
		dstFile := filepath.Join(system32Dir, fileName)

		// Удаляем старую заглушку Wine
		os.Remove(dstFile)

		// Копируем файл
		if err := copyFile(srcFile, dstFile); err != nil {
			slog.Warn("Ошибка копирования файла", "file", fileName, "err", err)
		} else {
			// Отрезаем расширение (.dll) для записи в реестр
			ext := filepath.Ext(fileName)
			baseName := strings.TrimSuffix(fileName, ext)
			injectedDlls = append(injectedDlls, baseName)

			count++
		}
	}

	slog.Info("Копирование библиотек успешно завершено", "скопировано_файлов", count)
	return injectedDlls, nil
}

// Вспомогательная функция для копирования файлов
func copyFile(src, dst string) error {
	source, err := os.Open(src)
	if err != nil {
		return err
	}
	defer source.Close()

	destination, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer destination.Close()

	_, err = io.Copy(destination, source)
	return err
}

func ApplyRegistryOverrides(ctx context.Context, env []string, wineBin string, dlls []string) error {
	if len(dlls) == 0 {
		slog.Info("Список DLL для реестра пуст, пропускаем шаг")
		return nil
	}

	slog.Info("Запись DLL Overrides в реестр префикса", "количество", len(dlls))

	for _, dll := range dlls {
		// Команда перезапишет ключ, если он уже есть (благодаря флагу /f)
		cmd := exec.CommandContext(ctx, wineBin, "reg", "add",
			`HKEY_CURRENT_USER\Software\Wine\DllOverrides`,
			"/v", dll,
			"/t", "REG_SZ",
			"/d", "native,builtin",
			"/f",
		)
		cmd.Env = env

		if err := cmd.Run(); err != nil {
			slog.Warn("Не удалось прописать override в реестр", "dll", dll, "err", err)
		}
	}

	slog.Info("Реестр успешно обновлен динамическим списком")
	return nil
}

func RecoverSteamDRM(ctx context.Context, gameRoot string, creds []byte) error {
	return nil
}
