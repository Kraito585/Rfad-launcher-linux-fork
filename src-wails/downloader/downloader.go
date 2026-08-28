// downloader/downloader.go
package downloader

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"google.golang.org/api/drive/v3"
	"google.golang.org/api/option"
)

const yandexAPIBase = "https://cloud-api.yandex.net/v1/disk/public/resources/download"

// Обновленная структура для подсчета глобального прогресса и скорости
type progressWriter struct {
	globalTotal int64
	globalBase  int64
	currentGot  int64
	lastTime    time.Time
	lastBytes   int64
	speed       float64
	msg         string
	cb          func(float64, float64, string)
}

func (p *progressWriter) Write(b []byte) (int, error) {
	n := len(b)
	p.currentGot += int64(n)
	totalGot := p.globalBase + p.currentGot

	now := time.Now()
	duration := now.Sub(p.lastTime).Seconds()

	if duration >= 0.5 {
		delta := float64(totalGot - p.lastBytes)
		p.speed = delta / duration
		p.lastTime = now
		p.lastBytes = totalGot
	}

	if p.globalTotal > 0 && p.cb != nil {
		p.cb(float64(totalGot)/float64(p.globalTotal), p.speed, p.msg)
	}
	return n, nil
}

func init() {
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	})))
}

func DownloadURL(ctx context.Context, downloadUrl, destDir string, progressCb func(float64, float64, string)) (destPath string, err error) {
	if err = os.MkdirAll(destDir, 0755); err != nil {
		return "", err
	}

	// Безопасно извлекаем имя файла (обрезаем параметры запроса, если они есть)
	name := filepath.Base(downloadUrl)
	if idx := strings.Index(name, "?"); idx != -1 {
		name = name[:idx]
	}
	destPath = filepath.Join(destDir, name)

	// Создаём временный файл
	tmpPath := destPath + ".tmp"
	out, err := os.Create(tmpPath)
	if err != nil {
		return "", err
	}

	// defer теперь безошибочно видит итоговый err благодаря именованным возвращаемым параметрам
	defer func() {
		out.Close()
		if err != nil {
			os.Remove(tmpPath)
		}
	}()

	req, err := http.NewRequestWithContext(ctx, "GET", downloadUrl, nil)
	if err != nil {
		return "", err
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		err = fmt.Errorf("http %d", resp.StatusCode)
		return "", err
	}

	// Инициализируем наш progressWriter
	pw := &progressWriter{
		globalTotal: resp.ContentLength,
		globalBase:  0,
		lastTime:    time.Now(),
		msg:         "Загрузка: " + name, // Передаем красивое имя в UI
		cb:          progressCb,
	}

	// Копируем с трекингом прогресса и скорости
	_, err = io.Copy(out, io.TeeReader(resp.Body, pw))
	if err != nil {
		return "", err
	}

	// Атомарно переименовываем .tmp в финальный файл
	if err = os.Rename(tmpPath, destPath); err != nil {
		return "", err
	}

	return destPath, nil
}

func DownloadDriveFolder(ctx context.Context, creds []byte, folderID, destDir string, progressCb func(float64, float64, string)) ([]string, error) {
	if err := os.MkdirAll(destDir, 0755); err != nil {
		return nil, err
	}
	srv, err := drive.NewService(ctx, option.WithCredentialsJSON(creds))
	if err != nil {
		return nil, err
	}

	var files []struct {
		relPath, id string
		size        int64
	}
	var total int64

	err = walk(srv, folderID, "", func(rel, id string, size int64) {
		files = append(files, struct {
			relPath, id string
			size        int64
		}{rel, id, size})
		total += size
	})
	if err != nil {
		return nil, err
	}

	var downloaded int64
	var savedPaths []string

	pw := &progressWriter{
		globalTotal: total,
		lastTime:    time.Now(),
		cb:          progressCb,
	}

	for _, f := range files {
		select {
		case <-ctx.Done():
			return savedPaths, ctx.Err()
		default:
		}
		local := filepath.Join(destDir, f.relPath)
		os.MkdirAll(filepath.Dir(local), 0755)

		pw.currentGot = 0
		// Красивое сообщение с именем файла для UI
		pw.msg = "Загрузка: " + filepath.Base(f.relPath)

		if err := downloadOne(srv, f.id, local, pw); err != nil {
			return savedPaths, err
		}
		savedPaths = append(savedPaths, local)

		downloaded += f.size
		pw.globalBase = downloaded
	}
	return savedPaths, nil
}

func walk(srv *drive.Service, folder, rel string, cb func(rel, id string, size int64)) error {
	page := ""
	for {
		q := srv.Files.List().Q(fmt.Sprintf("'%s' in parents", folder)).
			Fields("nextPageToken, files(id, name, mimeType, size)").PageToken(page)
		r, err := q.Do()
		if err != nil {
			return err
		}
		for _, f := range r.Files {
			if f.MimeType == "application/vnd.google-apps.folder" {
				if err := walk(srv, f.Id, filepath.Join(rel, f.Name), cb); err != nil {
					return err
				}
				continue
			}
			// Пропускаем Google Docs, Sheets, Slides и т.п.
			if strings.HasPrefix(f.MimeType, "application/vnd.google-apps.") {
				slog.Info("Skipping non-binary Google file", "name", f.Name, "mime", f.MimeType)
				continue
			}
			// Обычные бинарные файлы
			cb(filepath.Join(rel, f.Name), f.Id, f.Size)
		}
		page = r.NextPageToken
		if page == "" {
			break
		}
	}
	return nil
}

func downloadOne(srv *drive.Service, id, path string, pw io.Writer) error {
	tmpPath := path + ".tmp"
	out, err := os.Create(tmpPath)
	if err != nil {
		return err
	}
	defer func() {
		out.Close()
		if err != nil {
			os.Remove(tmpPath)
		}
	}()

	resp, err := srv.Files.Get(id).Download()
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	src := io.TeeReader(resp.Body, pw)

	_, err = io.Copy(out, src)
	if err != nil {
		return err
	}

	return os.Rename(tmpPath, path)
}

// GetYandexDownloadURL возвращает прямую ссылку и имя файла.
func GetYandexDownloadURL(ctx context.Context, publicURL string) (directURL, fileName string, err error) {
	req, err := http.NewRequestWithContext(ctx, "GET", yandexAPIBase+"?public_key="+url.QueryEscape(publicURL), nil)
	if err != nil {
		return "", "", fmt.Errorf("ошибка создания запроса к API Яндекс.Диска: %w", err)
	}
	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", "", fmt.Errorf("ошибка соединения с API Яндекс.Диска: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", "", fmt.Errorf("API Яндекс.Диска вернул статус %d", resp.StatusCode)
	}
	var result struct {
		Href string `json:"href"`
		Name string `json:"name"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", "", fmt.Errorf("ошибка парсинга ответа API Яндекс.Диска: %w", err)
	}
	if result.Href == "" {
		return "", "", fmt.Errorf("прямая ссылка не получена")
	}
	if result.Name == "" {
		parts := strings.Split(publicURL, "/")
		result.Name = parts[len(parts)-1]
	}
	return result.Href, result.Name, nil
}

func DownloadYandex(ctx context.Context, publicURL, destDir, fileName string, progressCb func(float64, float64, string)) (string, error) {
	directURL, _, err := GetYandexDownloadURL(ctx, publicURL)
	if err != nil {
		return "", err
	}
	destPath := filepath.Join(destDir, fileName)

	err = downloadFileToPath(ctx, directURL, destPath, fileName, progressCb)
	if err != nil {
		return "", err
	}
	return destPath, nil
}

func downloadFileToPath(ctx context.Context, url, destPath, fileName string, progressCb func(float64, float64, string)) error {
	if err := os.MkdirAll(filepath.Dir(destPath), 0755); err != nil {
		return err
	}

	out, err := os.Create(destPath)
	if err != nil {
		return err
	}
	defer func() {
		out.Close()

		if err != nil {
			os.Remove(destPath)
		}
	}()

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP ошибка: %d", resp.StatusCode)
	}

	pw := &progressWriter{
		globalTotal: resp.ContentLength,
		globalBase:  0,
		lastTime:    time.Now(),
		msg:         "Загрузка: " + fileName,
		cb:          progressCb,
	}

	_, err = io.Copy(out, io.TeeReader(resp.Body, pw))
	return err
}

// ======================
// загрузка пакетов
// ======================
//
// Вспомогательная фунция записи патчей загруженых файлов и константы файлов

const (
	UpdateFolderID            string = "1JUOctbsugh2IIEUCWcBkupXYVYoJMg4G"
	YandexPrefixURL           string = "https://disk.yandex.ru/d/y3mx1DXn83CbgQ"
	SteamFixID                string = "17BUNJj1akU-ktCOK7iDVErFMZJK6_sfQ"
	PrefixFileCDNURL          string = "https://mirror.kraito.ru/rfad/prefix/prefix.tar.gz"
	SteamFixCDNURL            string = "https://mirror.kraito.ru/rfad/SteamFix/SteamFix.zip"
	CommunityShaderURL        string = "https://mirror.kraito.ru/rfad/shaders/Community%20Shaders%2086492%201.7.3%202026-06-27T10-38Z%206Xybdafll.tar.gz"
	CommunityShaderUpsacleURL string = "https://mirror.kraito.ru/rfad/shaders/Upscaling%20156952%201.4.0%202026-05-31T10-27Z%20L5WQbqiov.tar.gz"
	GEProtonUrl               string = "https://github.com/GloriousEggroll/proton-ge-custom/releases/download/GE-Proton11-5/GE-Proton11-5-x86_64.tar.gz"
	ConfigPatchURL            string = "https://api.kraito.ru/api/v1/config"
	// InnoExtract      string = "https://github.com/dscharrer/innoextract/releases/download/1.9/innoextract-1.9-linux.tar.xz" unused
)

func alreadyDownloaded(destDir, key string) bool {
	statusFile := filepath.Join(destDir, "download_status.txt")

	data, err := os.ReadFile(statusFile)
	if err != nil {
		return false
	}

	lines := strings.Split(string(data), "\n")
	prefix := fmt.Sprintf("complete %s ", key)

	for _, line := range lines {
		if strings.HasPrefix(line, prefix) {
			return true
		}
	}
	return false
}

func rewriteStatus(destDir, key string, values ...string) error {
	statusFile := filepath.Join(destDir, "download_status.txt")

	var newLines []string

	// 1. Пытаемся прочитать текущий файл
	data, err := os.ReadFile(statusFile)
	if err == nil {
		lines := strings.Split(string(data), "\n")
		prefix := fmt.Sprintf("complete %s ", key)

		for _, line := range lines {
			trimmed := strings.TrimSpace(line)
			if trimmed == "" {
				continue
			}
			// Если строка НЕ начинается с нашего префикса, сохраняем её
			if !strings.HasPrefix(trimmed, prefix) {
				newLines = append(newLines, trimmed)
			}
		}
	}

	// 2. Добавляем новые значения в конец
	for _, val := range values {
		if val != "" {
			newLines = append(newLines, fmt.Sprintf("complete %s %s", key, val))
		}
	}

	// 3. Формируем итоговый текст (обязательно с переносом строки в конце,
	// чтобы следующий обычный WriteStatus через Append ничего не сломал)
	output := ""
	if len(newLines) > 0 {
		output = strings.Join(newLines, "\n") + "\n"
	}

	// 4. Перезаписываем файл целиком (O_TRUNC по умолчанию в os.WriteFile)
	return os.WriteFile(statusFile, []byte(output), 0644)
}

func removeDownloadedFiles(destDir, key string) error {
	statusFile := filepath.Join(destDir, "download_status.txt")

	data, err := os.ReadFile(statusFile)
	if err != nil {
		// Если статус-файла нет, значит и удалять нечего
		return nil
	}

	var newLines []string
	prefix := fmt.Sprintf("complete %s ", key)

	lines := strings.Split(string(data), "\n")
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}

		if strings.HasPrefix(trimmed, prefix) {
			// Извлекаем путь к файлу (всё, что идет после префикса)
			filePath := strings.TrimPrefix(trimmed, prefix)
			if filePath != "" {
				// os.RemoveAll безопасно удалит и файл, и папку, и не упадёт, если файла уже нет
				os.RemoveAll(filePath)
			}
		} else {
			// Строки от других ключей сохраняем
			newLines = append(newLines, trimmed)
		}
	}

	// Перезаписываем статус-файл без удаленных записей
	output := ""
	if len(newLines) > 0 {
		output = strings.Join(newLines, "\n") + "\n"
	}

	return os.WriteFile(statusFile, []byte(output), 0644)
}

func lastUpdateFromCDN(ctx context.Context) (string, error) {
	type UpdateData struct {
		ID        string `json:"id"`
		Version   string `json:"version"`
		URL       string `json:"url"`
		CreatedAt int64  `json:"created_at"`
	}

	type UpdateResponse struct {
		Success bool       `json:"success"`
		Data    UpdateData `json:"data"`
	}

	apiUrl := "https://api.kraito.ru/api/v1/updates/latest"

	req, err := http.NewRequestWithContext(ctx, "GET", apiUrl, nil)
	if err != nil {
		return "", err
	}

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("ошибка соединения с сервером: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("сервер вернул статус: %d", resp.StatusCode)
	}

	var apiResult UpdateResponse
	if err := json.NewDecoder(resp.Body).Decode(&apiResult); err != nil {
		return "", fmt.Errorf("ошибка чтения ответа API: %w", err)
	}

	if !apiResult.Success {
		return "", fmt.Errorf("API вернул success: false")
	}
	s3BaseURL := "https://mirror.kraito.ru/"
	return s3BaseURL + apiResult.Data.URL, nil
}

// Сами функции загрузки пакетов
func DownloadUpdate(ctx context.Context, gameRoot string, downloadType string, creds []byte, forceDownload bool, progressCb func(float64, float64, string)) error {
	destDir := filepath.Join(gameRoot, "download")
	key := "update"
	if !alreadyDownloaded(destDir, key) || forceDownload {
		removeDownloadedFiles(destDir, key)
		if downloadType == "cdn" {
			var path string
			url, err := lastUpdateFromCDN(ctx)
			path, err = DownloadURL(ctx, url, destDir, progressCb)
			if err != nil {
				return fmt.Errorf("ошибка получения обновления попробуйте использовать google drive %s", err)
			}
			return rewriteStatus(destDir, key, path)

		}
		var paths []string
		paths, err := DownloadDriveFolder(ctx, creds, UpdateFolderID, destDir, progressCb)
		if err != nil {
			return fmt.Errorf("ошибка получения обновления попробуйте использовать cdn(mirror) %s", err)
		}
		return rewriteStatus(destDir, key, paths...)
	}

	return nil
}

func DownloadPrefix(ctx context.Context, gameRoot string, downloadType string, creds []byte, forceDownload bool, progressCb func(float64, float64, string)) error {
	destDir := filepath.Join(gameRoot, "download")
	key := "prefix"
	if !alreadyDownloaded(destDir, key) || forceDownload {
		removeDownloadedFiles(destDir, key)
		if downloadType == "cdn" {
			var path string
			path, err := DownloadURL(ctx, PrefixFileCDNURL, destDir, progressCb)
			if err != nil {
				return fmt.Errorf("ошибка получения префикса wine попробуйте использовать google drive %s", err)
			}
			return rewriteStatus(destDir, key, path)
		}
		var path string
		path, err := DownloadYandex(ctx, YandexPrefixURL, destDir, "pfx dotnet.7z", progressCb)
		if err != nil {
			return fmt.Errorf("ошибка загрузки префикса с Яндекс.Диска: %w", err)
		}
		return rewriteStatus(destDir, key, path)
	}

	return nil
}

func DownloadSteamfix(ctx context.Context, gameRoot string, downloadType string, creds []byte, forceDownload bool, progressCb func(float64, float64, string)) error {
	destDir := filepath.Join(gameRoot, "download")
	key := "steamfix"
	if !alreadyDownloaded(destDir, key) || forceDownload {
		removeDownloadedFiles(destDir, key)
		if downloadType == "cdn" {
			var path string
			path, err := DownloadURL(ctx, SteamFixCDNURL, destDir, progressCb)
			if err != nil {
				return fmt.Errorf("ошибка получения префикса wine попробуйте использовать google drive %s", err)
			}
			return rewriteStatus(destDir, key, path)
		}
		var paths []string
		paths, err := DownloadDriveFolder(ctx, creds, SteamFixID, destDir, progressCb)
		if err != nil {
			return fmt.Errorf("ошибка получения префикса wine попробуйте использовать cdn(mirror) %s", err)
		}
		return rewriteStatus(destDir, key, paths...)
	}

	return nil
}

func DownloadGEProton(ctx context.Context, gameRoot string, forceDownload bool, progressCb func(float64, float64, string)) error {
	destDir := filepath.Join(gameRoot, "download")
	key := "GE-Proton"
	if !alreadyDownloaded(destDir, key) || forceDownload {
		removeDownloadedFiles(destDir, key)
		var path string
		path, err := DownloadURL(ctx, GEProtonUrl, destDir, progressCb)
		if err != nil {
			return fmt.Errorf("Ошибка загрузки GE-Proton повторите попытку позже %s", err)
		}
		return rewriteStatus(destDir, key, path)

	}
	return nil
}

func DownloadConfig(ctx context.Context, gameRoot string, forceDownload bool) error {
	destDir := filepath.Join(gameRoot, "download")
	key := "config"
	if !alreadyDownloaded(destDir, key) || forceDownload {
		removeDownloadedFiles(destDir, key)
		type ConfigData struct {
			Data []json.RawMessage `json:"data"`
		}

		type ConfigResponse struct {
			Success bool       `json:"success"`
			Data    ConfigData `json:"data"`
		}
		configFile := filepath.Join(destDir, "config.json")

		req, err := http.NewRequestWithContext(ctx, "GET", ConfigPatchURL, nil)
		if err != nil {
			return err
		}

		client := &http.Client{Timeout: 10 * time.Second}
		resp, err := client.Do(req)
		if err != nil {
			return fmt.Errorf("ошибка соединения с сервером: %w", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			return fmt.Errorf("сервер вернул статус: %d", resp.StatusCode)
		}

		var apiResult ConfigResponse
		if err := json.NewDecoder(resp.Body).Decode(&apiResult); err != nil {
			return fmt.Errorf("ошибка чтения ответа API: %w", err)
		}

		if !apiResult.Success {
			return fmt.Errorf("API вернул success: false")
		}

		cfgFile, err := os.Create(configFile)
		jsonData, err := json.MarshalIndent(apiResult.Data.Data, "", "    ")
		if err != nil {
			return fmt.Errorf("неудалось пропарсить конфиг с сервера: %w", err)
		}
		_, err = cfgFile.Write(jsonData)

		return rewriteStatus(destDir, key, configFile)
	}
	return nil
}

func DownloadCommunityShaders(ctx context.Context, gameRoot string, forceDownload bool, progressCb func(float64, float64, string)) error {
	destDir := filepath.Join(gameRoot, "download")
	key1 := "Community"
	key2 := "Upscal"
	if !alreadyDownloaded(destDir, key1) || forceDownload {
		removeDownloadedFiles(destDir, key1)
		var path string
		path, err := DownloadURL(ctx, CommunityShaderURL, destDir, progressCb)
		if err != nil {
			return fmt.Errorf("ошибка получения префикса wine попробуйте использовать google drive %s", err)
		}
		return rewriteStatus(destDir, key1, path)
	}
	if !alreadyDownloaded(destDir, key2) || forceDownload {
		removeDownloadedFiles(destDir, key2)
		var path string
		path, err := DownloadURL(ctx, CommunityShaderUpsacleURL, destDir, progressCb)
		if err != nil {
			return fmt.Errorf("ошибка получения префикса wine попробуйте использовать google drive %s", err)
		}
		return rewriteStatus(destDir, key2, path)
	}

	return nil
}
