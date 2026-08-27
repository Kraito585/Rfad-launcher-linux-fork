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

type progressWriter struct {
	Total      int64
	Downloaded int64
	Callback   func(float64)
}

func (pw *progressWriter) Write(p []byte) (int, error) {
	n := len(p)
	pw.Downloaded += int64(n)
	if pw.Total > 0 && pw.Callback != nil {
		pw.Callback(float64(pw.Downloaded) / float64(pw.Total))
	}
	return n, nil
}

func init() {
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	})))
}

func DownloadURL(ctx context.Context, url, destDir string, progressCb func(float64)) (string, error) {
	if err := os.MkdirAll(destDir, 0755); err != nil {
		return "", err
	}
	name := filepath.Base(url)
	destPath := filepath.Join(destDir, name)

	// Создаём временный файл в той же папке
	tmpPath := destPath + ".tmp"
	out, err := os.Create(tmpPath)
	if err != nil {
		return "", err
	}
	defer func() {
		out.Close()
		if err != nil {
			os.Remove(tmpPath)
		}
	}()

	req, _ := http.NewRequestWithContext(ctx, "GET", url, nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return "", fmt.Errorf("http %d", resp.StatusCode)
	}

	pw := &pw{total: resp.ContentLength, cb: progressCb}
	_, err = io.Copy(out, io.TeeReader(resp.Body, pw))
	if err != nil {
		return "", err
	}

	if err := os.Rename(tmpPath, destPath); err != nil {
		return "", err
	}
	return destPath, nil
}

func DownloadDriveFolder(ctx context.Context, creds []byte, folderID, destDir string, progressCb func(float64)) ([]string, error) {
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
	for _, f := range files {
		select {
		case <-ctx.Done():
			return savedPaths, ctx.Err()
		default:
		}
		local := filepath.Join(destDir, f.relPath)
		os.MkdirAll(filepath.Dir(local), 0755)
		if err := downloadOne(srv, f.id, local); err != nil {
			return savedPaths, err
		}
		savedPaths = append(savedPaths, local)
		downloaded += f.size
		if progressCb != nil && total > 0 {
			progressCb(float64(downloaded) / float64(total))
		}
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

func downloadOne(srv *drive.Service, id, path string) error {
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

	_, err = io.Copy(out, resp.Body)
	if err != nil {
		return err
	}

	return os.Rename(tmpPath, path)
}

type pw struct {
	total int64
	got   int64
	cb    func(float64)
}

func (p *pw) Write(b []byte) (int, error) {
	n := len(b)
	p.got += int64(n)
	if p.total > 0 && p.cb != nil {
		p.cb(float64(p.got) / float64(p.total))
	}
	return n, nil
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
		// fallback: извлечь имя из публичной ссылки (последняя часть)
		parts := strings.Split(publicURL, "/")
		result.Name = parts[len(parts)-1]
		// если это папка, то имя может быть не файлом, но мы попробуем
	}
	return result.Href, result.Name, nil
}

// DownloadYandex скачивает файл с Яндекс.Диска по публичной ссылке,
// сохраняя его с правильным именем.
func DownloadYandex(ctx context.Context, publicURL, destDir, fileName string, progressCb func(float64)) (string, error) {
	directURL, _, err := GetYandexDownloadURL(ctx, publicURL)
	if err != nil {
		return "", err
	}
	destPath := filepath.Join(destDir, fileName)
	err = downloadFileToPath(ctx, directURL, destPath, progressCb)
	if err != nil {
		return "", err
	}
	return destPath, nil
}

// downloadFileToPath скачивает файл по URL и сохраняет в заданный путь.
func downloadFileToPath(ctx context.Context, url, destPath string, progressCb func(float64)) error {
	// Создаём папку для файла, если её нет
	if err := os.MkdirAll(filepath.Dir(destPath), 0755); err != nil {
		return err
	}
	// Открываем или создаём файл
	out, err := os.Create(destPath)
	if err != nil {
		return err
	}
	defer out.Close()

	// Делаем HTTP-запрос
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

	// Копируем с прогрессом
	pw := &progressWriter{
		Total:    resp.ContentLength,
		Callback: progressCb,
	}
	_, err = io.Copy(out, io.TeeReader(resp.Body, pw))
	return err
}
