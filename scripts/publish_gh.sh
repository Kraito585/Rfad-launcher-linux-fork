#!/bin/bash
set -e

echo "=== Проверка токена GitHub ==="
if [ -z "$GH_TOKEN" ]; then
  echo "КРИТИЧЕСКАЯ ОШИБКА: Токен GitHub не передан!"
  exit 1
fi

# Сохраняем путь к корню репозитория, где лежит папка bin/
WORKSPACE_DIR=$(pwd)

echo "=== Установка зависимостей ==="
apt-get update
apt-get install -y git dpkg-dev apt-utils createrepo-c flatpak ostree pacman-package-manager zstd libarchive-tools

git config --global user.email "ci-bot@rfad.local"
git config --global user.name "CI Bot"

echo "=> Клонирование репозитория..."
git clone "https://github.com/${CI_REPO}.git" gh-pages-repo
cd gh-pages-repo

if git show-ref --verify --quiet refs/remotes/origin/gh-pages; then
  git checkout gh-pages
else
  git checkout --orphan gh-pages
  git rm -rf .
fi

mkdir -p package

# --- 1. APT Репозиторий ---
echo "=> Генерация APT репозитория..."
mkdir -p package/apt/pool/main package/apt/dists/stable/main/binary-amd64
cp "$WORKSPACE_DIR/bin/RFADLauncherLinux.deb" package/apt/pool/main/
(
  cd package/apt
  apt-ftparchive packages pool/main > dists/stable/main/binary-amd64/Packages
  gzip -k -f dists/stable/main/binary-amd64/Packages
  
  cat <<EOF > apt-release.conf
APT::FTPArchive::Release::Codename "stable";
APT::FTPArchive::Release::Components "main";
APT::FTPArchive::Release::Architectures "amd64";
EOF
  
  apt-ftparchive -c apt-release.conf release dists/stable > dists/stable/Release
  rm apt-release.conf
)

# --- 2. RPM Репозиторий ---
echo "=> Генерация RPM репозитория..."
mkdir -p package/rpm
cp "$WORKSPACE_DIR/bin/RFADLauncherLinux.rpm" package/rpm/
createrepo_c package/rpm/

# --- 3. Pacman Репозиторий ---
echo "=> Генерация Pacman репозитория..."
mkdir -p package/pacman/x86_64
cp "$WORKSPACE_DIR/bin/RFADLauncherLinux.pkg.tar.zst" package/pacman/x86_64/
(
  cd package/pacman/x86_64
  /bin/bash /usr/bin/repo-add rfad.db.tar.gz RFADLauncherLinux.pkg.tar.zst
)

# --- 4. Flatpak OSTree Репозиторий ---
echo "=> Генерация Flatpak репозитория..."
mkdir -p package/repo
if [ ! -f "package/repo/config" ]; then
  ostree init --mode=archive-z2 --repo=package/repo
fi
flatpak build-import-bundle package/repo "$WORKSPACE_DIR/bin/RFADLauncherLinux-x86_64.flatpak"
flatpak build-update-repo package/repo

cat <<EOF > package/repo/rfad-launcher.flatpakrepo
[Flatpak Repo]
Title=RFAD Launcher
Url=https://kraito585.github.io/Rfad-launcher-linux-fork/package/repo/
Homepage=https://github.com/Kraito585/Rfad-launcher-linux-fork
IsSubset=false
EOF

# --- Финализация и отправка на GitHub ---
echo "=> Отправка обновленных репозиториев на GitHub Pages..."
git add package/
git commit -m "Update Repositories for ${CI_COMMIT_TAG}" || echo "Нет изменений для коммита"

export GIT_TERMINAL_PROMPT=0
git push "https://x-access-token:${GH_TOKEN}@github.com/${CI_REPO}.git" gh-pages

echo "=== Публикация успешно завершена! ==="