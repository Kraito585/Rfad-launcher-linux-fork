package graficswitch

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"rfad-launcher-linux/src-wails/downloader"
	"rfad-launcher-linux/src-wails/utils"
	fsrswitch "rfad-launcher-linux/src-wails/utils/fsr_switch"
	"strings"
)

// SwitchGrafikMod управляет переключением между CommunityShader, ENB, ReShade и т.д.
func SwitchGrafikMod(ctx context.Context, gameRoot string, newMod string, unpackCb func(float64, string), downloadCb func(float64, float64, string)) error {
	if unpackCb != nil {
		unpackCb(0.05, "Очистка от старых графических модов...")
	}

	cleanRootFromGraphicsMods(gameRoot)
	clearShaderCache(gameRoot)

	if newMod == "CommunityShader" {
		if unpackCb != nil {
			unpackCb(0.1, "Настройка Community Shaders...")
		}

		// Передаем оба коллбэка
		csFolder, upFolder, err := installCommunityShadersIfNeeded(ctx, gameRoot, unpackCb, downloadCb)
		if err != nil {
			return err
		}

		if err := toggleCSInModlist(gameRoot, true, csFolder, upFolder); err != nil {
			return fmt.Errorf("ошибка обновления modlist.txt: %w", err)
		}

	} else {
		if err := toggleCSInModlist(gameRoot, false, "", ""); err != nil {
			slog.Warn("Не удалось отключить Community Shaders", "err", err)
		}

		if newMod != "Нету" && newMod != "None" && newMod != "" {
			if unpackCb != nil {
				unpackCb(0.5, fmt.Sprintf("Установка пресета %s...", newMod))
			}

			modSourcePath := filepath.Join(gameRoot, "disabledGameFiles", "GraphicFiles", newMod)

			if _, err := os.Stat(modSourcePath); !os.IsNotExist(err) {
				if err := copyDir(modSourcePath, gameRoot); err != nil {
					return fmt.Errorf("ошибка при копировании файлов мода %s: %w", newMod, err)
				}
			} else {
				slog.Warn("Папка с графическим модом не найдена", "path", modSourcePath)
			}
		}
	}

	if unpackCb != nil {
		unpackCb(0.9, "Адаптация разрешения и FSR...")
	}

	// Синхронизация FSR (как и было)
	if err := fsrswitch.SyncFSRSettings(ctx, gameRoot, newMod); err != nil {
		slog.Warn("Не удалось синхронизовать FSR при смене мода", "error", err)
	}

	if unpackCb != nil {
		unpackCb(100.0, "Графический мод успешно изменён")
	}

	return nil
}

// installCommunityShadersIfNeeded проверяет наличие модов в MO2, и если их нет — находит/качает архивы и распаковывает

func installCommunityShadersIfNeeded(ctx context.Context, gameRoot string, unpackCb func(float64, string), downloadCb func(float64, float64, string)) (string, string, error) {
	modsDir := filepath.Join(gameRoot, "MO2", "mods")
	downloadDir := filepath.Join(gameRoot, "download")

	var csFolder, upFolder string
	hasCS, hasUp := false, false

	entries, _ := os.ReadDir(modsDir)
	for _, e := range entries {
		if e.IsDir() {
			nameLower := strings.ToLower(e.Name())
			if strings.Contains(nameLower, "community shaders") || strings.Contains(nameLower, "communityshader") {
				hasCS = true
				csFolder = e.Name()
			}
			if strings.Contains(nameLower, "upscaling") {
				hasUp = true
				upFolder = e.Name()
			}
		}
	}

	if hasCS && hasUp {
		return csFolder, upFolder, nil
	}

	csArchive := findArchiveByContains(downloadDir, "Community")
	upArchive := findArchiveByContains(downloadDir, "Upscal")

	if csArchive == "" || upArchive == "" {
		if unpackCb != nil {
			unpackCb(0.2, "Загрузка архивов Community Shaders...")
		}

		// Теперь мы напрямую передаем чистый downloadCb, так как он поддерживает скорость
		if err := downloader.DownloadCommunityShaders(ctx, gameRoot, false, downloadCb); err != nil {
			return "", "", fmt.Errorf("ошибка загрузки CS: %w", err)
		}

		csArchive = findArchiveByContains(downloadDir, "Community")
		upArchive = findArchiveByContains(downloadDir, "Upscal")
		if csArchive == "" || upArchive == "" {
			return "", "", fmt.Errorf("архивы CS не найдены даже после загрузки")
		}
	}

	if !hasCS && csArchive != "" {
		if unpackCb != nil {
			unpackCb(0.5, "Распаковка Community Shaders...")
		}
		folder, err := extractModToMO2(ctx, csArchive, modsDir, unpackCb)
		if err != nil {
			return "", "", err
		}
		csFolder = folder
	}

	if !hasUp && upArchive != "" {
		if unpackCb != nil {
			unpackCb(0.7, "Распаковка Upscaler...")
		}
		folder, err := extractModToMO2(ctx, upArchive, modsDir, unpackCb)
		if err != nil {
			return "", "", err
		}
		upFolder = folder
	}

	return csFolder, upFolder, nil
}

// toggleCSInModlist включает или выключает CS в профиле MO2
func toggleCSInModlist(gameRoot string, enable bool, csFolder, upFolder string) error {
	modlistPath := filepath.Join(gameRoot, "MO2", "profiles", "RFAD_SE", "modlist.txt")
	data, err := os.ReadFile(modlistPath)
	if err != nil {
		return err // Если файла нет, MO2 не настроен
	}

	lines := strings.Split(string(data), "\n")
	csFound, upFound := false, false

	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}

		// Ищем Community Shaders (сравниваем без учета регистра)
		lowerLine := strings.ToLower(trimmed)
		if strings.Contains(lowerLine, "community shaders") || strings.Contains(lowerLine, "communityshader") {
			csFound = true
			if enable && strings.HasPrefix(trimmed, "-") {
				lines[i] = "+" + trimmed[1:]
			} else if !enable && strings.HasPrefix(trimmed, "+") {
				lines[i] = "-" + trimmed[1:]
			}
		}

		// Ищем Upscaling
		if strings.Contains(lowerLine, "upscaling") {
			upFound = true
			if enable && strings.HasPrefix(trimmed, "-") {
				lines[i] = "+" + trimmed[1:]
			} else if !enable && strings.HasPrefix(trimmed, "+") {
				lines[i] = "-" + trimmed[1:]
			}
		}
	}

	// Если мы ВКЛЮЧАЕМ мод, и строчек в файле вообще не было (чистая установка мода),
	// добавляем их в самое начало файла (сразу после комментария)
	if enable {
		var toInsert []string
		if !csFound && csFolder != "" {
			toInsert = append(toInsert, "+"+csFolder)
		}
		if !upFound && upFolder != "" {
			toInsert = append(toInsert, "+"+upFolder)
		}

		if len(toInsert) > 0 {
			// Вставляем новые строки под заголовком MO2 (индекс 1)
			lines = append(lines[:1], append(toInsert, lines[1:]...)...)
		}
	}

	return os.WriteFile(modlistPath, []byte(strings.Join(lines, "\n")), 0644)
}

// === ВСПОМОГАТЕЛЬНЫЕ ФУНКЦИИ ===

func findArchiveByContains(dir, pattern string) string {
	pattern = strings.ToLower(pattern)
	entries, _ := os.ReadDir(dir)
	for _, e := range entries {
		if !e.IsDir() {
			name := strings.ToLower(e.Name())
			// Проверяем все возможные форматы архивов, включая .tar.gz и .gz
			if strings.HasSuffix(name, ".zip") ||
				strings.HasSuffix(name, ".7z") ||
				strings.HasSuffix(name, ".rar") ||
				strings.HasSuffix(name, ".tar.gz") ||
				strings.HasSuffix(name, ".gz") ||
				strings.HasSuffix(name, ".tar") {

				if strings.Contains(name, pattern) {
					return filepath.Join(dir, e.Name())
				}
			}
		}
	}
	return ""
}

func extractModToMO2(ctx context.Context, archivePath, modsDir string, unpackCb func(float64, string)) (string, error) {
	tempDir := filepath.Join(modsDir, "temp_extract_"+filepath.Base(archivePath))
	os.MkdirAll(tempDir, 0755)
	defer os.RemoveAll(tempDir)

	if err := utils.ExtractArchive(ctx, archivePath, tempDir, unpackCb); err != nil {
		return "", fmt.Errorf("сбой экстрактора: %w", err)
	}

	entries, err := os.ReadDir(tempDir)
	if err != nil {
		return "", err
	}

	// Очищаем имя архива от всех возможных расширений (включая двойные вроде .tar.gz)
	archiveName := filepath.Base(archivePath)
	exts := []string{".tar.gz", ".zip", ".7z", ".rar", ".gz", ".tar"}
	for _, ext := range exts {
		if strings.HasSuffix(strings.ToLower(archiveName), ext) {
			archiveName = archiveName[:len(archiveName)-len(ext)]
			break
		}
	}

	var modFolderName string

	// Проверяем структуру: есть ли внутри ровно одна папка?
	if len(entries) == 1 && entries[0].IsDir() {
		dirName := entries[0].Name()
		dirLower := strings.ToLower(dirName)

		// Список стандартных папок движка игры. Если папка называется так, обертки в архиве нет.
		isDataDir := dirLower == "skse" || dirLower == "meshes" ||
			dirLower == "textures" || dirLower == "scripts" ||
			dirLower == "sound" || dirLower == "interface" ||
			dirLower == "mwse" || dirLower == "obse" ||
			dirLower == "f4se" || dirLower == "nvse" ||
			dirLower == "data" || dirLower == "plugins" ||
			dirLower == "shaders"

		if isDataDir {
			// Это не папка-обертка. Берем имя из названия самого архива.
			modFolderName = archiveName
			target := filepath.Join(modsDir, modFolderName)
			os.RemoveAll(target)
			if err := os.Rename(tempDir, target); err != nil {
				return "", fmt.Errorf("ошибка перемещения папки мода: %w", err)
			}
		} else {
			// Это действительно папка-обертка (например, "Community Shaders v1.0")
			modFolderName = dirName
			target := filepath.Join(modsDir, modFolderName)
			os.RemoveAll(target)
			if err := os.Rename(filepath.Join(tempDir, dirName), target); err != nil {
				return "", fmt.Errorf("ошибка перемещения папки мода: %w", err)
			}
		}
	} else {
		// Файлов несколько, лежат россыпью в корне (например, SKSE/ + Shaders/ + meta.ini)
		modFolderName = archiveName
		target := filepath.Join(modsDir, modFolderName)
		os.RemoveAll(target)
		if err := os.Rename(tempDir, target); err != nil {
			return "", fmt.Errorf("ошибка перемещения папки мода: %w", err)
		}
	}

	return modFolderName, nil
}

func cleanRootFromGraphicsMods(gameRoot string) {
	filesToRemove := []string{
		"d3d11.dll",
		"dxgi.dll",
		"enblocal.ini",
		"enbseries.ini",
		"enbseries",
		"reshade-shaders",
		"ReShade.ini",
		"ReShadePreset.ini",
		"dxgi.log",
	}

	for _, item := range filesToRemove {
		targetPath := filepath.Join(gameRoot, item)
		if err := os.RemoveAll(targetPath); err != nil {
			slog.Warn("Ошибка при удалении файла графического мода", "file", item, "err", err)
		}
	}
	slog.Info("Корень игры очищен от старых графических модов")
}
func copyDir(src string, dst string) error {
	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Вычисляем относительный путь
		relPath, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}

		targetPath := filepath.Join(dst, relPath)

		if info.IsDir() {
			return os.MkdirAll(targetPath, info.Mode())
		}

		// Если это файл, копируем его
		return copyFile(path, targetPath)
	})
}

// copyFile - простая утилита для физического копирования файла
func copyFile(src, dst string) error {
	sourceFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer sourceFile.Close()

	destFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer destFile.Close()

	_, err = io.Copy(destFile, sourceFile)
	return err
}

func clearShaderCache(gameRoot string) {
	prefixPath := filepath.Join(gameRoot, "wine", "prefix", "pfx")

	// 1. Папка кэша, которую мы задаем в start.sh
	customCachePath := filepath.Join(prefixPath, "shadercache")

	// 2. Дефолтная папка DXVK в Windows (та, что мелькает в логе)
	appDataDxvkPath := filepath.Join(prefixPath, "drive_c", "users", "steamuser", "AppData", "Local", "dxvk")

	pathsToClean := []string{customCachePath, appDataDxvkPath}

	for _, targetDir := range pathsToClean {
		// Сносим папку с кэшем и тут же создаем пустую
		if err := os.RemoveAll(targetDir); err == nil {
			os.MkdirAll(targetDir, 0755)
		}
	}

	// 3. На всякий случай ищем и удаляем файлы *.dxvk-cache в корне игры
	entries, err := os.ReadDir(gameRoot)
	if err == nil {
		for _, e := range entries {
			if !e.IsDir() && strings.HasSuffix(e.Name(), ".dxvk-cache") {
				os.Remove(filepath.Join(gameRoot, e.Name()))
			}
		}
	}
	slog.Info("Кэш шейдеров DXVK успешно очищен")
}
