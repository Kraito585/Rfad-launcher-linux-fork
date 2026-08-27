#!/bin/bash
# Запуск игры через Wine из Proton

GAME_ROOT="${GAME_ROOT:-}"
WINE_BIN="${WINE_BIN:-}"
EXE_PATH="${EXE_PATH:-}"
PREFIX_PATH="${PREFIX_PATH:-}"
MO2_ARGS="${MO2_ARGS:-}"
USE_GAMEMODE="${USE_GAMEMODE:-false}"

if [ -z "$WINE_BIN" ] || [ -z "$EXE_PATH" ] || [ -z "$PREFIX_PATH" ]; then
    echo "Ошибка: не заданы обязательные пути"
    exit 1
fi

export WINEPREFIX="$PREFIX_PATH"
export WINEDLLOVERRIDES="concrt140=n;xaudio2_7=n,b;d3d11=n,b;dxgi=n,b;d3dx9_42=n,b;d3dcompiler_47=n,b;dinput8=n,b;mscoree=n"
export DXVK_ASYNC=1
export PROTON_ENABLE_NVAPI=1
export DXVK_ENABLE_NVAPI=1

export WINEDLLPATH="$WINEDLLPATH"
export LD_LIBRARY_PATH="$LD_LIBRARY_PATH"
export PATH="$PATH"

cd "$(dirname "$EXE_PATH")" || exit

if [ ! -x "$WINE_BIN" ]; then
    echo "Ошибка: wine не найден в $WINE_BIN"
    exit 1
fi

# Запуск с gamemode, если включено и пакет установлен
if [ "$USE_GAMEMODE" = "true" ] && command -v gamemoderun &>/dev/null; then
    exec gamemoderun "$WINE_BIN" "$EXE_PATH" $MO2_ARGS
else
    exec "$WINE_BIN" "$EXE_PATH" $MO2_ARGS
fi