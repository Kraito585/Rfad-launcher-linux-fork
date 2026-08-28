package config_patcher

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type ConfigPatch struct {
	TargetFile    string            `json:"TargetFile"`
	Replace       map[string]string `json:"Replace"`
	InsertAfter   map[string]string `json:"InsertAfter"`
	ReplacePrefix map[string]string `json:"ReplacePrefix"`
}

// ApplyPatchesFromJSON применяет патчи из списка к файлам в корневой папке gameRoot.
// Возвращает количество успешно применённых патчей и ошибку, если что-то критическое.
func ApplyPatchesFromJSON(gameRoot string, patches []ConfigPatch, progressCb func(percent float64, msg string)) (int, error) {
	total := len(patches)
	applied := 0

	for i, patch := range patches {
		fullPath := filepath.Join(gameRoot, patch.TargetFile)
		if progressCb != nil {
			progressCb(float64(i+1)/float64(total), fmt.Sprintf("Патчинг: %s", patch.TargetFile))
		}

		// Проверяем существование файла
		if _, err := os.Stat(fullPath); os.IsNotExist(err) {
			// Логируем предупреждение, но не останавливаем
			fmt.Printf("WARN: файл не найден: %s\n", fullPath)
			continue
		}

		// Применяем патч
		modified, err := applySinglePatchSafe(fullPath, patch)
		if err != nil {
			fmt.Printf("ERROR: ошибка патчинга %s: %v\n", patch.TargetFile, err)
			continue
		}
		if modified {
			applied++
		}
	}

	return applied, nil
}

// applySinglePatchSafe применяет один патч, проверяя, не применён ли уже.
// Возвращает true, если файл был изменён.
func applySinglePatchSafe(filePath string, patch ConfigPatch) (bool, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return false, err
	}
	defer file.Close()

	var lines []string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}
	if err := scanner.Err(); err != nil {
		return false, err
	}

	originalLines := make([]string, len(lines))
	copy(originalLines, lines)

	for search, replace := range patch.Replace {
		alreadyReplaced := false
		for _, line := range lines {
			if strings.TrimSpace(line) == replace {
				alreadyReplaced = true
				break
			}
		}
		if alreadyReplaced {
			continue
		}

		for i, line := range lines {
			trimmed := strings.TrimSpace(line)
			if trimmed == search {
				lines[i] = replace
				break
			}
		}
	}

	for prefix, replace := range patch.ReplacePrefix {
		for i, line := range lines {
			if strings.HasPrefix(strings.TrimSpace(line), prefix) {
				if strings.TrimSpace(line) != replace {
					lines[i] = replace
				}
				break
			}
		}
	}

	for search, insertBlock := range patch.InsertAfter {
		insertLines := strings.Split(insertBlock, "\n")
		alreadyInserted := false
		for _, line := range lines {
			if strings.TrimSpace(line) == strings.TrimSpace(insertLines[0]) {
				alreadyInserted = true
				break
			}
		}
		if alreadyInserted {
			continue
		}

		insertIdx := -1
		for i, line := range lines {
			if strings.TrimSpace(line) == search {
				insertIdx = i
				break
			}
		}
		if insertIdx == -1 {
			continue
		}

		var newLines []string
		for i, line := range lines {
			newLines = append(newLines, line)
			if i == insertIdx {
				for _, ins := range insertLines {
					if ins != "" {
						newLines = append(newLines, ins)
					}
				}
			}
		}
		lines = newLines
	}

	changed := !equalSlices(originalLines, lines)
	if !changed {
		return false, nil
	}
	return true, os.WriteFile(filePath, []byte(strings.Join(lines, "\n")), 0644)
}

func equalSlices(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
