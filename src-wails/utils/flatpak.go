package utils

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
)

func IsFlatpak() bool {
	_, err := os.Stat("/.flatpak-info")
	return err == nil
}

func NewHostCommand(env []string, name string, args ...string) *exec.Cmd {
	if IsFlatpak() {
		var flatpakArgs []string

		// Используем системную утилиту env на хосте для абсолютно безопасной
		// инициализации переменных окружения (обходим баги парсера --env=)
		flatpakArgs = append(flatpakArgs, "--host", "env")

		// Передаем переменные напрямую в env (формат "KEY=VALUE")
		for _, e := range env {
			flatpakArgs = append(flatpakArgs, e)
		}

		flatpakArgs = append(flatpakArgs, name)
		flatpakArgs = append(flatpakArgs, args...)

		return exec.Command("flatpak-spawn", flatpakArgs...)
	}

	cmd := exec.Command(name, args...)
	if len(env) > 0 {
		cmd.Env = env
	}
	return cmd
}

func NewHostCommandContext(ctx context.Context, env []string, name string, args ...string) *exec.Cmd {
	if IsFlatpak() {
		var flatpakArgs []string

		// Используем системную утилиту env на хосте
		flatpakArgs = append(flatpakArgs, "--host", "env")

		for _, e := range env {
			flatpakArgs = append(flatpakArgs, e)
		}

		flatpakArgs = append(flatpakArgs, name)
		flatpakArgs = append(flatpakArgs, args...)

		return exec.CommandContext(ctx, "flatpak-spawn", flatpakArgs...)
	}

	cmd := exec.CommandContext(ctx, name, args...)
	if len(env) > 0 {
		cmd.Env = env
	}
	return cmd
}

func HostCommandExists(name string) bool {
	if IsFlatpak() {
		err := exec.Command("flatpak-spawn", "--host", "which", name).Run()
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
		// Используем -f для перезаписи и CombinedOutput для захвата реальной причины ошибки
		cmd := exec.Command("flatpak-spawn", "--host", "ln", "-f", absSrc, absDst)
		out, err := cmd.CombinedOutput()
		if err != nil {
			slog.Debug("HostHardLink не удался на хосте", "out", string(out), "err", err)
			return fmt.Errorf("host ln error: %v, %s", err, string(out))
		}
		return nil
	}

	// Запасной вариант для работы без Flatpak
	return os.Link(absSrc, absDst)
}
