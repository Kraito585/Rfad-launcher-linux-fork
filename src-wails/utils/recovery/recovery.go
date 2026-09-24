package recovery

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"rfad-launcher-linux/src-wails/utils"
	"strings"
)

// ==========================
// PREFIX RECOVERY UTILS
// ==========================

func RecoverPrefix(gameRoot string) error {
	prefixTarget := filepath.Join(gameRoot, "wine", "prefix")
	prefixPath := filepath.Join(prefixTarget, "pfx")

	slog.Info("Запуск лечения префикса (Proton PE-режим)", "prefixPath", prefixPath)

	// =========================================================================
	// 1. ФИКС МЕРТВЫХ СИМЛИНКОВ (ГЛАВНАЯ ПРИЧИНА c0000135)
	// Создаем мосты в папке wine/, чтобы относительные пути ../../../../../lib
	// из default_pfx корректно разрешались в папку proton/files/lib
	// =========================================================================
	wineDir := filepath.Join(gameRoot, "wine")

	os.Remove(filepath.Join(wineDir, "lib"))
	os.Symlink("proton/files/lib", filepath.Join(wineDir, "lib"))

	os.Remove(filepath.Join(wineDir, "lib64"))
	os.Symlink("proton/files/lib64", filepath.Join(wineDir, "lib64"))

	os.Remove(filepath.Join(wineDir, "share"))
	os.Symlink("proton/files/share", filepath.Join(wineDir, "share"))

	slog.Info("Мосты для относительных симлинков Proton успешно созданы")

	// =========================================================================
	// 2. РАЗБЛОКИРОВКА ПРАВ
	// =========================================================================
	slog.Info("Выдача прав на запись (chmod +w) для префикса...")
	chmodCmd := utils.NewHostCommand(nil, "chmod", "-R", "u+w", prefixTarget)
	_ = chmodCmd.Run()

	// =========================================================================
	// 3. ВОССОЗДАНИЕ DOSDEVICES
	// =========================================================================
	dosdevicesPath := filepath.Join(prefixPath, "dosdevices")
	os.RemoveAll(dosdevicesPath)
	os.MkdirAll(dosdevicesPath, 0755)

	driveCPath := filepath.Join(prefixPath, "drive_c")
	os.MkdirAll(driveCPath, 0755)

	cDrivePath := filepath.Join(dosdevicesPath, "c:")
	os.Symlink("../drive_c", cDrivePath)

	zDrivePath := filepath.Join(dosdevicesPath, "z:")
	os.Symlink("/", zDrivePath)

	// =========================================================================
	// 4. БАЗОВЫЕ ПУТИ PROTON
	// =========================================================================
	protonDir := filepath.Join(gameRoot, "wine", "proton", "files")
	wineBinDir := filepath.Join(protonDir, "bin")
	wineLibDir := filepath.Join(protonDir, "lib")
	wineLib64Dir := filepath.Join(protonDir, "lib64")
	wineShareDir := filepath.Join(protonDir, "share", "wine")

	wineBin := filepath.Join(wineBinDir, "wine64")
	if _, err := os.Stat(wineBin); os.IsNotExist(err) {
		wineBin = filepath.Join(wineBinDir, "wine")
	}
	wineServer := filepath.Join(wineBinDir, "wineserver")

	if _, err := os.Stat(wineBin); os.IsNotExist(err) {
		return fmt.Errorf("wine не найден: %s", wineBin)
	}

	// =========================================================================
	// 5. ОКРУЖЕНИЕ И ЗАПУСК
	// =========================================================================
	wineDllPath := filepath.Join(wineLib64Dir, "wine") + ":" + filepath.Join(wineLibDir, "wine")
	ldLibraryPath := wineLibDir + ":" + wineLib64Dir + ":" +
		filepath.Join(wineLibDir, "x86_64-linux-gnu") + ":" +
		filepath.Join(wineLibDir, "i386-linux-gnu") + ":" + os.Getenv("LD_LIBRARY_PATH")

	env := os.Environ()
	env = append(env,
		"WINEPREFIX="+prefixPath,
		"WINEDLLPATH="+wineDllPath,
		"WINEDATADIR="+wineShareDir,
		"WINESERVER="+wineServer,
		"LD_LIBRARY_PATH="+ldLibraryPath,
		"PATH="+wineBinDir+":"+os.Getenv("PATH"),
		"WINEARCH=win64",
		"WINEESYNC=0",
		"WINEFSYNC=0",
		"WINEDEBUG=-all",
	)

	slog.Info("Запуск wineboot -u", "wineBin", wineBin)
	cmd := utils.NewHostCommand(env, wineBin, "wineboot", "-u")
	cmd.Dir = prefixPath

	out, err := cmd.CombinedOutput()
	if err != nil {
		slog.Error("wineboot завершился с ошибкой", "err", err, "out", string(out))
		return fmt.Errorf("ошибка при лечении префикса (wineboot): %w\nВывод: %s", err, string(out))
	}
	slog.Info("wineboot успешно завершен", "out", string(out))

	// =========================================================================
	// 6. ИНЪЕКЦИЯ БИБЛИОТЕК И РЕЕСТР
	// =========================================================================
	slog.Info("Запуск InjectLibraries...")
	injectedDlls, err := InjectLibraries(gameRoot)
	if err != nil {
		slog.Warn("Ошибка инъекции библиотек в system32", "err", err)
	}

	if len(injectedDlls) > 0 {
		if err := ApplyRegistryOverrides(env, wineBin, injectedDlls); err != nil {
			slog.Warn("Ошибка модификации реестра", "err", err)
		}
	}

	slog.Info("Префикс успешно инициализирован!")
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

		os.Remove(dstFile)

		// Используем общую надежную функцию копирования
		if err := utils.CopyFile(srcFile, dstFile); err != nil {
			slog.Warn("Ошибка копирования файла", "file", fileName, "err", err)
		} else {
			ext := filepath.Ext(fileName)
			baseName := strings.TrimSuffix(fileName, ext)
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
		// Заменили CommandContext на обычный Command
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
