package prefix_install

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"rfad-launcher-linux/src-wails/utils"
	"rfad-launcher-linux/src-wails/utils/recovery"
	"strings"
)

func UnpackPrefix(gameRoot string, unpackCb func(float64, string)) error {
	destDir := filepath.Join(gameRoot, "download")

	// ==========================================
	// 1. СОЗДАНИЕ ПРЕФИКСА ИЗ ШАБЛОНА PROTON
	// ==========================================
	if unpackCb != nil {
		unpackCb(0.1, "Формирование чистого префикса...")
	}

	sourcePfx := filepath.Join(gameRoot, "wine", "proton", "files", "share", "default_pfx")
	if _, err := os.Stat(sourcePfx); os.IsNotExist(err) {
		return fmt.Errorf("шаблон префикса не найден: %s (убедитесь, что Proton скачан и распакован)", sourcePfx)
	}

	wineDir := filepath.Join(gameRoot, "wine", "prefix")
	prefixTarget := filepath.Join(wineDir, "pfx")

	os.RemoveAll(wineDir)
	if err := os.MkdirAll(wineDir, 0755); err != nil {
		return fmt.Errorf("ошибка создания директории префикса: %w", err)
	}

	// Копируем префикс с сохранением всех важных симлинков
	cmd := utils.NewHostCommand(nil, "cp", "-a", sourcePfx, prefixTarget)
	if out, err := cmd.CombinedOutput(); err != nil {
		slog.Error("Ошибка копирования default_pfx", "out", string(out), "err", err)
		return fmt.Errorf("ошибка копирования шаблона префикса: %v", err)
	}

	// ==========================================
	// 2. РАСПАКОВКА БИБЛИОТЕК В TMP/LIBS
	// ==========================================
	if unpackCb != nil {
		unpackCb(0.4, "Распаковка библиотек окружения (DXVK)...")
	}

	libsArchivePath, err := utils.GetDownloadedPath(destDir, "libs")
	if err == nil && libsArchivePath != "" {
		libsTargetDir := filepath.Join(gameRoot, "wine", "tmp", "libs")

		os.RemoveAll(libsTargetDir)
		os.MkdirAll(libsTargetDir, 0755)

		if err := utils.ExtractArchive(libsArchivePath, libsTargetDir, nil); err != nil {
			slog.Warn("Ошибка распаковки библиотек окружения", "err", err)
		}
	} else {
		slog.Warn("Архив с библиотеками (libs) не найден, симлинки созданы не будут")
	}

	// ==========================================
	// 3. ЛЕЧЕНИЕ И СОЗДАНИЕ СИМЛИНКОВ
	// ==========================================
	if unpackCb != nil {
		unpackCb(0.5, "Инициализация реестра префикса...")
	}

	// Запускаем wineboot и хуки лечения
	if err := recovery.RecoverPrefix(gameRoot); err != nil {
		return fmt.Errorf("ошибка при лечении префикса (wineboot): %w", err)
	}

	// ==========================================
	// 4. ТИХАЯ УСТАНОВКА СИСТЕМНЫХ БИБЛИОТЕК
	// ==========================================
	if unpackCb != nil {
		unpackCb(0.6, "Установка системных компонентов (VCRuntime, .NET)...")
	}

	statusFile := filepath.Join(destDir, "download_status.txt")
	var redistPaths []string

	if content, err := os.ReadFile(statusFile); err == nil {
		lines := strings.Split(string(content), "\n")
		for _, line := range lines {
			line = strings.TrimSpace(line)
			parts := strings.SplitN(line, " ", 3)
			if len(parts) == 3 && parts[0] == "complete" && parts[1] == "redists" {
				redistPaths = append(redistPaths, strings.TrimSpace(parts[2]))
			}
		}
	}

	if len(redistPaths) > 0 {
		for i, exePath := range redistPaths {
			if _, statErr := os.Stat(exePath); os.IsNotExist(statErr) {
				slog.Warn("Файл библиотеки не найден на диске", "path", exePath)
				continue
			}

			// Плавное заполнение прогресс-бара от 60% до 95%
			if unpackCb != nil {
				step := 0.6 + (float64(i)/float64(len(redistPaths)))*0.35
				unpackCb(step, fmt.Sprintf("Установка компонента: %s", filepath.Base(exePath)))
			}

			var args []string
			lowerPath := strings.ToLower(exePath)
			if strings.Contains(lowerPath, "2010") || strings.Contains(lowerPath, "2012") {
				args = []string{"/q", "/norestart"}
			} else {
				args = []string{"/install", "/quiet", "/norestart"}
			}

			if installErr := RunWineInstaller(gameRoot, exePath, args...); installErr != nil {
				slog.Warn("Сбой установки компонента", "exe", filepath.Base(exePath), "err", installErr)
			}
		}
	} else {
		slog.Info("Библиотеки (redists) не найдены в status file, пропускаем установку.")
	}

	slog.Info("Префикс успешно распакован и настроен")
	if unpackCb != nil {
		unpackCb(1.0, "Установка префикса завершена")
	}

	return nil
}

func RunWineInstaller(gameRoot string, exePath string, silentArgs ...string) error {
	exeName := filepath.Base(exePath)
	slog.Info("Подготовка к запуску установщика", "exe", exeName)

	// =========================================================================
	// 1. БАЗОВЫЕ ПУТИ PROTON
	// =========================================================================
	protonDir := filepath.Join(gameRoot, "wine", "proton", "files")
	wineBinDir := filepath.Join(protonDir, "bin")
	wineLibDir := filepath.Join(protonDir, "lib")
	wineLib64Dir := filepath.Join(protonDir, "lib64")
	wineShareDir := filepath.Join(protonDir, "share", "wine")
	prefixPath := filepath.Join(gameRoot, "wine", "prefix", "pfx")

	// Возвращаемся к стандартному скрипту wine, он лучше определяет архитектуру
	wineBin := filepath.Join(wineBinDir, "wine")
	if _, err := os.Stat(wineBin); os.IsNotExist(err) {
		wineBin = filepath.Join(wineBinDir, "wine64") // Фоллбэк
	}
	wineServer := filepath.Join(wineBinDir, "wineserver")

	if _, err := os.Stat(wineBin); os.IsNotExist(err) {
		return fmt.Errorf("wine не найден: %s", wineBin)
	}
	if _, err := os.Stat(exePath); os.IsNotExist(err) {
		return fmt.Errorf("установщик не найден: %s", exePath)
	}

	// =========================================================================
	// 2. ИДЕАЛЬНОЕ ОКРУЖЕНИЕ PROTON (С ФИКСИРОВАННЫМ WINEDLLPATH)
	// =========================================================================
	// КРИТИЧЕСКИ ВАЖНО: Добавлены папки x86_64-windows и i386-windows!
	wineDllPath := strings.Join([]string{
		filepath.Join(wineLib64Dir, "wine", "x86_64-windows"),
		filepath.Join(wineLibDir, "wine", "x86_64-windows"),
		filepath.Join(wineLibDir, "wine", "i386-windows"),
		filepath.Join(wineLib64Dir, "wine"),
		filepath.Join(wineLibDir, "wine"),
	}, ":")

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
		"WINE_USE_NTSYNC=0", // Отключаем ntsync на время установки для стабильности
		// WINEDEBUG=-all УБРАНО! Теперь мы увидим реальные ошибки Wine в логах.
	)

	// =========================================================================
	// 3. ЭЛЕГАНТНЫЙ ЗАПУСК ЧЕРЕЗ WINE START /UNIX
	// =========================================================================
	// Команда start /wait /unix берет на себя всю трансляцию путей и дожидается конца установки.
	cmdArgs := []string{"start", "/wait", "/unix", exePath}
	cmdArgs = append(cmdArgs, silentArgs...)

	cmd := utils.NewHostCommand(env, wineBin, cmdArgs...)
	cmd.Dir = prefixPath

	slog.Info("Запуск установщика Wine", "exe", exeName, "args", silentArgs)

	out, err := cmd.CombinedOutput()
	if err != nil {
		slog.Error("Сбой установки компонента", "exe", exeName, "err", err, "out", string(out))
		return fmt.Errorf("ошибка при установке %s: %w\nВывод Wine:\n%s", exeName, err, string(out))
	}

	slog.Info("Компонент успешно установлен", "exe", exeName, "out", string(out))
	return nil
}
