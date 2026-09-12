FROM ubuntu:22.04

# Отключаем интерактивные запросы и настраиваем AppImage
ENV DEBIAN_FRONTEND=noninteractive
ENV APPIMAGE_EXTRACT_AND_RUN=1
ENV PATH=$PATH:/usr/local/go/bin:/root/go/bin

# 1. Устанавливаем системные зависимости
RUN apt-get update && apt-get install -y \
    curl wget git build-essential pkg-config \
    libgtk-3-dev libwebkit2gtk-4.1-dev desktop-file-utils file libglib2.0-bin

# 2. Устанавливаем Node.js 20
RUN curl -fsSL https://deb.nodesource.com/setup_20.x | bash - && \
    apt-get install -y nodejs

# 3. Устанавливаем Go
RUN curl -fsSL https://go.dev/dl/go1.22.5.linux-amd64.tar.gz | tar -C /usr/local -xzf -

# 4. Устанавливаем Wails CLI
RUN GOPROXY=direct go install -v github.com/wailsapp/wails/v2/cmd/wails@latest  

# 5. Скачиваем утилиты linuxdeploy (чтобы не качать их при каждой сборке)
WORKDIR /tools
RUN wget https://github.com/linuxdeploy/linuxdeploy/releases/download/continuous/linuxdeploy-x86_64.AppImage && \
    wget https://raw.githubusercontent.com/linuxdeploy/linuxdeploy-plugin-gtk/master/linuxdeploy-plugin-gtk.sh && \
    chmod +x linuxdeploy-x86_64.AppImage linuxdeploy-plugin-gtk.sh

# Рабочая директория, куда мы будем монтировать код
WORKDIR /src

# Скрипт сборки, который запускается при старте контейнера
CMD GOPROXY=direct wails build -platform linux/amd64 -m && \
    mkdir -p AppDir/usr/bin AppDir/usr/share/applications AppDir/usr/share/icons/hicolor/256x256/apps && \
    cp build/bin/RFADLauncherLinux AppDir/usr/bin/RFADLauncherLinux && \
    cp build/appicon.png AppDir/rfad.png && \
    cp build/appicon.png AppDir/usr/share/icons/hicolor/256x256/apps/rfad.png && \
    printf "[Desktop Entry]\nName=RFAD Launcher\nExec=RFADLauncherLinux\nIcon=rfad\nType=Application\nCategories=Game;\n" > AppDir/usr/share/applications/rfad.desktop && \
    /tools/linuxdeploy-x86_64.AppImage --appdir AppDir -e AppDir/usr/bin/RFADLauncherLinux -d AppDir/usr/share/applications/rfad.desktop -i AppDir/rfad.png --plugin gtk && \
    echo "Удаляем жестко зашитый WebKit из сборки..." && \
    rm -f AppDir/usr/lib/x86_64-linux-gnu/libwebkit2gtk* && \
    rm -f AppDir/usr/lib/x86_64-linux-gnu/libjavascriptcoregtk* && \
    /tools/linuxdeploy-x86_64.AppImage --appdir AppDir --output appimage && \
    chown 1000:1000 RFAD-Launcher-x86_64.AppImage