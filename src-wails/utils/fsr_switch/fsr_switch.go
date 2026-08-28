package fsrswitch

import (
	"context"
	"fmt"
	config_patcher "rfad-launcher-linux/src-wails/patches/patch_configs"
	"rfad-launcher-linux/src-wails/utils"
	"strconv"
	"strings"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

func getBaseResolution(ctx context.Context, gameRoot string) (float64, float64) {
	wStr, _ := utils.GetOneSetting(gameRoot, "BaseWidth:")
	hStr, _ := utils.GetOneSetting(gameRoot, "BaseHeight:")

	w, errW := strconv.ParseFloat(strings.TrimSpace(wStr), 64)
	h, errH := strconv.ParseFloat(strings.TrimSpace(hStr), 64)

	if errW == nil && errH == nil && w > 0 && h > 0 {
		return w, h
	}

	width, height := 1920.0, 1080.0
	screens, _ := runtime.ScreenGetAll(ctx)
	if len(screens) > 0 {
		// Берем первый (основной) монитор
		width = float64(screens[0].Size.Width)
		height = float64(screens[0].Size.Height)
	}

	utils.SetOneSetting(gameRoot, "BaseWidth:", width)
	utils.SetOneSetting(gameRoot, "BaseHeight:", height)

	return width, height
}

func ApplyFsrPatches(ctx context.Context, gameRoot string, fsrLvl string) error {
	grafikMod, _ := utils.GetOneSetting(gameRoot, "GrafikMod:")
	grafikMod = strings.TrimSpace(grafikMod)

	useFSRStr, _ := utils.GetOneSetting(gameRoot, "FSR:")
	useFSR := strings.TrimSpace(useFSRStr) == "true"

	var patches []config_patcher.ConfigPatch

	if useFSR && grafikMod == "CommunityShader" {
		qualityMode := 0
		switch fsrLvl {
		case "95":
			qualityMode = 0
		case "75":
			qualityMode = 1
		case "50":
			qualityMode = 2
		case "25":
			qualityMode = 3
		default:
			qualityMode = 0
		}

		// Обрати внимание: сохраняем оригинальные отступы мода (2 пробела)
		fsrLevelStr := fmt.Sprintf(`  "qualityMode": %d,`, qualityMode)
		patches = append(patches, config_patcher.ConfigPatch{
			TargetFile: "MO2/overwrite/SKSE/Plugins/CommunityShaders/SettingsUser.json",
			ReplacePrefix: map[string]string{
				`"qualityMode":`: fsrLevelStr,
			},
		})

	} else if useFSR && grafikMod != "CommunityShader" {
		// Читаем базовое (нативное) разрешение из конфига или получаем через Wails
		baseWidth, baseHeight := getBaseResolution(ctx, gameRoot)

		multiplier := 1.0
		switch fsrLvl {
		case "95":
			multiplier = 0.95
		case "75":
			multiplier = 0.75
		case "50":
			multiplier = 0.50
		case "25":
			multiplier = 0.25
		default:
			multiplier = 0.95
		}

		finalW := fmt.Sprintf("%d", int(baseWidth*multiplier))
		finalH := fmt.Sprintf("%d", int(baseHeight*multiplier))

		// Патчим SkyrimPrefs.ini через префиксы
		patches = append(patches, config_patcher.ConfigPatch{
			TargetFile: "MO2/profiles/RFAD_SE/SkyrimPrefs.ini",
			ReplacePrefix: map[string]string{
				"iSize W=": fmt.Sprintf("iSize W=%s", finalW),
				"iSize H=": fmt.Sprintf("iSize H=%s", finalH),
			},
		})

		// Патчим SSE Display Tweaks (здесь комбинируем точный Replace и ReplacePrefix)
		resString := fmt.Sprintf("Resolution = %sx%s", finalW, finalH)
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
	}

	if len(patches) > 0 {
		_, err := config_patcher.ApplyPatchesFromJSON(gameRoot, patches, nil)
		if err != nil {
			return fmt.Errorf("ошибка при применении патчей FSR: %w", err)
		}
	}

	return nil
}

func SyncFSRSettings(ctx context.Context, gameRoot string, grafikMod string) error {
	grafikMod = strings.TrimSpace(grafikMod)
	useFSRStr, _ := utils.GetOneSetting(gameRoot, "FSR:")
	useFSR := strings.TrimSpace(useFSRStr) == "true"

	fsrLvl, _ := utils.GetOneSetting(gameRoot, "FsrLvl:")
	fsrLvl = strings.TrimSpace(fsrLvl)

	baseWidth, baseHeight := getBaseResolution(ctx, gameRoot)
	var patches []config_patcher.ConfigPatch

	if grafikMod == "CommunityShader" {
		// --- ЛОГИКА COMMUNITY SHADERS ---
		if useFSR {
			qualityMode := 0
			switch fsrLvl {
			case "95":
				qualityMode = 0
			case "75":
				qualityMode = 1
			case "50":
				qualityMode = 2
			case "25":
				qualityMode = 3
			}

			patches = append(patches, config_patcher.ConfigPatch{
				TargetFile: "MO2/overwrite/SKSE/Plugins/CommunityShaders/SettingsUser.json",
				ReplacePrefix: map[string]string{
					`"qualityMode":`: fmt.Sprintf(`  "qualityMode": %d,`, qualityMode),
				},
			})
		}

		// Возвращаем нативное разрешение (100%)
		finalW := fmt.Sprintf("%d", int(baseWidth))
		finalH := fmt.Sprintf("%d", int(baseHeight))

		patches = append(patches, config_patcher.ConfigPatch{
			TargetFile: "MO2/profiles/RFAD_SE/SkyrimPrefs.ini",
			ReplacePrefix: map[string]string{
				"iSize W=": fmt.Sprintf("iSize W=%s", finalW),
				"iSize H=": fmt.Sprintf("iSize H=%s", finalH),
			},
		})
		resString := fmt.Sprintf("Resolution = %sx%s", finalW, finalH)
		patches = append(patches, config_patcher.ConfigPatch{
			TargetFile: "MO2/mods/SSE Display Tweaks/SKSE/Plugins/SSEDisplayTweaks.ini",
			ReplacePrefix: map[string]string{
				"Resolution =": resString,
			},
		})

	} else {
		multiplier := 1.0
		if useFSR {
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

		finalW := fmt.Sprintf("%d", int(baseWidth*multiplier))
		finalH := fmt.Sprintf("%d", int(baseHeight*multiplier))

		patches = append(patches, config_patcher.ConfigPatch{
			TargetFile: "MO2/profiles/RFAD_SE/SkyrimPrefs.ini",
			ReplacePrefix: map[string]string{
				"iSize W=": fmt.Sprintf("iSize W=%s", finalW),
				"iSize H=": fmt.Sprintf("iSize H=%s", finalH),
			},
		})

		resString := fmt.Sprintf("Resolution = %sx%s", finalW, finalH)
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
	}

	if len(patches) > 0 {
		_, err := config_patcher.ApplyPatchesFromJSON(gameRoot, patches, nil)
		if err != nil {
			return fmt.Errorf("ошибка синхронизации FSR: %w", err)
		}
	}

	return nil
}

func ResetToNativeBorderless(ctx context.Context, gameRoot string) error {
	// Берем базовое (нативное) разрешение экрана
	baseWidth, baseHeight := getBaseResolution(ctx, gameRoot)

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

func ApplyFSR(ctx context.Context, gameRoot string, fsrEnabled bool) error {
	if !fsrEnabled {
		return ResetToNativeBorderless(ctx, gameRoot)
	}

	grafikMod, _ := utils.GetOneSetting(gameRoot, "GrafikMod:")
	grafikMod = strings.TrimSpace(grafikMod)

	fsrLvl, _ := utils.GetOneSetting(gameRoot, "FsrLvl:")
	fsrLvl = strings.TrimSpace(fsrLvl)
	if fsrLvl == "" {
		fsrLvl = "95"
	}

	baseWidth, baseHeight := getBaseResolution(ctx, gameRoot)
	var patches []config_patcher.ConfigPatch

	if grafikMod == "CommunityShader" {
		qualityMode := 0
		switch fsrLvl {
		case "95":
			qualityMode = 0
		case "75":
			qualityMode = 1
		case "50":
			qualityMode = 2
		case "25":
			qualityMode = 3
		}

		patches = append(patches, config_patcher.ConfigPatch{
			TargetFile: "MO2/overwrite/SKSE/Plugins/CommunityShaders/SettingsUser.json",
			ReplacePrefix: map[string]string{
				`"qualityMode":`: fmt.Sprintf(`  "qualityMode": %d,`, qualityMode),
			},
		})

		finalW := fmt.Sprintf("%d", int(baseWidth))
		finalH := fmt.Sprintf("%d", int(baseHeight))

		patches = append(patches, config_patcher.ConfigPatch{
			TargetFile: "MO2/profiles/RFAD_SE/SkyrimPrefs.ini",
			ReplacePrefix: map[string]string{
				"iSize W=": fmt.Sprintf("iSize W=%s", finalW),
				"iSize H=": fmt.Sprintf("iSize H=%s", finalH),
			},
		})
		resString := fmt.Sprintf("Resolution = %sx%s", finalW, finalH)
		patches = append(patches, config_patcher.ConfigPatch{
			TargetFile: "MO2/mods/SSE Display Tweaks/SKSE/Plugins/SSEDisplayTweaks.ini",
			ReplacePrefix: map[string]string{
				"Resolution =": resString,
			},
		})

	} else {
		multiplier := 0.95
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

		finalW := fmt.Sprintf("%d", int(baseWidth*multiplier))
		finalH := fmt.Sprintf("%d", int(baseHeight*multiplier))

		patches = append(patches, config_patcher.ConfigPatch{
			TargetFile: "MO2/profiles/RFAD_SE/SkyrimPrefs.ini",
			ReplacePrefix: map[string]string{
				"iSize W=": fmt.Sprintf("iSize W=%s", finalW),
				"iSize H=": fmt.Sprintf("iSize H=%s", finalH),
			},
		})

		resString := fmt.Sprintf("Resolution = %sx%s", finalW, finalH)
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
	}

	if len(patches) > 0 {
		_, err := config_patcher.ApplyPatchesFromJSON(gameRoot, patches, nil)
		if err != nil {
			return fmt.Errorf("ошибка применения FSR: %w", err)
		}
	}

	return nil
}
