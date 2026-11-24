package recovery

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"rfad-launcher-linux/src-wails/utils"
	"strings"
)

func RecoverPrefix(gameRoot string) error {
	prefixTarget := filepath.Join(gameRoot, "wine", "prefix")
	prefixPath := filepath.Join(prefixTarget, "pfx")

	slog.Info("Запуск лечения префикса", "prefixPath", prefixPath)

	// 1. Удаляем битые симлинки
	dosdevicesPath := filepath.Join(prefixPath, "dosdevices")
	if err := os.RemoveAll(dosdevicesPath); err != nil {
		slog.Warn("Не удалось очистить dosdevices (не критично, продолжаем)", "error", err)
	}

	// 2. Пути к wine-tkg
	wineBinDir := filepath.Join(gameRoot, "wine", "bin")
	wineBin := filepath.Join(wineBinDir, "wine")
	wineLibDir := filepath.Join(gameRoot, "wine", "lib")

	if _, err := os.Stat(wineBin); os.IsNotExist(err) {
		return fmt.Errorf("wine не найден, невозможно инициализировать префикс: %s", wineBin)
	}

	wineDllPath := filepath.Join(wineLibDir, "wine")
	ldLibraryPath := wineLibDir + ":" + os.Getenv("LD_LIBRARY_PATH")

	// 3. Окружение для wineboot
	env := os.Environ()
	env = append(env,
		"WINEPREFIX="+prefixPath,
		"WINEDLLPATH="+wineDllPath,
		"LD_LIBRARY_PATH="+ldLibraryPath,
		"PATH="+wineBinDir+":"+os.Getenv("PATH"),
		"WINEDLLOVERRIDES=mscoree,mshtml=",
		"WINEDEBUG=-all",
	)

	// 4. Выполняем wineboot -u
	cmd := utils.NewHostCommand(env, wineBin, "wineboot", "-u")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("ошибка при лечении префикса (wineboot): %w", err)
	}

	usersDir := filepath.Join(prefixPath, "drive_c", "users")
	if userEntries, err := os.ReadDir(usersDir); err == nil {
		for _, u := range userEntries {
			if !u.IsDir() || strings.EqualFold(u.Name(), "Public") {
				continue
			}
			userRoot := filepath.Join(usersDir, u.Name())
			_ = os.MkdirAll(filepath.Join(userRoot, "AppData", "Local", "Skyrim Special Edition"), 0755)
			_ = os.MkdirAll(filepath.Join(userRoot, "Documents", "My Games", "Skyrim Special Edition"), 0755)
		}
	}

	injectedDlls, err := InjectLibraries(gameRoot)
	if err != nil {
		slog.Warn("Ошибка инъекции библиотек в system32", "err", err)
	}

	if len(injectedDlls) > 0 {
		if err := ApplyRegistryOverrides(env, wineBin, injectedDlls); err != nil {
			slog.Warn("Ошибка модификации реестра", "err", err)
		}
	}

	slog.Info("Префикс успешно инициализирован и готов к запуску")
	return nil
}

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

		origName := entry.Name()
		lowerName := strings.ToLower(origName)

		// Не перезаписываем встроенный XInput Wine (нужен для геймпадов / Steam Deck)
		if strings.HasPrefix(lowerName, "xinput") {
			continue
		}

		srcFile := filepath.Join(libsDir, origName)
		dstFile := filepath.Join(system32Dir, lowerName)

		os.Remove(dstFile)

		if err := utils.CopyFile(srcFile, dstFile); err != nil {
			slog.Warn("Ошибка копирования файла", "file", origName, "err", err)
		} else {
			ext := filepath.Ext(lowerName)
			baseName := strings.TrimSuffix(lowerName, ext)
			injectedDlls = append(injectedDlls, baseName)
			count++
		}
	}

	slog.Info("Копирование библиотек успешно завершено", "скопировано_файлов", count)
	return injectedDlls, nil
}

func ApplyRegistryOverrides(env []string, wineBin string, dlls []string) error {
	if len(dlls) == 0 {
		slog.Info("Список DLL для реестра пуст, пропускаем шаг")
		return nil
	}

	slog.Info("Запись DLL Overrides в реестр префикса", "количество", len(dlls))

	for _, dll := range dlls {
		cmd := utils.NewHostCommand(env, wineBin, "reg", "add",
			`HKEY_CURRENT_USER\Software\Wine\DllOverrides`,
			"/v", dll, "/t", "REG_SZ", "/d", "native,builtin", "/f")

		if err := cmd.Run(); err != nil {
			slog.Warn("Не удалось прописать override в реестр", "dll", dll, "err", err)
		}
	}

	slog.Info("Реестр успешно обновлен динамическим списком")
	return nil
}
