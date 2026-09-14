package utils

import (
	"archive/tar"
	"archive/zip"
	"bufio"
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/bodgit/sevenzip"
)

const systemConfigPath = "~/.config/rfad-launcher/launcher.conf"

func ExtractArchive(archivePath, destDir string, progressCb func(float64, string)) error {
	switch {
	case strings.HasSuffix(archivePath, ".tar.gz"):
		return extractTarGz(archivePath, destDir, progressCb)
	case strings.HasSuffix(archivePath, ".7z"):
		return extract7z(archivePath, destDir, progressCb)
	case strings.HasSuffix(archivePath, ".zip"):
		return extractZip(archivePath, destDir, progressCb)
	default:
		return fmt.Errorf("неподдерживаемый формат архива: %s", filepath.Ext(archivePath))
	}
}

func GetDownloadedPath(destDir, key string) (string, error) {
	statusFile := filepath.Join(destDir, "download_status.txt")
	f, err := os.Open(statusFile)
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}
		return "", err
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	prefix := "complete " + key + " "
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, prefix) {
			// Формат: "complete <key> <path>"
			path := strings.TrimPrefix(line, prefix)
			return path, nil
		}
	}
	return "", scanner.Err()
}

// expandHome заменяет ведущую тильду (~) на реальный путь к домашней директории пользователя
func expandHome(path string) (string, error) {
	if strings.HasPrefix(path, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		return filepath.Join(home, path[2:]), nil
	}
	return path, nil
}

// GetOneSetting считывает конкретный ключ из центрального файла конфигурации
func GetOneSetting(key string) (string, error) {
	configPath, err := expandHome(systemConfigPath)
	if err != nil {
		return "", err
	}

	f, err := os.Open(configPath)
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}
		return "", err
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	prefix := key + " "
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, prefix) {
			val := strings.TrimPrefix(line, prefix)
			return val, nil
		}
	}
	return "", scanner.Err()
}

// SetOneSetting принимает value любого типа (interface{}),
// конвертирует его в строку и перезаписывает значение в конфиге.
func SetOneSetting(key string, value interface{}) error {
	configPath, err := expandHome(systemConfigPath)
	if err != nil {
		return err
	}

	// Убеждаемся, что директория ~/.config/rfad-launcher существует
	dir := filepath.Dir(configPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("ошибка создания директории конфига: %w", err)
	}

	prefix := key + " "
	strValue := fmt.Sprintf("%v", value)

	var lines []string
	keyFound := false

	// Читаем исходный файл, если он существует
	f, err := os.Open(configPath)
	if err == nil {
		scanner := bufio.NewScanner(f)
		for scanner.Scan() {
			line := scanner.Text()
			if strings.HasPrefix(line, prefix) {
				lines = append(lines, prefix+strValue)
				keyFound = true
			} else {
				lines = append(lines, line)
			}
		}
		f.Close()

		if err := scanner.Err(); err != nil {
			return fmt.Errorf("ошибка чтения файла: %w", err)
		}
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("ошибка открытия файла: %w", err)
	}

	// Если ключа не было, добавляем его в конец
	if !keyFound {
		lines = append(lines, prefix+strValue)
	}

	// Перезаписываем файл
	output := strings.Join(lines, "\n") + "\n"
	err = os.WriteFile(configPath, []byte(output), 0644)
	if err != nil {
		return fmt.Errorf("ошибка записи файла: %w", err)
	}

	return nil
}

type LauncherConfig struct {
	LinuxPatchComplete bool   `json:"linuxPatchComplete"`
	MangoHud           bool   `json:"mangoHud"`
	FSR                bool   `json:"fsr"`
	ShaderCache        bool   `json:"shaderCache"`
	HDR                bool   `json:"hdr"`
	SteamFix           bool   `json:"steamFix"`
	FpsLimit           string `json:"fpsLimit"`
	WineDllOverrides   string `json:"wineDllOverrides"`
	GrafikMod          string `json:"grafikMod"`
	FsrLvl             string `json:"fsrLvl"`
}

func parseBool(val string) bool {
	return strings.ToLower(val) == "true"
}

func GetLauncherConfig() (*LauncherConfig, error) {
	configPath, err := expandHome(systemConfigPath)
	if err != nil {
		return &LauncherConfig{}, err
	}

	file, err := os.Open(configPath)
	if err != nil {
		return &LauncherConfig{}, err
	}
	defer file.Close()

	cfg := &LauncherConfig{}
	scanner := bufio.NewScanner(file)

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		// ИСПРАВЛЕНИЕ: Теперь разделяем по ПЕРВОМУ ПРОБЕЛУ, а не по двоеточию
		parts := strings.SplitN(line, " ", 2)
		if len(parts) != 2 {
			continue
		}

		key := strings.TrimSpace(parts[0])
		val := strings.TrimSpace(parts[1])
		val = strings.Trim(val, `"'`)

		switch key {
		case "linux-patch-complite":
			cfg.LinuxPatchComplete = parseBool(val)
		case "MangoHud":
			cfg.MangoHud = parseBool(val)
		case "FSR":
			cfg.FSR = parseBool(val)
		case "ShaderCache":
			cfg.ShaderCache = parseBool(val)
		case "HDR":
			cfg.HDR = parseBool(val)
		case "SteamFix":
			cfg.SteamFix = parseBool(val)
		case "FpsLimit":
			cfg.FpsLimit = val
		case "WineDllOverrides":
			cfg.WineDllOverrides = val
		case "GrafikMod":
			cfg.GrafikMod = val
		case "FsrLvl":
			cfg.FsrLvl = val
		}
	}

	if err := scanner.Err(); err != nil {
		return cfg, err
	}

	return cfg, nil
}

func extractTarGz(archivePath, targetDir string, progressCb func(float64, string)) error {
	fmt.Printf("Открытие tar.gz архива: %s\n", archivePath)
	f, err := os.Open(archivePath)
	if err != nil {
		return fmt.Errorf("не удалось открыть архив: %v", err)
	}
	defer f.Close()

	gzr, err := gzip.NewReader(f)
	if err != nil {
		return fmt.Errorf("не удалось создать gzip-ридер: %v", err)
	}
	tr := tar.NewReader(gzr)

	var totalFiles int
	for {
		_, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			gzr.Close()
			return fmt.Errorf("ошибка чтения tar (первый проход): %v", err)
		}
		totalFiles++
	}
	gzr.Close()

	fmt.Printf("tar.gz содержит %d записей, начало распаковки...\n", totalFiles)

	f.Seek(0, 0)
	gzr, err = gzip.NewReader(f)
	if err != nil {
		return fmt.Errorf("не удалось пересоздать gzip-ридер: %v", err)
	}
	defer gzr.Close()
	tr = tar.NewReader(gzr)

	var currentIndex int
	for {

		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("ошибка чтения файла из tar: %v", err)
		}

		currentIndex++
		targetPath := filepath.Join(targetDir, header.Name)

		if !filepath.HasPrefix(targetPath, filepath.Clean(targetDir)+string(os.PathSeparator)) {
			return fmt.Errorf("некорректный путь в архиве: %s", header.Name)
		}

		switch header.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(targetPath, os.FileMode(header.Mode)); err != nil {
				return err
			}
		case tar.TypeReg, tar.TypeRegA:
			if err := os.MkdirAll(filepath.Dir(targetPath), 0755); err != nil {
				return err
			}
			outFile, err := os.OpenFile(targetPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, os.FileMode(header.Mode))
			if err != nil {
				return err
			}
			if _, err := io.Copy(outFile, tr); err != nil {
				outFile.Close()
				return err
			}
			outFile.Close()
		case tar.TypeSymlink:
			if err := os.MkdirAll(filepath.Dir(targetPath), 0755); err != nil {
				return err
			}
			if err := os.Symlink(header.Linkname, targetPath); err != nil {
				return err
			}
		default:
			continue
		}

		if progressCb != nil {
			percent := float64(currentIndex) / float64(totalFiles)
			cleanName := sanitize(filepath.Base(header.Name))
			if len(cleanName) > 40 {
				cleanName = "..." + cleanName[len(cleanName)-37:]
			}
			progressCb(percent, fmt.Sprintf("Распаковка: %s", cleanName))
		}
	}

	fmt.Printf("Распаковка tar.gz %s завершена успешно\n", archivePath)
	return nil
}

// extract7z - нативная функция распаковки 7z архивов
func extract7z(archivePath, targetDir string, progressCb func(float64, string)) error {
	fmt.Printf("Открытие 7z архива: %s", archivePath)

	r, err := sevenzip.OpenReader(archivePath)
	if err != nil {
		fmt.Printf("extract7z: ошибка открытия 7z: %v", err)
		return fmt.Errorf("не удалось открыть 7z архив: %v", err)
	}
	defer r.Close()

	totalFiles := len(r.File)
	fmt.Printf("7z содержит %d файлов, начало распаковки...", totalFiles)

	for i, f := range r.File {
		fpath := filepath.Join(targetDir, f.Name)

		if !filepath.HasPrefix(fpath, filepath.Clean(targetDir)+string(os.PathSeparator)) && fpath != filepath.Clean(targetDir) {
			return fmt.Errorf("обнаружен некорректный путь в архиве: %s", f.Name)
		}

		if f.FileInfo().IsDir() {
			os.MkdirAll(fpath, os.ModePerm)
			continue
		}

		if err := os.MkdirAll(filepath.Dir(fpath), os.ModePerm); err != nil {
			return err
		}

		dstFile, err := os.OpenFile(fpath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, f.Mode())
		if err != nil {
			return err
		}

		srcFile, err := f.Open()
		if err != nil {
			dstFile.Close()
			return err
		}

		_, err = io.Copy(dstFile, srcFile)
		srcFile.Close()
		dstFile.Close()
		if err != nil {
			return err
		}

		if progressCb != nil {
			percent := float64(i+1) / float64(totalFiles)
			cleanName := sanitize(filepath.Base(f.Name))
			if len(cleanName) > 40 {
				cleanName = "..." + cleanName[len(cleanName)-37:]
			}
			progressCb(percent, fmt.Sprintf("Распаковка: %s", cleanName))
		}
	}

	fmt.Printf("Распаковка 7z %s завершена успешно", archivePath)
	return nil
}

// extractZip - нативная функция распаковки ZIP архивов
func extractZip(archivePath, targetDir string, progressCb func(float64, string)) error {
	fmt.Printf("Открытие ZIP-архива: %s", archivePath)
	r, err := zip.OpenReader(archivePath)
	if err != nil {
		fmt.Printf("extractZip: ошибка открытия ZIP: %v", err)
		return fmt.Errorf("не удалось открыть ZIP архив: %v", err)
	}
	defer r.Close()

	totalFiles := len(r.File)
	fmt.Printf("ZIP содержит %d файлов, начало распаковки...", totalFiles)

	for i, f := range r.File {
		fpath := filepath.Join(targetDir, f.Name)

		if !filepath.HasPrefix(fpath, filepath.Clean(targetDir)+string(os.PathSeparator)) && fpath != filepath.Clean(targetDir) {
			return fmt.Errorf("обнаружен некорректный путь в архиве: %s", f.Name)
		}

		if f.FileInfo().IsDir() {
			os.MkdirAll(fpath, os.ModePerm)
			continue
		}

		if err := os.MkdirAll(filepath.Dir(fpath), os.ModePerm); err != nil {
			return err
		}

		dstFile, err := os.OpenFile(fpath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, f.Mode())
		if err != nil {
			return err
		}

		srcFile, err := f.Open()
		if err != nil {
			dstFile.Close()
			return err
		}

		_, err = io.Copy(dstFile, srcFile)
		srcFile.Close()
		dstFile.Close()
		if err != nil {
			return err
		}

		if progressCb != nil {
			percent := float64(i+1) / float64(totalFiles)
			cleanName := sanitize(filepath.Base(f.Name))
			if len(cleanName) > 40 {
				cleanName = "..." + cleanName[len(cleanName)-37:]
			}
			progressCb(percent, fmt.Sprintf("Распаковка: %s", cleanName))
		}
	}
	return nil
}

func sanitize(s string) string {
	reg := regexp.MustCompile(`[^a-zA-Z0-9./\\ \-_]`)
	return reg.ReplaceAllString(s, "")
}

func CopyPath(src, dst string) error {
	info, err := os.Stat(src)
	if err != nil {
		return err
	}
	if info.IsDir() {
		return CopyDir(src, dst)
	}
	return CopyFile(src, dst)
}

// copyFile безопасно копирует один файл
func CopyFile(src, dst string) error {
	if err := os.Link(src, dst); err == nil {
		return nil
	}

	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()

	if _, err = io.Copy(out, in); err != nil {
		return err
	}

	if err := out.Sync(); err != nil {
		return err
	}

	if info, err := os.Stat(src); err == nil {
		os.Chmod(dst, info.Mode())
	}

	return nil
}

// copyDir рекурсивно копирует папку со всем ее содержимым
func CopyDir(src, dst string) error {
	info, err := os.Stat(src)
	if err != nil {
		return fmt.Errorf("ошибка получения инфо о папке %s: %w", src, err)
	}

	if err := os.MkdirAll(dst, info.Mode()); err != nil {
		return fmt.Errorf("ошибка создания папки %s: %w", dst, err)
	}

	entries, err := os.ReadDir(src)
	if err != nil {
		return fmt.Errorf("ошибка чтения папки %s: %w", src, err)
	}

	for _, entry := range entries {
		srcPath := filepath.Join(src, entry.Name())
		dstPath := filepath.Join(dst, entry.Name())

		if entry.IsDir() {
			if err := CopyDir(srcPath, dstPath); err != nil {
				return err
			}
		} else {
			// Наша новая CopyFile сама разберется, как копировать файл быстрее всего
			if err := CopyFile(srcPath, dstPath); err != nil {
				return fmt.Errorf("ошибка копирования файла %s в %s: %w", srcPath, dstPath, err)
			}
		}
	}
	return nil
}
