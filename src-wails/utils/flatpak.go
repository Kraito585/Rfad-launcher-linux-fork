package utils

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func IsFlatpak() bool {
	_, err := os.Stat("/.flatpak-info")
	return err == nil
}

// sanitizeHostEnv очищает переменные окружения песочницы Flatpak,
// сохраняя пользовательские переопределения (WINEPREFIX, WINE, пути к wine/bin и wine/lib).
func sanitizeHostEnv(env []string) []string {
	var cleaned []string

	blockedPrefixes := []string{
		"DBUS_SESSION_BUS_ADDRESS=",
		"FLATPAK_",
		"container=",
		"TMPDIR=",
		"XDG_DATA_DIRS=",
		"XDG_CONFIG_DIRS=",
		"GSETTINGS_SCHEMA_DIR=",
		"GI_TYPELIB_PATH=",
		"GST_PLUGIN_PATH=",
		"GST_PLUGIN_SYSTEM_PATH=",
		"GST_REGISTRY=",
		"QT_PLUGIN_PATH=",
		"QML2_IMPORT_PATH=",
		"LD_PRELOAD=",
	}

	for _, e := range env {
		skip := false
		for _, prefix := range blockedPrefixes {
			if strings.HasPrefix(e, prefix) {
				skip = true
				break
			}
		}
		if skip {
			continue
		}

		// Вырезаем пути Flatpak runtime из LD_LIBRARY_PATH, оставляя только пути игры/wine
		if strings.HasPrefix(e, "LD_LIBRARY_PATH=") {
			val := strings.TrimPrefix(e, "LD_LIBRARY_PATH=")
			var validPaths []string
			for _, p := range strings.Split(val, ":") {
				if p == "" || strings.HasPrefix(p, "/app") || strings.HasPrefix(p, "/usr/lib/") {
					continue
				}
				validPaths = append(validPaths, p)
			}
			if len(validPaths) > 0 {
				cleaned = append(cleaned, "LD_LIBRARY_PATH="+strings.Join(validPaths, ":"))
			}
			continue
		}

		// Вырезаем /app и /usr/lib/sdk из PATH и гарантируем наличие стандартных путей хоста
		if strings.HasPrefix(e, "PATH=") {
			val := strings.TrimPrefix(e, "PATH=")
			var validPaths []string
			for _, p := range strings.Split(val, ":") {
				if p == "" || strings.HasPrefix(p, "/app") || strings.HasPrefix(p, "/usr/lib/sdk") {
					continue
				}
				validPaths = append(validPaths, p)
			}
			hostDefaults := []string{"/usr/local/sbin", "/usr/local/bin", "/usr/sbin", "/usr/bin", "/sbin", "/bin"}
			for _, def := range hostDefaults {
				found := false
				for _, vp := range validPaths {
					if vp == def {
						found = true
						break
					}
				}
				if !found {
					validPaths = append(validPaths, def)
				}
			}
			cleaned = append(cleaned, "PATH="+strings.Join(validPaths, ":"))
			continue
		}

		cleaned = append(cleaned, e)
	}

	return cleaned
}

func buildHostCommandArgs(env []string, name string, args ...string) []string {
	flatpakArgs := []string{"--host", "env"}
	flatpakArgs = append(flatpakArgs, sanitizeHostEnv(env)...)
	flatpakArgs = append(flatpakArgs, name)
	flatpakArgs = append(flatpakArgs, args...)
	return flatpakArgs
}

func NewHostCommand(env []string, name string, args ...string) *exec.Cmd {
	if IsFlatpak() {
		return exec.Command("flatpak-spawn", buildHostCommandArgs(env, name, args...)...)
	}

	cmd := exec.Command(name, args...)
	if len(env) > 0 {
		cmd.Env = env
	}
	return cmd
}

func NewHostCommandContext(ctx context.Context, env []string, name string, args ...string) *exec.Cmd {
	if IsFlatpak() {
		return exec.CommandContext(ctx, "flatpak-spawn", buildHostCommandArgs(env, name, args...)...)
	}

	cmd := exec.CommandContext(ctx, name, args...)
	if len(env) > 0 {
		cmd.Env = env
	}
	return cmd
}

func HostCommandExists(name string) bool {
	if IsFlatpak() {
		err := exec.Command("flatpak-spawn", "--host", "sh", "-c", `command -v "$1" >/dev/null 2>&1`, "_", name).Run()
		return err == nil
	}
	_, err := exec.LookPath(name)
	return err == nil
}

// HostHardLink пытается создать жесткую ссылку.
// Если мы во Flatpak, команда отправляется на хост для обхода виртуальной файловой системы.
func HostHardLink(src, dst string) error {
	absSrc, err := filepath.Abs(src)
	if err != nil {
		absSrc = src
	}
	absDst, err := filepath.Abs(dst)
	if err != nil {
		absDst = dst
	}

	if IsFlatpak() {
		cmd := exec.Command("flatpak-spawn", "--host", "ln", "-f", absSrc, absDst)
		out, err := cmd.CombinedOutput()
		if err != nil {
			slog.Debug("HostHardLink не удался на хосте", "out", string(out), "err", err)
			return fmt.Errorf("host ln error: %v, %s", err, string(out))
		}
		return nil
	}

	return os.Link(absSrc, absDst)
}
