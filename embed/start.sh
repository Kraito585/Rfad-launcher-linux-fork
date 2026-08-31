#!/bin/bash
# Запуск игры через Wine из Proton

# Базовые пути
GAME_ROOT="${GAME_ROOT:-}"
WINE_BIN="${WINE_BIN:-}"
EXE_PATH="${EXE_PATH:-}"
PREFIX_PATH="${PREFIX_PATH:-}"
MO2_ARGS="${MO2_ARGS:-}"

# Новые параметры (с дефолтными значениями)
WINEDLLOVERRIDES_PARAM="${WINEDLLOVERRIDES_PARAM:-concrt140=n;xaudio2_7=n,b;d3d11=n,b;dxgi=n,b;d3dx9_42=n,b;d3dcompiler_47=n,b;dinput8=n,b;mscoree=n;d3d12=n,b;d3d12core=n,b}"
ENABLE_NVAPI="${ENABLE_NVAPI:-false}"
ENABLE_HDR="${ENABLE_HDR:-false}"
ENABLE_FSR="${ENABLE_FSR:-false}"
ENABLE_MANGOHUD="${ENABLE_MANGOHUD:-false}"
ENABLE_SHADER_CACHE="${ENABLE_SHADER_CACHE:-true}"
USE_GAMEMODE="${USE_GAMEMODE:-false}"
STEAM_FIX_ENABLED="${STEAM_FIX_ENABLED:-false}"

if [ -z "$WINE_BIN" ] || [ -z "$EXE_PATH" ] || [ -z "$PREFIX_PATH" ]; then
    echo "Ошибка: не заданы обязательные пути"
    exit 1
fi

# === Проверка Steam (если Steam Fix включен) ===
if [ "$STEAM_FIX_ENABLED" = "true" ] || [ "$STEAM_FIX_ENABLED" = "1" ]; then
    echo "Steam Fix включен. Проверяем статус Steam..."
    
    # Ищем процесс steam. Флаг -x ищет точное совпадение имени.
    if ! pgrep -x "steam" > /dev/null; then
        echo "Steam не запущен. Попытка фонового запуска..."
        
        # Запускаем Steam полностью отвязанным от текущего скрипта
        nohup steam < /dev/null > /dev/null 2>&1 &
        
        TIMEOUT=300
        ELAPSED=0
        STEAM_FOUND=false
        
        echo "Ожидание процесса steam (до 5 минут)..."
        while [ $ELAPSED -lt $TIMEOUT ]; do
            if pgrep -x "steam" > /dev/null; then
                STEAM_FOUND=true
                echo "Процесс Steam успешно обнаружен в системе."
                break
            fi
            sleep 2
            ELAPSED=$((ELAPSED + 2))
        done
        
        if [ "$STEAM_FOUND" = "false" ]; then
            echo "Критическая ошибка: Процесс Steam не появился по истечении 5 минут. Отмена запуска игры."
            exit 1
        fi
    else
        echo "Steam уже работает."
    fi
fi

# Экспорт базовых переменных
export WINEPREFIX="$PREFIX_PATH"
export WINEDLLOVERRIDES="$WINEDLLOVERRIDES_PARAM"
export DXVK_ASYNC=1

export WINEDLLPATH="$WINEDLLPATH"
export LD_LIBRARY_PATH="$LD_LIBRARY_PATH"
export PATH="$PATH"

# === Применение настроек из лаунчера ===

# NVAPI (для RTX 20+ серии)
if [ "$ENABLE_NVAPI" = "true" ] || [ "$ENABLE_NVAPI" = "1" ]; then
    export PROTON_ENABLE_NVAPI=1
    export DXVK_ENABLE_NVAPI=1
fi

# DXVK HDR
if [ "$ENABLE_HDR" = "true" ] || [ "$ENABLE_HDR" = "1" ]; then
    export DXVK_HDR=1
fi

# Wine Fullscreen FSR
if [ "$ENABLE_FSR" = "true" ] || [ "$ENABLE_FSR" = "1" ]; then
    export WINE_FULLSCREEN_FSR=1
fi

# Shader Cache (Кэш шейдеров)
if [ "$ENABLE_SHADER_CACHE" = "true" ] || [ "$ENABLE_SHADER_CACHE" = "1" ]; then
    export __GL_SHADER_DISK_CACHE=1
    export __GL_SHADER_DISK_CACHE_PATH="$PREFIX_PATH/shadercache"
    export DXVK_STATE_CACHE=1
    export DXVK_STATE_CACHE_PATH="$PREFIX_PATH/shadercache"
else
    export __GL_SHADER_DISK_CACHE=0
    export DXVK_STATE_CACHE=0
fi

# Переход в папку с игрой/MO2
cd "$(dirname "$EXE_PATH")" || exit

if [ ! -x "$WINE_BIN" ]; then
    echo "Ошибка: wine не найден в $WINE_BIN"
    exit 1
fi

# === Формирование цепочки запуска (Prefixing) ===
EXEC_CMD=""

if [ "$USE_GAMEMODE" = "true" ] || [ "$USE_GAMEMODE" = "1" ]; then
    if command -v gamemoderun &>/dev/null; then
        EXEC_CMD="gamemoderun"
    else
        echo "Внимание: gamemoderun не установлен, пропускаем."
    fi
fi

if [ "$ENABLE_MANGOHUD" = "true" ] || [ "$ENABLE_MANGOHUD" = "1" ]; then
    if command -v mangohud &>/dev/null; then
        if [ -z "$EXEC_CMD" ]; then
            EXEC_CMD="mangohud"
        else
            EXEC_CMD="$EXEC_CMD mangohud"
        fi
    else
        echo "Внимание: mangohud не установлен, пропускаем."
    fi
fi

# === Финальный запуск ===
if [ -n "$EXEC_CMD" ]; then
    exec $EXEC_CMD "$WINE_BIN" "$EXE_PATH" $MO2_ARGS
else
    exec "$WINE_BIN" "$EXE_PATH" $MO2_ARGS
fi