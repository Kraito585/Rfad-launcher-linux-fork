package steam_shortcut

import (
	"archive/tar"
	"bufio"
	"bytes"
	"compress/gzip"
	"encoding/binary"
	"errors"
	"fmt"
	"hash/crc32"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

const (
	vdfTypeMap     byte = 0x00
	vdfTypeString  byte = 0x01
	vdfTypeInt32   byte = 0x02
	vdfTypeFloat32 byte = 0x03
	vdfTypePointer byte = 0x04
	vdfTypeColor   byte = 0x06
	vdfTypeUint64  byte = 0x07
	vdfTypeEnd     byte = 0x08
)

type Options struct {
	AppName       string
	LaunchOptions string // пусто сейчас; позже можно передать "--steam" или "--run-game"
	SteamAssets   []byte // embed/steam-asets.tar.gz
	IconBytes     []byte // build/appicon.png
}

type vdfNode struct {
	Fields []*vdfField
}

type vdfField struct {
	Type   byte
	Key    string
	StrVal string
	IntVal uint32
	MapVal *vdfNode
	RawVal []byte
}

type steamAccount struct {
	steamRoot string
	configDir string
	isFlatpak bool
}

// AddLauncherToSteam добавляет или обновляет ярлык лаунчера и его обложки во всех профилях Steam.
func AddLauncherToSteam(opts Options) (int, error) {
	if opts.AppName == "" {
		opts.AppName = "RFAD SE"
	}

	homeDir, err := os.UserHomeDir()
	if err != nil {
		return 0, fmt.Errorf("не удалось определить домашнюю директорию: %w", err)
	}

	accounts, err := findSteamAccounts(homeDir)
	if err != nil {
		return 0, err
	}

	exe, startDir, launchOpts, flatpakID, err := resolveLauncherTarget(opts.LaunchOptions)
	if err != nil {
		return 0, err
	}

	appID := calculateAppID(exe, opts.AppName)
	slog.Info("Добавление ярлыка в Steam",
		"appName", opts.AppName,
		"appID", appID,
		"exe", exe,
		"launchOptions", launchOpts,
		"accounts", len(accounts),
	)

	updatedCount := 0
	for _, acc := range accounts {
		gridDir := filepath.Join(acc.configDir, "grid")
		iconRealPath, err := extractGridAssets(gridDir, appID, opts.SteamAssets, opts.IconBytes)
		if err != nil {
			return updatedCount, fmt.Errorf("ошибка записи обложек в %s: %w", gridDir, err)
		}

		iconVDFPath := iconPathForSteam(acc, iconRealPath, homeDir)
		vdfPath := filepath.Join(acc.configDir, "shortcuts.vdf")

		if err := upsertShortcut(vdfPath, shortcutData{
			AppID:         appID,
			AppName:       opts.AppName,
			Exe:           exe,
			StartDir:      startDir,
			Icon:          iconVDFPath,
			LaunchOptions: launchOpts,
			FlatpakAppID:  flatpakID,
		}); err != nil {
			return updatedCount, fmt.Errorf("ошибка обновления %s: %w", vdfPath, err)
		}

		updatedCount++
	}

	return updatedCount, nil
}

type shortcutData struct {
	AppID         uint32
	AppName       string
	Exe           string
	StartDir      string
	Icon          string
	LaunchOptions string
	FlatpakAppID  string
}

func calculateAppID(exe, appName string) uint32 {
	return crc32.ChecksumIEEE([]byte(exe+appName)) | 0x80000000
}

func resolveLauncherTarget(extraLaunchOpts string) (exe, startDir, launchOpts, flatpakAppID string, err error) {
	// Если сам лаунчер запущен внутри Flatpak — прописываем flatpak run <id>
	if id := detectSelfFlatpakID(); id != "" {
		exe = `"/usr/bin/flatpak"`
		startDir = `"/usr/bin/"`
		flatpakAppID = id
		launchOpts = "run " + id
		if trimmed := strings.TrimSpace(extraLaunchOpts); trimmed != "" {
			launchOpts += " " + trimmed
		}
		return exe, startDir, launchOpts, flatpakAppID, nil
	}

	exePath, err := os.Executable()
	if err != nil {
		return "", "", "", "", fmt.Errorf("не удалось определить путь к бинарнику лаунчера: %w", err)
	}
	if resolved, evalErr := filepath.EvalSymlinks(exePath); evalErr == nil {
		exePath = resolved
	}

	exe = `"` + exePath + `"`
	startDir = `"` + filepath.Dir(exePath) + `/"`
	launchOpts = strings.TrimSpace(extraLaunchOpts)
	return exe, startDir, launchOpts, "", nil
}

func detectSelfFlatpakID() string {
	f, err := os.Open("/.flatpak-info")
	if err != nil {
		return ""
	}
	defer f.Close()

	inAppSection := false
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if strings.HasPrefix(line, "[") {
			inAppSection = (line == "[Application]")
			continue
		}
		if inAppSection && strings.HasPrefix(line, "name=") {
			return strings.TrimSpace(strings.TrimPrefix(line, "name="))
		}
	}
	return ""
}

func findSteamAccounts(homeDir string) ([]steamAccount, error) {
	candidates := []struct {
		root      string
		isFlatpak bool
	}{
		{filepath.Join(homeDir, ".local", "share", "Steam"), false},
		{filepath.Join(homeDir, ".steam", "steam"), false},
		{filepath.Join(homeDir, ".steam", "root"), false},
		{filepath.Join(homeDir, ".var", "app", "com.valvesoftware.Steam", ".local", "share", "Steam"), true},
	}

	seenUserdata := make(map[string]struct{})
	var accounts []steamAccount

	for _, c := range candidates {
		userdataDir := filepath.Join(c.root, "userdata")
		info, err := os.Stat(userdataDir)
		if err != nil || !info.IsDir() {
			continue
		}

		realUserdata := userdataDir
		if resolved, err := filepath.EvalSymlinks(userdataDir); err == nil {
			realUserdata = resolved
		}
		if _, dup := seenUserdata[realUserdata]; dup {
			continue
		}
		seenUserdata[realUserdata] = struct{}{}

		entries, err := os.ReadDir(realUserdata)
		if err != nil {
			continue
		}
		for _, e := range entries {
			if !e.IsDir() || e.Name() == "0" || !isAllDigits(e.Name()) {
				continue
			}
			accounts = append(accounts, steamAccount{
				steamRoot: c.root,
				configDir: filepath.Join(realUserdata, e.Name(), "config"),
				isFlatpak: c.isFlatpak,
			})
		}
	}

	if len(accounts) == 0 {
		return nil, errors.New("не найдены профили Steam (userdata). Установите Steam и войдите в аккаунт хотя бы один раз")
	}
	return accounts, nil
}

func isAllDigits(s string) bool {
	if s == "" {
		return false
	}
	for i := 0; i < len(s); i++ {
		if s[i] < '0' || s[i] > '9' {
			return false
		}
	}
	return true
}

func iconPathForSteam(acc steamAccount, realIconPath, homeDir string) string {
	if realIconPath == "" {
		return ""
	}
	// Внутри песочницы Flatpak Steam путь .var/app/com.valvesoftware.Steam/... смонтирован как $HOME/...
	if acc.isFlatpak {
		flatpakHome := filepath.Join(homeDir, ".var", "app", "com.valvesoftware.Steam")
		if rel, err := filepath.Rel(flatpakHome, realIconPath); err == nil && !strings.HasPrefix(rel, "..") {
			return filepath.Join(homeDir, rel)
		}
	}
	return realIconPath
}

func extractGridAssets(gridDir string, appID uint32, steamAssets, iconBytes []byte) (string, error) {
	if err := os.MkdirAll(gridDir, 0755); err != nil {
		return "", err
	}

	if len(steamAssets) > 0 {
		gzr, err := gzip.NewReader(bytes.NewReader(steamAssets))
		if err != nil {
			return "", fmt.Errorf("не удалось открыть gzip архив steam-asets: %w", err)
		}
		defer gzr.Close()

		tr := tar.NewReader(gzr)
		for {
			hdr, err := tr.Next()
			if errors.Is(err, io.EOF) {
				break
			}
			if err != nil {
				return "", fmt.Errorf("ошибка чтения tar архива steam-asets: %w", err)
			}
			if hdr.Typeflag != tar.TypeReg {
				continue
			}

			base := filepath.Base(hdr.Name)
			if strings.HasPrefix(base, ".") {
				continue
			}

			targetName := mapGridFilename(base, appID)
			if targetName == "" {
				continue
			}

			dstPath := filepath.Join(gridDir, targetName)
			data, err := io.ReadAll(tr)
			if err != nil {
				return "", fmt.Errorf("ошибка распаковки %s: %w", base, err)
			}
			if err := os.WriteFile(dstPath, data, 0644); err != nil {
				return "", err
			}
		}
	}

	var iconPath string
	if len(iconBytes) > 0 {
		iconPath = filepath.Join(gridDir, fmt.Sprintf("%d_icon.png", appID))
		if err := os.WriteFile(iconPath, iconBytes, 0644); err != nil {
			return "", err
		}
	}

	return iconPath, nil
}

func mapGridFilename(origName string, appID uint32) string {
	ext := strings.ToLower(filepath.Ext(origName))
	if ext != ".png" && ext != ".jpg" && ext != ".jpeg" && ext != ".webp" {
		return ""
	}
	stem := strings.TrimSuffix(origName, filepath.Ext(origName))
	lowerStem := strings.ToLower(stem)

	switch {
	case strings.HasSuffix(lowerStem, "_hero"):
		return fmt.Sprintf("%d_hero%s", appID, ext)
	case strings.HasSuffix(lowerStem, "_logo"):
		return fmt.Sprintf("%d_logo%s", appID, ext)
	case isPortraitStem(lowerStem):
		return fmt.Sprintf("%dp%s", appID, ext)
	default:
		return fmt.Sprintf("%d%s", appID, ext)
	}
}

func isPortraitStem(lowerStem string) bool {
	if lowerStem == "p" || strings.HasSuffix(lowerStem, ">p") || strings.HasSuffix(lowerStem, "_p") {
		return true
	}
	if len(lowerStem) >= 2 && lowerStem[len(lowerStem)-1] == 'p' {
		prev := lowerStem[len(lowerStem)-2]
		return prev >= '0' && prev <= '9'
	}
	return false
}

// ==========================================
// Чтение, обновление и запись shortcuts.vdf
// ==========================================

func upsertShortcut(vdfPath string, sc shortcutData) error {
	if err := os.MkdirAll(filepath.Dir(vdfPath), 0755); err != nil {
		return err
	}

	var root *vdfNode
	raw, err := os.ReadFile(vdfPath)
	if err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			return err
		}
		root = &vdfNode{
			Fields: []*vdfField{
				{Type: vdfTypeMap, Key: "shortcuts", MapVal: &vdfNode{}},
			},
		}
	} else {
		// Сохраняем резервную копию перед модификацией
		_ = os.WriteFile(vdfPath+".bak", raw, 0644)

		root, err = parseVDF(raw)
		if err != nil {
			return fmt.Errorf("не удалось распарсить существующий shortcuts.vdf: %w", err)
		}
	}

	shortcutsField := root.findField("shortcuts")
	if shortcutsField == nil || shortcutsField.Type != vdfTypeMap || shortcutsField.MapVal == nil {
		shortcutsField = &vdfField{Type: vdfTypeMap, Key: "shortcuts", MapVal: &vdfNode{}}
		root.Fields = append(root.Fields, shortcutsField)
	}
	shortcuts := shortcutsField.MapVal

	// Ищем уже существующий ярлык RFAD SE, чтобы обновить его на месте
	var targetEntry *vdfNode
	for _, f := range shortcuts.Fields {
		if f.Type != vdfTypeMap || f.MapVal == nil {
			continue
		}
		existingName := f.MapVal.getString("AppName")
		existingExe := f.MapVal.getString("Exe")
		if strings.EqualFold(existingName, sc.AppName) || (existingExe != "" && existingExe == sc.Exe) {
			targetEntry = f.MapVal
			break
		}
	}

	if targetEntry != nil {
		targetEntry.setInt("appid", sc.AppID)
		targetEntry.setString("AppName", sc.AppName)
		targetEntry.setString("Exe", sc.Exe)
		targetEntry.setString("StartDir", sc.StartDir)
		targetEntry.setString("icon", sc.Icon)
		targetEntry.setString("LaunchOptions", sc.LaunchOptions)
		targetEntry.setString("FlatpakAppID", sc.FlatpakAppID)
		targetEntry.setInt("AllowDesktopConfig", 1)
		targetEntry.setInt("AllowOverlay", 1)
	} else {
		nextIdx := nextShortcutIndex(shortcuts)
		entry := &vdfNode{
			Fields: []*vdfField{
				{Type: vdfTypeInt32, Key: "appid", IntVal: sc.AppID},
				{Type: vdfTypeString, Key: "AppName", StrVal: sc.AppName},
				{Type: vdfTypeString, Key: "Exe", StrVal: sc.Exe},
				{Type: vdfTypeString, Key: "StartDir", StrVal: sc.StartDir},
				{Type: vdfTypeString, Key: "icon", StrVal: sc.Icon},
				{Type: vdfTypeString, Key: "ShortcutPath", StrVal: ""},
				{Type: vdfTypeString, Key: "LaunchOptions", StrVal: sc.LaunchOptions},
				{Type: vdfTypeInt32, Key: "IsHidden", IntVal: 0},
				{Type: vdfTypeInt32, Key: "AllowDesktopConfig", IntVal: 1},
				{Type: vdfTypeInt32, Key: "AllowOverlay", IntVal: 1},
				{Type: vdfTypeInt32, Key: "OpenVR", IntVal: 0},
				{Type: vdfTypeInt32, Key: "Devkit", IntVal: 0},
				{Type: vdfTypeString, Key: "DevkitGameID", StrVal: ""},
				{Type: vdfTypeInt32, Key: "DevkitOverrideAppID", IntVal: 0},
				{Type: vdfTypeInt32, Key: "LastPlayTime", IntVal: 0},
				{Type: vdfTypeString, Key: "FlatpakAppID", StrVal: sc.FlatpakAppID},
				{Type: vdfTypeMap, Key: "tags", MapVal: &vdfNode{}},
			},
		}
		shortcuts.Fields = append(shortcuts.Fields, &vdfField{
			Type:   vdfTypeMap,
			Key:    strconv.Itoa(nextIdx),
			MapVal: entry,
		})
	}

	out := serializeVDF(root)
	tmpPath := vdfPath + ".tmp"
	if err := os.WriteFile(tmpPath, out, 0644); err != nil {
		return err
	}
	return os.Rename(tmpPath, vdfPath)
}

func nextShortcutIndex(shortcuts *vdfNode) int {
	maxIdx := -1
	for _, f := range shortcuts.Fields {
		if idx, err := strconv.Atoi(f.Key); err == nil && idx > maxIdx {
			maxIdx = idx
		}
	}
	return maxIdx + 1
}

func (n *vdfNode) findField(key string) *vdfField {
	for _, f := range n.Fields {
		if strings.EqualFold(f.Key, key) {
			return f
		}
	}
	return nil
}

func (n *vdfNode) getString(key string) string {
	if f := n.findField(key); f != nil && f.Type == vdfTypeString {
		return f.StrVal
	}
	return ""
}

func (n *vdfNode) setString(key, val string) {
	if f := n.findField(key); f != nil {
		f.Type = vdfTypeString
		f.StrVal = val
		return
	}
	n.Fields = append(n.Fields, &vdfField{Type: vdfTypeString, Key: key, StrVal: val})
}

func (n *vdfNode) setInt(key string, val uint32) {
	if f := n.findField(key); f != nil {
		f.Type = vdfTypeInt32
		f.IntVal = val
		return
	}
	n.Fields = append(n.Fields, &vdfField{Type: vdfTypeInt32, Key: key, IntVal: val})
}

func parseVDF(data []byte) (*vdfNode, error) {
	r := bytes.NewReader(data)
	return parseVDFNode(r)
}

func parseVDFNode(r *bytes.Reader) (*vdfNode, error) {
	node := &vdfNode{}
	for {
		t, err := r.ReadByte()
		if errors.Is(err, io.EOF) || t == vdfTypeEnd {
			return node, nil
		}
		if err != nil {
			return nil, err
		}

		key, err := readCString(r)
		if err != nil {
			return nil, err
		}

		switch t {
		case vdfTypeMap:
			child, err := parseVDFNode(r)
			if err != nil {
				return nil, err
			}
			node.Fields = append(node.Fields, &vdfField{Type: t, Key: key, MapVal: child})
		case vdfTypeString:
			val, err := readCString(r)
			if err != nil {
				return nil, err
			}
			node.Fields = append(node.Fields, &vdfField{Type: t, Key: key, StrVal: val})
		case vdfTypeInt32:
			var val uint32
			if err := binary.Read(r, binary.LittleEndian, &val); err != nil {
				return nil, err
			}
			node.Fields = append(node.Fields, &vdfField{Type: t, Key: key, IntVal: val})
		case vdfTypeFloat32, vdfTypePointer, vdfTypeColor:
			raw := make([]byte, 4)
			if _, err := io.ReadFull(r, raw); err != nil {
				return nil, err
			}
			node.Fields = append(node.Fields, &vdfField{Type: t, Key: key, RawVal: raw})
		case vdfTypeUint64:
			raw := make([]byte, 8)
			if _, err := io.ReadFull(r, raw); err != nil {
				return nil, err
			}
			node.Fields = append(node.Fields, &vdfField{Type: t, Key: key, RawVal: raw})
		default:
			return nil, fmt.Errorf("неизвестный тип VDF 0x%02x для ключа %q", t, key)
		}
	}
}

func readCString(r *bytes.Reader) (string, error) {
	var buf []byte
	for {
		b, err := r.ReadByte()
		if err != nil {
			return "", err
		}
		if b == 0x00 {
			return string(buf), nil
		}
		buf = append(buf, b)
	}
}

func serializeVDF(root *vdfNode) []byte {
	var buf bytes.Buffer
	writeVDFNode(&buf, root)
	return buf.Bytes()
}

func writeVDFNode(buf *bytes.Buffer, node *vdfNode) {
	for _, f := range node.Fields {
		buf.WriteByte(f.Type)
		buf.WriteString(f.Key)
		buf.WriteByte(0x00)

		switch f.Type {
		case vdfTypeMap:
			if f.MapVal != nil {
				writeVDFNode(buf, f.MapVal)
			} else {
				buf.WriteByte(vdfTypeEnd)
			}
		case vdfTypeString:
			buf.WriteString(f.StrVal)
			buf.WriteByte(0x00)
		case vdfTypeInt32:
			_ = binary.Write(buf, binary.LittleEndian, f.IntVal)
		default:
			buf.Write(f.RawVal)
		}
	}
	buf.WriteByte(vdfTypeEnd)
}
