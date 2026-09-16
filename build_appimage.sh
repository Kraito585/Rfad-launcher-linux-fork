#!/bin/bash
set -e

APP_NAME="RFADLauncherLinux"
BINARY_PATH="bin/$APP_NAME"
ICON_PATH="build/appicon.png"

echo "1. Скачиваем linuxdeploy и appimagetool..."
wget -q https://github.com/linuxdeploy/linuxdeploy/releases/download/continuous/linuxdeploy-x86_64.AppImage
wget -q https://github.com/AppImage/appimagetool/releases/download/continuous/appimagetool-x86_64.AppImage
chmod +x linuxdeploy-x86_64.AppImage appimagetool-x86_64.AppImage

# Распаковываем утилиты (обход ограничений Docker FUSE)
./linuxdeploy-x86_64.AppImage --appimage-extract > /dev/null
mv squashfs-root linuxdeploy-root
./appimagetool-x86_64.AppImage --appimage-extract > /dev/null
mv squashfs-root appimagetool-root

echo "2. Создаем структуру AppDir..."
rm -rf AppDir
mkdir -p AppDir/usr/bin AppDir/usr/share/applications AppDir/usr/share/icons/hicolor/256x256/apps

# Копируем бинарник и иконку
cp "$BINARY_PATH" AppDir/usr/bin/
cp "$ICON_PATH" AppDir/usr/share/icons/hicolor/256x256/apps/${APP_NAME}.png

echo "3. Генерируем .desktop файл..."
cat <<EOF > AppDir/usr/share/applications/${APP_NAME}.desktop
[Desktop Entry]
Type=Application
Name=RFAD Launcher
Exec=$APP_NAME
Icon=$APP_NAME
Categories=Utility;
EOF

echo "4. Вызываем linuxdeploy для сбора зависимостей..."
export NO_STRIP=1
./linuxdeploy-root/AppRun --appdir AppDir

echo "5. Ручной фикс WebKit6 (GTK4)..."
# linuxdeploy кладет библиотеки в usr/lib/, значит процессы должны лежать в usr/lib/webkitgtk-6.0/
TARGET_DIR="AppDir/usr/lib/webkitgtk-6.0"
mkdir -p "$TARGET_DIR"

# Ищем, где Debian 13 хранит процессы WebKit
WEBKIT_EXEC=$(find /usr/libexec /usr/lib -name "WebKitNetworkProcess" -type f 2>/dev/null | grep "webkitgtk-6.0" | head -n 1)
WEBKIT_DIR=$(dirname "$WEBKIT_EXEC")

# Копируем процессы и плагины
cp -r "$WEBKIT_DIR"/* "$TARGET_DIR/"
cp -r /usr/lib/x86_64-linux-gnu/webkitgtk-6.0/* "$TARGET_DIR/" 2>/dev/null || true

echo "6. Собираем финальный AppImage..."
ARCH=x86_64 ./appimagetool-root/AppRun AppDir bin/rfadlauncherlinux-x86_64.AppImage

echo "Готово! Кастомный AppImage успешно собран."
