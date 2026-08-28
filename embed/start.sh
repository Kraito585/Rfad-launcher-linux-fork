#!/bin/bash
# Запуск игры через Wine из Proton
# === OLD SCRIPT ===
# GAME_ROOT="${GAME_ROOT:-}"
# WINE_BIN="${WINE_BIN:-}"
# EXE_PATH="${EXE_PATH:-}"
# PREFIX_PATH="${PREFIX_PATH:-}"
# MO2_ARGS="${MO2_ARGS:-}"
# USE_GAMEMODE="${USE_GAMEMODE:-false}"

# if [ -z "$WINE_BIN" ] || [ -z "$EXE_PATH" ] || [ -z "$PREFIX_PATH" ]; then
#     echo "Ошибка: не заданы обязательные пути"
#     exit 1
# fi

# export WINEPREFIX="$PREFIX_PATH"
# export WINEDLLOVERRIDES="concrt140=n;xaudio2_7=n,b;d3d11=n,b;dxgi=n,b;d3dx9_42=n,b;d3dcompiler_47=n,b;dinput8=n,b;mscoree=n"
# export DXVK_ASYNC=1
# export PROTON_ENABLE_NVAPI=1
# export DXVK_ENABLE_NVAPI=1

# export WINEDLLPATH="$WINEDLLPATH"
# export LD_LIBRARY_PATH="$LD_LIBRARY_PATH"
# export PATH="$PATH"

# cd "$(dirname "$EXE_PATH")" || exit

# if [ ! -x "$WINE_BIN" ]; then
#     echo "Ошибка: wine не найден в $WINE_BIN"
#     exit 1
# fi

# # Запуск с gamemode, если включено и пакет установлен
# if [ "$USE_GAMEMODE" = "true" ] && command -v gamemoderun &>/dev/null; then
#     exec gamemoderun "$WINE_BIN" "$EXE_PATH" $MO2_ARGS
# else
#     exec "$WINE_BIN" "$EXE_PATH" $MO2_ARGS
# fi

# === NEW SCRIPT ===
# Базовые пути
GAME_ROOT="${GAME_ROOT:-}"
WINE_BIN="${WINE_BIN:-}"
EXE_PATH="${EXE_PATH:-}"
PREFIX_PATH="${PREFIX_PATH:-}"
MO2_ARGS="${MO2_ARGS:-}"

# Новые параметры (с дефолтными значениями на случай, если Go их не передаст)
WINEDLLOVERRIDES_PARAM="${WINEDLLOVERRIDES_PARAM:-concrt140=n;xaudio2_7=n,b;d3d11=n,b;dxgi=n,b;d3dx9_42=n,b;d3dcompiler_47=n,b;dinput8=n,b;mscoree=n}"
ENABLE_NVAPI="${ENABLE_NVAPI:-false}"
ENABLE_HDR="${ENABLE_HDR:-false}"
ENABLE_FSR="${ENABLE_FSR:-false}"
ENABLE_MANGOHUD="${ENABLE_MANGOHUD:-false}"
ENABLE_SHADER_CACHE="${ENABLE_SHADER_CACHE:-true}"
USE_GAMEMODE="${USE_GAMEMODE:-false}"

if [ -z "$WINE_BIN" ] || [ -z "$EXE_PATH" ] || [ -z "$PREFIX_PATH" ]; then
    echo "Ошибка: не заданы обязательные пути"
    exit 1
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
# Утилиты вроде gamemoderun и mangohud должны идти ПЕРЕД командой wine.
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
# Если $EXEC_CMD не пустой, bash подставит "gamemoderun mangohud wine game.exe"
if [ -n "$EXEC_CMD" ]; then
    exec $EXEC_CMD "$WINE_BIN" "$EXE_PATH" $MO2_ARGS
else
    exec "$WINE_BIN" "$EXE_PATH" $MO2_ARGS
fi