#!/bin/bash
set -e

# Базовые переменные
PACKAGE_NAME="RFADLauncherLinux"
BASE_URL="https://kraito585.github.io/Rfad-launcher-linux-fork/package"
FLATPAK_REPO_URL="$BASE_URL/repo"

echo "========================================"
echo "  Установка RFAD Launcher"
echo "========================================"

# 1. Проверка на Steam Deck (SteamOS) и неизменяемые системы (OSTree)
if grep -q "SteamOS" /etc/os-release 2>/dev/null || [ -f /run/ostree-booted ] || findmnt -n -o OPTIONS / | grep -Eq '\bro\b'; then
    echo "[+] Обнаружена неизменяемая система (Steam Deck / OSTree)."
    if command -v flatpak >/dev/null 2>&1; then
        echo "=> Подключение Flatpak репозитория..."
        flatpak remote-add --user --if-not-exists rfad-repo "$FLATPAK_REPO_URL"
        echo "=> Установка лаунчера..."
        flatpak install --user -y rfad-repo io.rfad.Launcher
        echo "✅ Готово! Лаунчер успешно установлен."
        exit 0
    else
        echo "❌ Ошибка: Flatpak не установлен в системе!"
        exit 1
    fi
fi

# 2. Arch Linux / CachyOS / Manjaro (Pacman)
if command -v pacman >/dev/null 2>&1; then
    echo "[+] Обнаружен Arch Linux / Pacman."
    
    # Проверяем, добавлен ли уже репозиторий [rfad]
    if ! grep -q "^\[rfad\]" /etc/pacman.conf; then
        echo "=> Подключение репозитория [rfad] в /etc/pacman.conf..."
        echo -e "\n[rfad]\nSigLevel = Optional TrustAll\nServer = $BASE_URL/pacman/x86_64" | sudo tee -a /etc/pacman.conf > /dev/null
    else
        echo "=> Репозиторий [rfad] уже подключен."
    fi
    
    echo "=> Синхронизация баз данных и установка..."
    sudo pacman -Sy --noconfirm "$PACKAGE_NAME"
    echo "✅ Готово! Лаунчер успешно установлен."
    exit 0
fi

# 3. Ubuntu / Debian / Linux Mint (APT)
if command -v apt-get >/dev/null 2>&1; then
    echo "[+] Обнаружен Debian / Ubuntu (APT)."
    echo "=> Подключение APT репозитория..."
    echo "deb [trusted=yes] $BASE_URL/apt stable main" | sudo tee /etc/apt/sources.list.d/rfad-launcher.list > /dev/null
    
    echo "=> Обновление индексов и установка..."
    sudo apt-get update
    # APT иногда приводит имена пакетов к нижнему регистру, поэтому используем apt-cache для надежности
    sudo apt-get install -y "$PACKAGE_NAME" || sudo apt-get install -y rfadlauncherlinux
    echo "✅ Готово! Лаунчер успешно установлен."
    exit 0
fi

# 4. Fedora / RedHat / Nobara (DNF)
if command -v dnf >/dev/null 2>&1; then
    echo "[+] Обнаружена Fedora (DNF)."
    echo "=> Подключение RPM репозитория..."
    cat <<EOF | sudo tee /etc/yum.repos.d/rfad-launcher.repo > /dev/null
[rfad-launcher]
name=RFAD Launcher Repository
baseurl=$BASE_URL/rpm/
enabled=1
gpgcheck=0
EOF
    
    echo "=> Установка пакета..."
    sudo dnf install -y "$PACKAGE_NAME"
    echo "✅ Готово! Лаунчер успешно установлен."
    exit 0
fi

# 5. Резервный вариант (Fallback) на Flatpak для остальных дистрибутивов
if command -v flatpak >/dev/null 2>&1; then
    echo "[?] Пакетный менеджер не распознан. Переход к универсальной установке через Flatpak..."
    flatpak remote-add --user --if-not-exists rfad-repo "$FLATPAK_REPO_URL"
    flatpak install --user -y rfad-repo io.rfad.Launcher
    echo "✅ Готово! Лаунчер успешно установлен."
    exit 0
fi

echo "❌ Ошибка: Не удалось найти поддерживаемый пакетный менеджер (pacman, apt, dnf или flatpak)."
exit 1