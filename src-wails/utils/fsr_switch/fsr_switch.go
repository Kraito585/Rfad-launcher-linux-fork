package fsrswitch

import (
	"fmt"
	config_patcher "rfad-launcher-linux/src-wails/patches/patch_configs"
	"rfad-launcher-linux/src-wails/utils"
	"strings"

	"github.com/wailsapp/wails/v3/pkg/application"
)

func getBaseResolution() (int, int) {
	primaryScreen := application.Get().Screen.GetPrimary()
	if primaryScreen == nil {
		// Фоллбэк на случай, если монитор не удалось определить
		return 1920, 1080
	}

	// Можно использовать PhysicalBounds.Width / Height или поле Size
	return primaryScreen.PhysicalBounds.Width, primaryScreen.PhysicalBounds.Height
}

func SyncFSRSettings(gameRoot string, grafikMod string) error {
	grafikMod = strings.TrimSpace(grafikMod)
	useFSRStr, _ := utils.GetOneSetting("FSR")
	useFSR := strings.TrimSpace(useFSRStr) == "true"

	fsrLvl, _ := utils.GetOneSetting("FsrLvl")
	fsrLvl = strings.TrimSpace(fsrLvl)

	baseWidth, baseHeight := getBaseResolution()
	var patches []config_patcher.ConfigPatch

	if grafikMod == "CommunityShader" {
		// ==========================================
		// --- ЛОГИКА COMMUNITY SHADERS ---
		// ==========================================

		csSettingsPatch := config_patcher.ConfigPatch{
			TargetFile:    "MO2/overwrite/SKSE/Plugins/CommunityShaders/SettingsUser.json",
			ReplacePrefix: make(map[string]string),
		}

		if useFSR {
			qualityMode := 0
			switch fsrLvl {
			case "95":
				qualityMode = 1
			case "75":
				qualityMode = 2
			case "50":
				qualityMode = 3
			case "25":
				qualityMode = 4
			}
			csSettingsPatch.ReplacePrefix[`"qualityMode":`] = fmt.Sprintf(`  "qualityMode": %d,`, qualityMode)
			csSettingsPatch.ReplacePrefix[`"frameGenerationMode":`] = `  "frameGenerationMode": 1,`
		} else {
			csSettingsPatch.ReplacePrefix[`"frameGenerationMode":`] = `  "frameGenerationMode": 0,`
		}
		patches = append(patches, csSettingsPatch)

		finalW := fmt.Sprintf("%d", int(baseWidth))
		finalH := fmt.Sprintf("%d", int(baseHeight))
		resString := fmt.Sprintf("Resolution = %sx%s", finalW, finalH)

		// Для SkyrimPrefs ВСЕГДА оставляем bFull Screen=0 (требование Display Tweaks)
		patches = append(patches, config_patcher.ConfigPatch{
			TargetFile: "MO2/profiles/RFAD_SE/SkyrimPrefs.ini",
			ReplacePrefix: map[string]string{
				"iSize W=":      fmt.Sprintf("iSize W=%s", finalW),
				"iSize H=":      fmt.Sprintf("iSize H=%s", finalH),
				"bFull Screen=": "bFull Screen=0",
				"bBorderless=":  "bBorderless=1",
			},
		})

		patches = append(patches, config_patcher.ConfigPatch{
			TargetFile: "MO2/mods/SSE Display Tweaks/SKSE/Plugins/SSEDisplayTweaks.ini",
			Replace: map[string]string{
				"Fullscreen = true":  "Fullscreen = false",
				"Borderless = false": "Borderless = true",
			},
			ReplacePrefix: map[string]string{
				"Resolution =": resString,
			},
		})

		patches = append(patches, config_patcher.ConfigPatch{
			TargetFile: "MO2/profiles/RFAD_SE/modlist.txt",
			Replace: map[string]string{
				"+Enhanced Volumetric Lighting and Shadows (EVLaS)": "-Enhanced Volumetric Lighting and Shadows (EVLaS)",
			},
		})

	} else {
		// ==========================================
		// --- ЛОГИКА ДЛЯ ENB ИЛИ ВАНИЛЛЫ ---
		// ==========================================
		multiplier := 1.0
		isWineFullscreen := false

		if useFSR {
			isWineFullscreen = true
			switch fsrLvl {
			case "95":
				multiplier = 0.95
			case "75":
				multiplier = 0.75
			case "50":
				multiplier = 0.50
			case "25":
				multiplier = 0.25
			}
		}

		finalW := fmt.Sprintf("%d", int(float64(baseWidth)*multiplier))
		finalH := fmt.Sprintf("%d", int(float64(baseHeight)*multiplier))
		resString := fmt.Sprintf("Resolution = %sx%s", finalW, finalH)

		patches = append(patches, config_patcher.ConfigPatch{
			TargetFile: "MO2/profiles/RFAD_SE/SkyrimPrefs.ini",
			ReplacePrefix: map[string]string{
				"iSize W=":      fmt.Sprintf("iSize W=%s", finalW),
				"iSize H=":      fmt.Sprintf("iSize H=%s", finalH),
				"bFull Screen=": "bFull Screen=0",
				"bBorderless=":  "bBorderless=1",
			},
		})

		if isWineFullscreen {
			patches = append(patches, config_patcher.ConfigPatch{
				TargetFile: "MO2/mods/SSE Display Tweaks/SKSE/Plugins/SSEDisplayTweaks.ini",
				Replace: map[string]string{
					"Fullscreen = false": "Fullscreen = true",
					"Borderless = true":  "Borderless = false",
				},
				ReplacePrefix: map[string]string{
					"Resolution =": resString,
				},
			})
		} else {
			// Дефолтный оконный безрамочный режим
			patches = append(patches, config_patcher.ConfigPatch{
				TargetFile: "MO2/mods/SSE Display Tweaks/SKSE/Plugins/SSEDisplayTweaks.ini",
				Replace: map[string]string{
					"Fullscreen = true":  "Fullscreen = false",
					"Borderless = false": "Borderless = true",
				},
				ReplacePrefix: map[string]string{
					"Resolution =": resString,
				},
			})
		}

		patches = append(patches, config_patcher.ConfigPatch{
			TargetFile: "MO2/profiles/RFAD_SE/modlist.txt",
			Replace: map[string]string{
				"-Enhanced Volumetric Lighting and Shadows (EVLaS)": "+Enhanced Volumetric Lighting and Shadows (EVLaS)",
			},
		})
	}

	if len(patches) > 0 {
		_, err := config_patcher.ApplyPatchesFromJSON(gameRoot, patches, nil)
		if err != nil {
			return fmt.Errorf("ошибка синхронизации FSR и профилей модов: %w", err)
		}
	}

	return nil
}

func ResetToNativeBorderless(gameRoot string) error {
	baseWidth, baseHeight := getBaseResolution()

	finalW := fmt.Sprintf("%d", int(baseWidth))
	finalH := fmt.Sprintf("%d", int(baseHeight))
	resString := fmt.Sprintf("Resolution = %sx%s", finalW, finalH)

	// Формируем патчи для возврата в оконный безрамочный режим
	patches := []config_patcher.ConfigPatch{
		{
			TargetFile: "MO2/profiles/RFAD_SE/SkyrimPrefs.ini",
			ReplacePrefix: map[string]string{
				"iSize W=": fmt.Sprintf("iSize W=%s", finalW),
				"iSize H=": fmt.Sprintf("iSize H=%s", finalH),
			},
		},
		{
			TargetFile: "MO2/mods/SSE Display Tweaks/SKSE/Plugins/SSEDisplayTweaks.ini",
			Replace: map[string]string{
				// Меняем обратно на оконный безрамочный
				"Fullscreen = true":  "Fullscreen = false",
				"Borderless = false": "Borderless = true",
			},
			ReplacePrefix: map[string]string{
				"Resolution =": resString,
			},
		},
	}

	// Применяем патчи
	_, err := config_patcher.ApplyPatchesFromJSON(gameRoot, patches, nil)
	if err != nil {
		return fmt.Errorf("ошибка сброса в оконный режим: %w", err)
	}

	return nil
}
