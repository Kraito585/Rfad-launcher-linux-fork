package migrations

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"rfad-launcher-linux/src-wails/core"
	"rfad-launcher-linux/src-wails/downloader"
	"rfad-launcher-linux/src-wails/patches/prefix_install"
	"rfad-launcher-linux/src-wails/patches/proton_install"
	"rfad-launcher-linux/src-wails/utils"

	"github.com/wailsapp/wails/v3/pkg/application"
)

type Migration struct {
	ID  int
	Run func() error
}

func StartMigrations(
	gameRoot string,
	latestMigrationID, curentID int,
	libs, offlineConfig []byte,
	unpackCb func(float64, string),
	downloadCb func(float64, float64, string),
) error {
	migrations := []Migration{
		{
			ID: 1,
			Run: func() error {
				return migrateOne(gameRoot, libs, offlineConfig, unpackCb, downloadCb)
			},
		},
	}

	for _, m := range migrations {
		if curentID >= m.ID {
			continue
		}

		slog.Info("Выполнение миграции", "id", m.ID)
		if err := m.Run(); err != nil {
			return fmt.Errorf("миграция #%d завершилась с ошибкой: %w", m.ID, err)
		}

		// Сразу фиксируем успешный шаг в конфиге
		if err := utils.SetOneSetting("migrationVersion", m.ID); err != nil {
			return err
		}
		curentID = m.ID
	}

	return nil
}

func migrateOne(
	gameRoot string,
	libs, offlineConfig []byte,
	unpackCb func(float64, string),
	downloadCb func(float64, float64, string),
) error {
	slog.Info("Начата полная переустановка Proton/Wine и префикса")

	application.Get().Event.Emit("update-status", map[string]string{"status": "download-started"})

	err := downloader.DownloadWine(gameRoot, true, downloadCb)
	if err != nil {
		application.Get().Event.Emit("update-status", map[string]string{"status": "process-error"})
		return fmt.Errorf("ошибка загрузки proton: %w", err)
	}

	err = downloader.DownloadPrefix(gameRoot, true, downloadCb)
	if err != nil {
		application.Get().Event.Emit("update-status", map[string]string{"status": "process-error"})
		return fmt.Errorf("ошибка загрузки префикса: %w", err)
	}

	application.Get().Event.Emit("update-status", map[string]string{"status": "unpack-started"})

	wineTarget := filepath.Join(gameRoot, "wine")

	slog.Info("Удаление старых директорий proton и prefix")
	os.RemoveAll(wineTarget)

	slog.Info("Распаковка Proton")
	err = proton_install.InstallGEProton(gameRoot, unpackCb)
	if err != nil {
		application.Get().Event.Emit("update-status", map[string]string{"status": "process-error"})
		return fmt.Errorf("ошибка распаковки proton: %w", err)
	}

	slog.Info("Распаковка Prefix")
	if err := core.PrepareEmbeddedLibs(gameRoot, libs, unpackCb); err != nil {
		slog.Warn("Не удалось подготовить встроенные библиотеки при переустановке", "err", err)
	}

	err = prefix_install.UnpackPrefix(gameRoot, unpackCb)
	if err != nil {
		application.Get().Event.Emit("update-status", map[string]string{"status": "process-error"})
		return fmt.Errorf("ошибка распаковки префикса: %w", err)
	}

	application.Get().Event.Emit("update-status", map[string]string{"status": "process-finished"})
	slog.Info("Proton и Prefix успешно переустановлены")

	downloadDir := filepath.Join(gameRoot, "download")
	if err := os.MkdirAll(downloadDir, 0755); err != nil {
		return fmt.Errorf("не удалось создать папку download: %w", err)
	}

	configPath := filepath.Join(downloadDir, "config.json")
	if err := os.WriteFile(configPath, offlineConfig, 0644); err != nil {
		slog.Error("Не удалось записать offlineConfig при миграции", "err", err)
		return fmt.Errorf("ошибка записи config.json: %w", err)
	}

	if err := downloader.RewriteStatus(downloadDir, "config", configPath); err != nil {
		return fmt.Errorf("не удалось обновить статус config в download_status.txt: %w", err)
	}

	_, err = core.ApplyConfigPatches(gameRoot, unpackCb)
	if err != nil {
		return err
	}

	application.Get().Event.Emit("update-status", map[string]string{"status": "process-finished"})
	slog.Info("Миграция #1 успешно завершена")

	return nil
}
