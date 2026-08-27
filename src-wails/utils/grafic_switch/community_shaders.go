package grafic_switch

// package downloader

// import (
// 	"archive/zip"
// 	"encoding/json"
// 	"fmt"
// 	"io"
// 	"net/http"
// 	"os"
// 	"path/filepath"
// 	"rfad-installer/tui"
// 	"strconv"
// 	"strings"
// 	"time"

// 	"github.com/bodgit/sevenzip"
// )

// // Структуры под твой API
// type PresetsResponse struct {
// 	Success bool     `json:"success"`
// 	Data    []Preset `json:"data"`
// }

// type Preset struct {
// 	ID                string         `json:"id"`
// 	URL               string         `json:"url"`
// 	Images            []string       `json:"images"`
// 	PerformanceImpact int            `json:"performance_impact"`
// 	Metadata          PresetMetadata `json:"metadata"`
// 	CreatedAt         time.Time      `json:"created_at"`
// }

// type PresetMetadata struct {
// 	OriginURL      string        `json:"originUrl"`
// 	AuthorNickname string        `json:"authorNickname"`
// 	Description    string        `json:"description"`
// 	OptionalMods   []OptionalMod `json:"optional_mods"`
// }

// type OptionalMod struct {
// 	ID         string   `json:"id"`
// 	Name       string   `json:"name"`
// 	URL        string   `json:"url"`
// 	IsRequired bool     `json:"is_required"`
// 	DependsOn  []string `json:"depends_on"`
// }

// var GlobalPresets []Preset

// // ВАЖНО: Ссылки возвращены на оригинальные .7z архивы
// const (
// 	CSBaseURL     = "https://mirror.kraito.ru/rfad/shaders/Community%20Shaders%2086492%201.7.3%202026-06-27T10-38Z%206Xybdafll.7z"
// 	CSBaseModName = "Community Shaders 86492 1.7.3 2026-06-27T10-38Z 6Xybdafll"

// 	CSUpscaleURL     = "https://mirror.kraito.ru/rfad/shaders/Upscaling%20156952%201.4.0%202026-05-31T10-27Z%20L5WQbqiov.7z"
// 	CSUpscaleModName = "Upscaling 156952 1.4.0 2026-05-31T10-27Z L5WQbqiov"
// )

// // GetQualityModeString возвращает нужный ID пресета в виде строки для патчера
// func GetQualityModeString(cfg *tui.InstallConfig) string {
// 	if cfg.FSRLevel == 4 {
// 		scale, err := strconv.ParseFloat(cfg.CustomFSRScale, 64)
// 		if err != nil || scale <= 0 {
// 			return "1" // По умолчанию Quality
// 		}
// 		if scale >= 95.0 {
// 			return "0" // Native
// 		}
// 		if scale >= 63.0 {
// 			return "1" // Quality
// 		}
// 		if scale >= 55.0 {
// 			return "2" // Balanced
// 		}
// 		if scale >= 45.0 {
// 			return "3" // Performance
// 		}
// 		return "4" // Ultra Performance
// 	}
// 	return fmt.Sprintf("%d", cfg.FSRLevel)
// }

// // FetchAndCachePresets вызываем в самом начале функции main() в main.go
// func FetchAndCachePresets(cacheDir string) {
// 	apiURL := "https://api.kraito.ru/api/v1/community/shaders"

// 	resp, err := http.Get(apiURL)
// 	if err != nil {
// 		LogError("Не удалось получить пресеты: %v", err)
// 		return
// 	}
// 	defer resp.Body.Close()

// 	var result PresetsResponse
// 	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
// 		LogError("Ошибка парсинга JSON пресетов: %v", err)
// 		return
// 	}

// 	GlobalPresets = result.Data

// 	// Запускаем фоновую загрузку картинок
// 	go func() {
// 		imagesDir := filepath.Join(cacheDir, "images")
// 		os.MkdirAll(imagesDir, 0755)

// 		for _, preset := range GlobalPresets {
// 			for i, imgPath := range preset.Images {
// 				fullURL := "https://mirror.kraito.ru/" + imgPath
// 				localPath := filepath.Join(imagesDir, fmt.Sprintf("%s_%d.png", preset.ID, i))

// 				if _, err := os.Stat(localPath); os.IsNotExist(err) {
// 					downloadFile(fullURL, localPath)
// 				}
// 			}
// 		}
// 	}()
// }

// func downloadFile(url, dest string) error {
// 	out, err := os.Create(dest)
// 	if err != nil {
// 		return err
// 	}
// 	defer out.Close()
// 	resp, err := http.Get(url)
// 	if err != nil {
// 		return err
// 	}
// 	defer resp.Body.Close()
// 	_, err = io.Copy(out, resp.Body)
// 	return err
// }

// // Вспомогательная функция для поиска всех плагинов (.esp, .esm, .esl) в директории мода
// func findAndEnablePlugins(installPath, modDir string) {
// 	err := filepath.Walk(modDir, func(path string, info os.FileInfo, err error) error {
// 		if err != nil {
// 			return nil // Игнорируем недоступные файлы
// 		}
// 		if !info.IsDir() {
// 			ext := strings.ToLower(filepath.Ext(info.Name()))
// 			if ext == ".esp" || ext == ".esm" || ext == ".esl" {
// 				LogInfo("Найден плагин: %s. Добавляем в plugins.txt", info.Name())
// 				// Вызываем твою функцию из core
// 				if enableErr := EnablePlugin(installPath, info.Name()); enableErr != nil {
// 					LogWarn("Ошибка активации плагина %s: %v", info.Name(), enableErr)
// 				}
// 			}
// 		}
// 		return nil
// 	})
// 	if err != nil {
// 		LogWarn("Ошибка при поиске плагинов в %s: %v", modDir, err)
// 	}
// }

// // InstallCSBaseMods скачивает и нативно распаковывает обязательную базу CS
// func InstallCSBaseMods(installPath, cacheDir string, progressCb func(float64, string)) error {
// 	modsDir := filepath.Join(installPath, "MO2", "mods")

// 	// ==========================================
// 	// 1. Установка базового мода Community Shaders
// 	// ==========================================
// 	baseArchive := filepath.Join(cacheDir, "CS_Base.7z")
// 	baseDest := filepath.Join(modsDir, CSBaseModName)

// 	if progressCb != nil {
// 		progressCb(-1, "Скачивание базового мода Community Shaders...")
// 	}

// 	if _, err := os.Stat(baseArchive); os.IsNotExist(err) {
// 		if err := downloadFile(CSBaseURL, baseArchive); err != nil {
// 			return fmt.Errorf("ошибка скачивания CS Base: %v", err)
// 		}
// 	}

// 	if err := os.MkdirAll(baseDest, 0755); err != nil {
// 		return err
// 	}

// 	if err := extract7z(baseArchive, baseDest, progressCb); err != nil {
// 		return fmt.Errorf("ошибка распаковки CS Base: %v", err)
// 	}
// 	findAndEnablePlugins(installPath, baseDest)

// 	// ==========================================
// 	// 2. Установка мода Upscaling
// 	// ==========================================
// 	upscaleArchive := filepath.Join(cacheDir, "CS_Upscaling.7z")
// 	upscaleDest := filepath.Join(modsDir, CSUpscaleModName)

// 	if progressCb != nil {
// 		progressCb(-1, "Скачивание мода Upscaling...")
// 	}

// 	if _, err := os.Stat(upscaleArchive); os.IsNotExist(err) {
// 		if err := downloadFile(CSUpscaleURL, upscaleArchive); err != nil {
// 			return fmt.Errorf("ошибка скачивания CS Upscaling: %v", err)
// 		}
// 	}

// 	if err := os.MkdirAll(upscaleDest, 0755); err != nil {
// 		return err
// 	}

// 	if err := extract7z(upscaleArchive, upscaleDest, progressCb); err != nil {
// 		return fmt.Errorf("ошибка распаковки CS Upscaling: %v", err)
// 	}
// 	findAndEnablePlugins(installPath, upscaleDest)

// 	return nil
// }

// // InstallCSPresetMods скачивает и устанавливает выбранный пресет и его опциональные моды
// func InstallCSPresetMods(installPath string, cfg *tui.InstallConfig, cacheDir string, progressCb func(float64, string)) error {
// 	if cfg.ShaderPresetID == "" {
// 		return nil
// 	}

// 	var activePreset *Preset
// 	for _, p := range GlobalPresets {
// 		if p.ID == cfg.ShaderPresetID {
// 			activePreset = &p
// 			break
// 		}
// 	}
// 	if activePreset == nil {
// 		return fmt.Errorf("пресет с ID %s не найден в кэше", cfg.ShaderPresetID)
// 	}

// 	modsDir := filepath.Join(installPath, "MO2", "mods")

// 	// Вспомогательная функция загрузки и распаковки
// 	processArchive := func(relativeURL, folderName string) error {
// 		fullURL := "https://mirror.kraito.ru/" + relativeURL
// 		fileName := filepath.Base(relativeURL)

// 		archivePath := filepath.Join(cacheDir, fileName)
// 		targetDir := filepath.Join(modsDir, folderName)

// 		if progressCb != nil {
// 			progressCb(-1, fmt.Sprintf("Скачивание: %s...", folderName))
// 		}

// 		if _, err := os.Stat(archivePath); os.IsNotExist(err) {
// 			if err := downloadFile(fullURL, archivePath); err != nil {
// 				return fmt.Errorf("ошибка скачивания %s: %v", folderName, err)
// 			}
// 		}

// 		if err := os.MkdirAll(targetDir, 0755); err != nil {
// 			return err
// 		}

// 		if progressCb != nil {
// 			progressCb(-1, fmt.Sprintf("Распаковка: %s...", folderName))
// 		}

// 		// Универсальная маршрутизация распаковки (7z и zip)
// 		if strings.HasSuffix(fileName, ".7z") {
// 			if err := extract7z(archivePath, targetDir, progressCb); err != nil {
// 				return err
// 			}
// 		} else if strings.HasSuffix(fileName, ".zip") {
// 			if err := extractZip(archivePath, targetDir, progressCb); err != nil {
// 				return err
// 			}
// 		} else {
// 			return fmt.Errorf("неподдерживаемый формат архива: %s", fileName)
// 		}

// 		// Сканируем только что распакованную папку на наличие плагинов и активируем их
// 		findAndEnablePlugins(installPath, targetDir)
// 		return nil
// 	}

// 	// 2. Установка основного пресета
// 	mainPresetName := fmt.Sprintf("CS Preset - %s", activePreset.Metadata.AuthorNickname)
// 	if err := processArchive(activePreset.URL, mainPresetName); err != nil {
// 		return err
// 	}

// 	// 3. Установка опциональных модов
// 	for _, mod := range activePreset.Metadata.OptionalMods {
// 		if cfg.ShaderMods[mod.ID] {
// 			modFolderName := fmt.Sprintf("CS Addon - %s", sanitize(mod.Name))
// 			if err := processArchive(mod.URL, modFolderName); err != nil {
// 				return err
// 			}
// 		}
// 	}

// 	return nil
// }

// 	patches := []ConfigPatch{
// 		{
// 			TargetFile: "MO2/mods/SSE Display Tweaks/SKSE/Plugins/SSEDisplayTweaks.ini",
// 			Replace:    map[string]string{"FramerateLimit = 60": "FramerateLimit = 0"},
// 		},
// 	}

// 	if cfg.UseFSR && cfg.GraphicsMod != "Community Shaders" {
// 		var finalW, finalH string

// 		switch cfg.FSRLevel {
// 		case 1:
// 			finalW = fmt.Sprintf("%d", int(float64(cfg.BaseWidth)*0.75))
// 			finalH = fmt.Sprintf("%d", int(float64(cfg.BaseHeight)*0.75))
// 		case 2:
// 			finalW = fmt.Sprintf("%d", int(float64(cfg.BaseWidth)*0.50))
// 			finalH = fmt.Sprintf("%d", int(float64(cfg.BaseHeight)*0.50))
// 		case 3:
// 			finalW = fmt.Sprintf("%d", int(float64(cfg.BaseWidth)*0.25))
// 			finalH = fmt.Sprintf("%d", int(float64(cfg.BaseHeight)*0.25))
// 		case 4:
// 			scale, err := strconv.ParseFloat(cfg.CustomFSRScale, 64)
// 			if err != nil || scale <= 0 {
// 				scale = 67.0
// 			}
// 			multiplier := scale / 100.0
// 			finalW = fmt.Sprintf("%d", int(float64(cfg.BaseWidth)*multiplier))
// 			finalH = fmt.Sprintf("%d", int(float64(cfg.BaseHeight)*multiplier))
// 		default:
// 			finalW = fmt.Sprintf("%d", int(float64(cfg.BaseWidth)*0.75))
// 			finalH = fmt.Sprintf("%d", int(float64(cfg.BaseHeight)*0.75))
// 		}

// 		// Патчим SkyrimPrefs.ini
// 		patches = append(patches, ConfigPatch{
// 			TargetFile: "MO2/profiles/RFAD_SE/SkyrimPrefs.ini",
// 			ReplacePrefix: map[string]string{
// 				"iSize W=": fmt.Sprintf("iSize W=%s", finalW),
// 				"iSize H=": fmt.Sprintf("iSize H=%s", finalH),
// 			},
// 		})

// 		// Патчим SSE Display Tweaks
// 		resString := fmt.Sprintf("Resolution = %sx%s", finalW, finalH)
// 		patches = append(patches, ConfigPatch{
// 			TargetFile: "MO2/mods/SSE Display Tweaks/SKSE/Plugins/SSEDisplayTweaks.ini",
// 			Replace: map[string]string{
// 				"Fullscreen = false": "Fullscreen = true",
// 				"Borderless = true":  "Borderless = false",
// 			},
// 			ReplacePrefix: map[string]string{
// 				"Resolution =": resString,
// 			},
// 		})
// 	}

// 	// Влкючение Community Shaders и лечение его зависимостей
// 	if cfg.GraphicsMod == "Community Shaders" {
// 		insertMods := "\n+Community Shaders 86492 1.7.3 2026-06-27T10-38Z 6Xybdafll\n+Upscaling 156952 1.4.0 2026-05-31T10-27Z L5WQbqiov"

// 		// Находим активный пресет для добавления его в список
// 		for _, p := range GlobalPresets {
// 			if p.ID == cfg.ShaderPresetID {
// 				insertMods += fmt.Sprintf("\n+CS Preset - %s", p.Metadata.AuthorNickname)

// 				// Добавляем включенные дополнения
// 				for _, mod := range p.Metadata.OptionalMods {
// 					if cfg.ShaderMods[mod.ID] {
// 						insertMods += fmt.Sprintf("\n+CS Addon - %s", sanitize(mod.Name))
// 					}
// 				}
// 				break
// 			}
// 		}
// 		insertMods += "\n-Community Shaders_separator"

// 		// Патчим modlist.txt
// 		patches = append(patches, ConfigPatch{
// 			TargetFile: "MO2/profiles/RFAD_SE/modlist.txt",
// 			Replace: map[string]string{
// 				"+Enhanced Volumetric Lighting and Shadows (EVLaS)": "-Enhanced Volumetric Lighting and Shadows (EVLaS)",
// 			},
// 			InsertAfter: map[string]string{
// 				"-MY MODS_separator": insertMods,
// 			},
// 		})

// 		patches = append(patches, ConfigPatch{
// 			TargetFile: "MO2/overwrite/SKSE/Plugins/CommunityShaders/SettingsUser.json",
// 			ReplacePrefix: map[string]string{
// 				// Ищем без пробелов в начале (так как строка обрезается через TrimSpace)
// 				`"frameGenerationMode":`: `  "frameGenerationMode": 1,`,
// 			},
// 		})

// 	}

// 	if cfg.UseFSR && cfg.GraphicsMod == "Community Shaders" {
// 		fsrLevelStr := fmt.Sprintf(`  "qualityMode": %v,`, getQualityMode(cfg))

// 		patches = append(patches, ConfigPatch{
// 			TargetFile: "MO2/overwrite/SKSE/Plugins/CommunityShaders/SettingsUser.json",
// 			ReplacePrefix: map[string]string{
// 				// Ищем без пробелов в начале
// 				`"qualityMode":`: fsrLevelStr,
// 			},
// 		})
// 	}

// 	if !cfg.UseFSR && cfg.GraphicsMod == "Community Shaders" {
// 		patches = append(patches, ConfigPatch{
// 			TargetFile: "MO2/overwrite/SKSE/Plugins/CommunityShaders/SettingsUser.json",
// 			ReplacePrefix: map[string]string{
// 				// Ищем без пробелов в начале
// 				`"upscaleMethodNoDLSS":`: `  "upscaleMethodNoDLSS": 0,`,
// 				`"qualityMode":`:         `  "qualityMode": 0,`,
// 			},
// 		})
// 	}
