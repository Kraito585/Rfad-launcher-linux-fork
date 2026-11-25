package recovery

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"rfad-launcher-linux/src-wails/utils"
	"strings"
)

func setEnv(base []string, overrides map[string]string) []string {
	out := make([]string, 0, len(base)+len(overrides))
	for _, kv := range base {
		k, _, ok := strings.Cut(kv, "=")
		if ok {
			if _, exists := overrides[k]; exists {
				continue
			}
		}
		out = append(out, kv)
	}
	for k, v := range overrides {
		out = append(out, k+"="+v)
	}
	return out
}

func RecoverPrefix(gameRoot string) error {
	prefixTarget := filepath.Join(gameRoot, "wine", "prefix")
	prefixPath := filepath.Join(prefixTarget, "pfx")

	slog.Info("Запуск лечения префикса", "prefixPath", prefixPath)

	dosdevicesPath := filepath.Join(prefixPath, "dosdevices")
	if err := os.RemoveAll(dosdevicesPath); err != nil {
		slog.Warn("Не удалось очистить dosdevices (не критично, продолжаем)", "error", err)
	}

	wineBinDir := filepath.Join(gameRoot, "wine", "bin")
	wineBin := filepath.Join(wineBinDir, "wine")
	wineserverBin := filepath.Join(wineBinDir, "wineserver")
	wineLibDir := filepath.Join(gameRoot, "wine", "lib")

	if _, err := os.Stat(wineBin); os.IsNotExist(err) {
		return fmt.Errorf("wine не найден, невозможно инициализировать префикс: %s", wineBin)
	}

	wineDllPath := strings.Join([]string{
		filepath.Join(wineLibDir, "wine", "x86_64-unix"),
		filepath.Join(wineLibDir, "wine", "x86_64-windows"),
		filepath.Join(wineLibDir, "wine", "i386-unix"),
		filepath.Join(wineLibDir, "wine", "i386-windows"),
	}, ":")

	ldParts := []string{
		filepath.Join(wineLibDir, "x86_64-linux-gnu"),
		filepath.Join(wineLibDir, "i386-linux-gnu"),
		wineLibDir,
	}
	if curLD := os.Getenv("LD_LIBRARY_PATH"); curLD != "" {
		ldParts = append(ldParts, curLD)
	}

	env := setEnv(os.Environ(), map[string]string{
		"WINEPREFIX":       prefixPath,
		"WINEDLLPATH":      wineDllPath,
		"LD_LIBRARY_PATH":  strings.Join(ldParts, ":"),
		"PATH":             wineBinDir + ":" + os.Getenv("PATH"),
		"WINEDLLOVERRIDES": "mscoree,mscoreei,mshtml=;uiautomationcore=;tabtip.exe=",
		"WINEDEBUG":        "-all",
	})

	cmd := utils.NewHostCommand(env, wineBin, "wineboot", "-u")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("ошибка при лечении префикса (wineboot): %w", err)
	}

	// Ждём завершения всех фоновых процессов wineboot (rundll32/services) и записи .update-timestamp
	if _, err := os.Stat(wineserverBin); err == nil {
		_ = utils.NewHostCommand(env, wineserverBin, "-w").Run()
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

	if err := InjectSteamBridge(gameRoot); err != nil {
		slog.Warn("Не удалось скопировать компоненты моста Steam", "err", err)
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

	if _, err := os.Stat(wineserverBin); err == nil {
		_ = utils.NewHostCommand(env, wineserverBin, "-w").Run()
	}

	slog.Info("Префикс успешно инициализирован и готов к запуску")
	return nil
}

func InjectSteamBridge(gameRoot string) error {
	wineLibDir := filepath.Join(gameRoot, "wine", "lib", "wine")
	pfxDir := filepath.Join(gameRoot, "wine", "prefix", "pfx")
	sys32 := filepath.Join(pfxDir, "drive_c", "windows", "system32")
	syswow64 := filepath.Join(pfxDir, "drive_c", "windows", "syswow64")

	pairs := [][2]string{
		{filepath.Join(wineLibDir, "x86_64-windows", "lsteamclient.dll"), filepath.Join(sys32, "lsteamclient.dll")},
		{filepath.Join(wineLibDir, "x86_64-windows", "steam.exe"), filepath.Join(sys32, "steam.exe")},
		{filepath.Join(wineLibDir, "i386-windows", "lsteamclient.dll"), filepath.Join(syswow64, "lsteamclient.dll")},
		{filepath.Join(wineLibDir, "i386-windows", "steam.exe"), filepath.Join(syswow64, "steam.exe")},
	}
	for _, p := range pairs {
		if _, err := os.Stat(p[0]); err != nil {
			continue
		}
		_ = os.Remove(p[1])
		if err := utils.CopyFile(p[0], p[1]); err != nil {
			return err
		}
	}

	// Копируем клиентские DLL из legacycompat нативного Steam в префикс
	homeDir, _ := os.UserHomeDir()
	steamDirs := []string{
		filepath.Join(homeDir, ".local", "share", "Steam"),
		filepath.Join(homeDir, ".steam", "steam"),
	}
	var legacyDir string
	for _, d := range steamDirs {
		cand := filepath.Join(d, "legacycompat")
		if info, err := os.Stat(cand); err == nil && info.IsDir() {
			legacyDir = cand
			break
		}
	}
	if legacyDir != "" {
		dstSteam := filepath.Join(pfxDir, "drive_c", "Program Files (x86)", "Steam")
		_ = os.MkdirAll(dstSteam, 0755)
		for _, dll := range []string{"steamclient.dll", "steamclient64.dll", "GameOverlayRenderer.dll", "GameOverlayRenderer64.dll"} {
			src := filepath.Join(legacyDir, dll)
			if _, err := os.Stat(src); err == nil {
				_ = utils.CopyFile(src, filepath.Join(dstSteam, dll))
			}
		}
	}
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
