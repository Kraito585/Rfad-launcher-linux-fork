package graficswitch

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"rfad-launcher-linux/src-wails/downloader"
	"rfad-launcher-linux/src-wails/utils"
	"strings"
)

// SwitchGrafikMod управляет переключением между CommunityShader, ENB, ReShade и т.д.
func SwitchGrafikMod(ctx context.Context, gameRoot string, newMod string, progressCb func(float64, string)) error {
	// 1. Очищаем корень игры от старых файлов ENB/ReShade
	// (Здесь должна быть логика удаления d3d11.dll, dxgi.dll и т.д. из корня игры)
	// cleanRootFromGraphicsMods(gameRoot)

	// 2. Логика для Community Shaders
	if newMod == "CommunityShader" {
		if progressCb != nil {
			progressCb(0.1, "Настройка Community Shaders...")
		}

		// Устанавливаем и получаем точные имена папок
		csFolder, upFolder, err := installCommunityShadersIfNeeded(ctx, gameRoot, progressCb)
		if err != nil {
			return err
		}

		// Включаем их в modlist.txt
		if err := toggleCSInModlist(gameRoot, true, csFolder, upFolder); err != nil {
			return fmt.Errorf("ошибка обновления modlist.txt: %w", err)
		}

	} else {
		// 3. Если выбрали другой мод (или "Нету") — выключаем Community Shaders
		if err := toggleCSInModlist(gameRoot, false, "", ""); err != nil {
			slog.Warn("Не удалось отключить Community Shaders", "err", err)
		}

		// 4. Копируем файлы нового мода из disabledGameFiles
		// Например: copyPath(filepath.Join(gameRoot, "disabledGameFiles", "GrafikMods", newMod), gameRoot)
	}

	if progressCb != nil {
		progressCb(1.0, "Графический мод успешно изменён")
	}

	return nil
}

// installCommunityShadersIfNeeded проверяет наличие модов в MO2, и если их нет — находит/качает архивы и распаковывает

func installCommunityShadersIfNeeded(ctx context.Context, gameRoot string, progressCb func(float64, string)) (string, string, error) {
	modsDir := filepath.Join(gameRoot, "MO2", "mods")
	downloadDir := filepath.Join(gameRoot, "download")

	var csFolder, upFolder string
	hasCS, hasUp := false, false

	// 1. Проверяем, установлены ли моды в MO2/mods
	entries, _ := os.ReadDir(modsDir)
	for _, e := range entries {
		if e.IsDir() {
			if strings.Contains(strings.ToLower(e.Name()), "community shaders") || strings.Contains(strings.ToLower(e.Name()), "communityshader") {
				hasCS = true
				csFolder = e.Name()
			}
			if strings.Contains(strings.ToLower(e.Name()), "upscaling") {
				hasUp = true
				upFolder = e.Name()
			}
		}
	}

	// Если оба мода уже установлены, просто возвращаем их имена
	if hasCS && hasUp {
		return csFolder, upFolder, nil
	}

	// 2. Ищем архивы в папке download (т.к. точное имя неизвестно, ищем по вхождению)
	csArchive := findArchiveByContains(downloadDir, "Community")
	upArchive := findArchiveByContains(downloadDir, "Upscal")

	// 3. Если архивов нет — вызываем загрузчик
	if csArchive == "" || upArchive == "" {
		if progressCb != nil {
			progressCb(0.2, "Загрузка архивов Community Shaders...")
		}

		// === АДАПТЕР КОЛЛБЭКА ===
		// Приводим функцию из 3 аргументов к 2, игнорируя скорость (speed)
		var downloadCb func(float64, float64, string)
		if progressCb != nil {
			downloadCb = func(p float64, speed float64, msg string) {
				progressCb(p, msg)
			}
		}

		// Передаем gameRoot (как требует новая сигнатура функции) и наш downloadCb
		if err := downloader.DownloadCommunityShaders(ctx, gameRoot, false, downloadCb); err != nil {
			return "", "", fmt.Errorf("ошибка загрузки CS: %w", err)
		}

		// Повторяем поиск после загрузки
		csArchive = findArchiveByContains(downloadDir, "Community")
		upArchive = findArchiveByContains(downloadDir, "Upscal")
		if csArchive == "" || upArchive == "" {
			return "", "", fmt.Errorf("архивы CS не найдены даже после загрузки")
		}
	}

	// 4. Распаковка архивов в MO2/mods
	if !hasCS && csArchive != "" {
		if progressCb != nil {
			progressCb(0.5, "Распаковка Community Shaders...")
		}
		folder, err := extractModToMO2(ctx, csArchive, modsDir, progressCb)
		if err != nil {
			return "", "", err
		}
		csFolder = folder
	}

	if !hasUp && upArchive != "" {
		if progressCb != nil {
			progressCb(0.7, "Распаковка Upscaler...")
		}
		folder, err := extractModToMO2(ctx, upArchive, modsDir, progressCb)
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
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".zip") {
			if strings.Contains(strings.ToLower(e.Name()), pattern) {
				return filepath.Join(dir, e.Name())
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
		return "", err
	}

	entries, err := os.ReadDir(tempDir)
	if err != nil {
		return "", err
	}

	var modFolderName string
	// Если внутри архива только одна папка, используем её как папку мода
	if len(entries) == 1 && entries[0].IsDir() {
		modFolderName = entries[0].Name()
		target := filepath.Join(modsDir, modFolderName)
		os.RemoveAll(target)
		os.Rename(filepath.Join(tempDir, modFolderName), target)
	} else {
		// Иначе создаем папку из названия архива
		modFolderName = strings.TrimSuffix(filepath.Base(archivePath), filepath.Ext(archivePath))
		target := filepath.Join(modsDir, modFolderName)
		os.RemoveAll(target)
		os.Rename(tempDir, target)
	}
	return modFolderName, nil
}
