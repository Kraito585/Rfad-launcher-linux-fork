#!/bin/bash
set -e

run_with_watchdog() {
    local max_retries=5
    local idle_timeout=300
    local cmd="$1"
    local attempt=1

    while [ $attempt -le $max_retries ]; do
        echo "=> Запуск (Попытка $attempt/$max_retries): $cmd"
        
        local log_file=$(mktemp)
        
        eval "$cmd" > >(tee -a "$log_file") 2>&1 &
        local cmd_pid=$!
        local timeout_triggered=0
        
        while kill -0 $cmd_pid 2>/dev/null; do
            sleep 5
            # Получаем Unix timestamp последнего изменения лог-файла
            local last_mod=$(stat -c %Y "$log_file" 2>/dev/null || echo $(date +%s))
            local current_time=$(date +%s)
            local diff=$((current_time - last_mod))
            
            if [ $diff -ge $idle_timeout ]; then
                echo "!!! ВНИМАНИЕ: Обнаружено зависание (нет вывода $idle_timeout сек). Прерывание..."
                # Жестко убиваем сам процесс и все возможные зависшие дочерние утилиты
                kill -9 $cmd_pid 2>/dev/null || true
                pkill -9 -P $cmd_pid 2>/dev/null || true
                pkill -9 -f "node|npm|npx|wails3|go|flatpak-builder" 2>/dev/null || true
                timeout_triggered=1
                break
            fi
        done
        
        wait $cmd_pid 2>/dev/null
        local exit_code=$?
        
        if [ $timeout_triggered -eq 0 ] && [ $exit_code -eq 0 ]; then
            rm -f "$log_file"
            return 0
        fi
        
        echo "=> Сбой этапа. Очистка кэшей перед следующей попыткой..."
        rm -f "$log_file"
        rm -rf frontend/node_modules frontend/.nuxt .flatpak-builder 2>/dev/null || true
        
        attempt=$((attempt + 1))
        sleep 5
    done
    
    echo "КРИТИЧЕСКАЯ ОШИБКА: Команда '$cmd' не удалась после $max_retries попыток."
    exit 1
}

run_build() {
    export NUXT_TELEMETRY_DISABLED=1
    export CI=true

    # Устанавливаем версию (фоллбэк на localbuild, если переменная пуста)
    export APP_VERSION="${APP_VERSION#v}"
    export APP_VERSION="${APP_VERSION:-localbuild}"

    echo "=== 0. Настройка версионирования ==="
    echo "Используемая версия: $APP_VERSION"
    # 1. Инъекция в бинарник
    if [ -f "main.go" ]; then
        sed -i "s/var version = \".*\"/var version = \"$APP_VERSION\"/" main.go
        echo "Файл main.go успешно обновлен."
    fi

    # 2. Инъекция в метаданные nFPM (заменяем любую версию на нашу)
    if [ -f "build/linux/nfpm/nfpm.yaml" ]; then
        sed -i -E "s/^[[:space:]]*version:.*/version: \"$APP_VERSION\"/" build/linux/nfpm/nfpm.yaml
        echo "Файл nfpm.yaml успешно обновлен."
    fi

    # 3. Инъекция в ярлык для нативных пакетов (добавляем X-App-Version)
    if [ -f "build/linux/RFADLauncherLinux.desktop" ]; then
        # Удаляем старую строку X-App-Version (если была), чтобы не дублировать
        sed -i '/^X-App-Version=/d' build/linux/RFADLauncherLinux.desktop
        # Дописываем нашу актуальную версию в конец файла
        echo "X-App-Version=$APP_VERSION" >> build/linux/RFADLauncherLinux.desktop
        echo "Файл RFADLauncherLinux.desktop успешно обновлен."
    fi

    echo "=== 1. Подготовка системы и базовых утилит ==="
    apt-get update -y
    apt-get install -y curl ca-certificates gnupg file build-essential pkg-config dbus dbus-x11

    echo "=== 2. Установка зависимостей Wails (GTK4 + WebKit6) ==="
    apt-get install -y libgtk-4-dev libwebkitgtk-6.0-dev
    
    echo "=== 3. Установка инструментов Flatpak ==="
    apt-get install -y flatpak flatpak-builder elfutils

    echo "=== 4. Установка NodeJS ==="
    curl -fsSL https://deb.nodesource.com/setup_26.x | bash -
    apt-get install -y nodejs

    echo "=== 5. Установка Wails CLI ==="
    go install github.com/wailsapp/wails/v3/cmd/wails3@v3.0.0-beta.24
    export PATH=$PATH:$(go env GOPATH)/bin

    echo "=== 6. Сборка нативных пакетов Wails (DEB, RPM, ZST) ==="
    unset GOFLAGS
    rm -rf bin/
    # Wails/nfpm подхватят переменную APP_VERSION автоматически
    run_with_watchdog "wails3 task linux:package"
    rm -f bin/*.AppImage

    echo "=== 7. Настройка окружения Flatpak ==="
    service dbus start || /etc/init.d/dbus start || true
    
    flatpak remote-add --user --if-not-exists flathub https://dl.flathub.org/repo/flathub.flatpakrepo
    flatpak install --user -y flathub org.gnome.Platform//46 org.gnome.Sdk//46 org.freedesktop.Sdk.Extension.golang//23.08

    echo "=== 8. Создание манифеста Flatpak ==="
    # Используем cat << EOF (без кавычек), чтобы bash мог вставить переменную $APP_VERSION внутрь ярлыка
    cat << EOF > io.rfad.Launcher.yml
app-id: io.rfad.Launcher
runtime: org.gnome.Platform
runtime-version: '46'
sdk: org.gnome.Sdk
command: RFADLauncherLinux
finish-args:
  - --share=network
  - --socket=wayland
  - --socket=fallback-x11
  - --device=dri
  - --filesystem=host
  - --talk-name=org.freedesktop.Flatpak
  - --own-name=org.wails.*
sdk-extensions:
  - org.freedesktop.Sdk.Extension.golang
build-options:
  append-path: /usr/lib/sdk/golang/bin
  build-args:
    - --share=network
  env:
    GOPROXY: https://proxy.golang.org
    GO111MODULE: on
modules:
  - name: rfad-launcher
    buildsystem: simple

    build-options:
      build-args:
        - --share=network
      env:
        GOPROXY: https://proxy.golang.org,direct
        
    build-commands:
      - go build -tags production -trimpath -o RFADLauncherLinux .
      - install -D RFADLauncherLinux /app/bin/RFADLauncherLinux
      - install -Dm644 build/appicon.png /app/share/icons/hicolor/256x256/apps/io.rfad.Launcher.png
      - mkdir -p /app/share/applications
      - |
        cat <<APP_EOF > /app/share/applications/io.rfad.Launcher.desktop
        [Desktop Entry]
        Name=RFAD Launcher
        Exec=RFADLauncherLinux
        Icon=io.rfad.Launcher
        Type=Application
        Categories=Utility;Game;
        X-App-Version=$APP_VERSION
        APP_EOF
    sources:
      - type: dir
        path: .
EOF

    echo "=== 9. Сборка Flatpak пакета ==="
    run_with_watchdog "flatpak-builder --user --disable-rofiles-fuse --force-clean flatpak-build io.rfad.Launcher.yml"
    
    flatpak build-export repo flatpak-build
    flatpak build-bundle repo bin/RFADLauncherLinux-x86_64.flatpak io.rfad.Launcher

    echo "=== Внутренняя сборка завершена! Итоговые артефакты: ==="
    ls -lh bin/
}

# Логика изоляции
export APP_VERSION="${APP_VERSION#v}"
export APP_VERSION="${APP_VERSION:-localbuild}"

if [ "$1" == "--internal" ]; then
    run_build
else
    echo "=== Поднятие Docker-песочницы для безопасной сборки ==="
    mkdir -p bin
    
    # Пробрасываем APP_VERSION внутрь контейнера через флаг -e
    tar -cf - --exclude=bin . | docker run --rm -i --privileged --network host \
      -e APP_VERSION="$APP_VERSION" \
      -v "$(pwd)/bin":/export \
      -w /app \
      golang:1.26-trixie \
      /bin/bash -c "
        echo '=> Получение исходного кода...' && \
        tar -xf - && \
        chmod +x build_linux.sh && \
        ./build_linux.sh --internal && \
        echo '=> Экспорт артефактов на хост-машину...' && \
        cp -a bin/* /export/
      "
    
    echo "=== Успех! Файлы безопасно выгружены в вашу папку bin/ ==="
fi