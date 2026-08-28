<script setup lang="ts">
import { ref, computed, onMounted } from 'vue';
import { EventsOn, BrowserOpenURL } from '~/wailsjs/runtime/runtime';

import config from '~/config';
import type { PatchComponentProps } from '~/components/PatchComponent.vue';

// ===== Состояния для модалки установки =====
const showInstallModal = ref(false);
const installerPath = ref('');
const installPath = ref('');
const isInstalling = ref(false);
const installProgress = ref(0);
const installStatus = ref('');
const needsFirstInstall = ref(false);

// Компоненты (оставляем без изменений)
import DiscordIcon from '~/components/icons/Discord.vue';
import Cog from '~/components/icons/Cog.vue';
import Telegram from '~/components/icons/Telegram.vue';
import Vk from '~/components/icons/Vk.vue';
import Boosty from '~/components/icons/Boosty.vue';
import FolderSmallStroke from '~/components/icons/FolderSmallStroke.vue';
import UpdateConfirmationMessage from '~/components/UpdateConfirmationMessage.vue';
import OpenBook from '~/components/icons/OpenBook.vue';
import MO2 from '~/components/icons/MO2.vue';
import Minus from '~/components/icons/Minus.vue';
import Expand from '~/components/icons/Expand.vue';
import SettingsModal from '~/components/SettingsModal.vue';
import RemoteErrorMessage from '~/components/RemoteErrorMessage.vue';
import NetworkErrorMessage from '~/components/NetworkErrorMessage.vue';

// ===== Модалка установки =====
const selectInstaller = async () => {
  const path = await window.go.main.App.SelectFile();
  if (path) installerPath.value = path;
};

const selectInstallDir = async () => {
  const path = await window.go.main.App.SelectDirectory();
  if (path) installPath.value = path;
};

const startInstall = async () => {
  if (!installerPath.value || !installPath.value) {
    // Можно показать предупреждение через ShowMessageDialog
    await window.go.main.App.ShowMessageDialog('Ошибка', 'Выберите установщик и папку для установки');
    return;
  }
  isInstalling.value = true;
  installProgress.value = 0;
  installStatus.value = 'Начинаем установку...';

  // Подписываемся на прогресс установки
  const uninstallProgress = EventsOn('install-progress', (data: any) => {
    installProgress.value = data.percentage * 100;
    installStatus.value = data.message || 'Установка...';
  });

  try {
    await window.go.main.App.InstallGame(installerPath.value, installPath.value);
    // После успешной установки обновляем состояние
    isPathExist.value = true;
    dirError.value = false;
    showInstallModal.value = false;
    await loadVersions(); // обновим версии
    needsFirstInstall.value = await window.go.main.App.GetFirstInstallStatus();
  } catch (e) {
    console.error('Installation failed', e);
    installStatus.value = 'Ошибка установки';
    await window.go.main.App.ShowMessageDialog('Ошибка', 'Не удалось установить игру');
  } finally {
    isInstalling.value = false;
    uninstallProgress();
  }
};

// ===== Вспомогательная функция загрузки версий =====
const loadVersions = async () => {
  const local = await window.go.main.App.GetLocalVersion();
  localVersion.value = local === 'NoPatch' ? '0.0' : local;
  const remote = await window.go.main.App.GetRemoteVersion();
  remoteVersion.value = remote === 'NoPatch' ? '0.0' : remote;
  updateAvailable.value = remoteVersion.value !== localVersion.value;
};

// ===== Реактивные переменные (без изменений) =====
const firstStart = ref(true);
const localVersion = ref('Загружаем...');
const remoteVersion = ref('Загружаем...');
const launcherVersion = ref('Загружаем...');

const updateStarted = ref(false);
const hideUpdate = ref(false);
const isPathExist = ref(true);

const updateDownloadStarted = ref(false);
const updateDownloadSpeed = ref('0');
const updateDownloadPercentage = ref(0);
const updateDownloaded = ref(false);

const updateUnpackStarted = ref(false);
const updateUnpackPercentage = ref(0);
const updateUnpacked = ref(false);

const updateAvailable = ref(false);
const remoteError = ref(false);
const netError = ref(false);

const additionalProgress = ref(0);
const dirError = ref(false);
const googleDriveDirError = ref(false);
const isGameStarting = ref(false);

const modsScrollableToDown = ref(true);
const modsScrollableToTop = ref(false);

const launcherUpdate = ref(false);
const patches = ref<PatchComponentProps[]>([]);

const fpsOptions = [60, 75, 120, 144, 165];
const selectedFps = ref<number | null>(null);
const initialFps = ref<number | null>(null);
const selectedVoice = ref<'ru' | 'en' | null>(null);
const initialVoice = ref<'ru' | 'en' | null>(null);
const isSettingsOpen = ref(false);
const isSavingSettings = ref(false);

const isSettingsDirty = computed(() => {
  if (selectedFps.value === null || selectedVoice.value === null) return false;
  return selectedFps.value !== initialFps.value || selectedVoice.value !== initialVoice.value;
});

const showConfirmation = ref(false);

const wait = (ms = 1000) => new Promise(resolve => setTimeout(resolve, ms));

// ===== Вспомогательные функции =====
const observeScrollability = (id: string) => {
  const element = document.getElementById(id);
  if (!element) return;
  const observer = new IntersectionObserver(
    (entries) => {
      entries.forEach((entry) => {
        if (entry.target === element.firstElementChild) modsScrollableToTop.value = !entry.isIntersecting;
        if (entry.target === element.lastElementChild) modsScrollableToDown.value = !entry.isIntersecting;
      });
    },
    { root: element, threshold: 0.9 }
  );
  if (element.firstElementChild) observer.observe(element.firstElementChild);
  if (element.lastElementChild) observer.observe(element.lastElementChild);
};

const updatePercentage = computed(() => {
  return (
    +(+updateDownloadPercentage.value.toFixed(0) / 2).toFixed(0) +
    +(+updateUnpackPercentage.value.toFixed(0) / 2.1).toFixed(0) +
    additionalProgress.value
  );
});

// ===== Загрузка настроек (заменяем invoke) =====
const loadSettings = async () => {
  if (!isPathExist.value) return;
  try {
    const fps = await window.go.main.App.GetFramerateLimit();
    selectedFps.value = fps;
    initialFps.value = fps;
  } catch (e) {
    console.error('Failed to load framerate limit', e);
  }
  try {
    const voice = await window.go.main.App.GetVoiceLocale();
    const normalized = voice === 'ru' ? 'ru' : 'en';
    selectedVoice.value = normalized;
    initialVoice.value = normalized;
  } catch (e) {
    console.error('Failed to load voice locale', e);
  }
};

const openSettings = async () => {
  if (!isPathExist.value) {
    dirError.value = true;
    return;
  }
  if (selectedFps.value === null || selectedVoice.value === null) await loadSettings();
  isSettingsOpen.value = true;
};

const closeSettings = () => { isSettingsOpen.value = false; };

const saveSettings = async () => {
  if (!isSettingsDirty.value || selectedFps.value === null || selectedVoice.value === null) return;
  isSavingSettings.value = true;
  try {
    await window.go.main.App.UpdateGameSettings(selectedFps.value, selectedVoice.value);
    initialFps.value = selectedFps.value;
    initialVoice.value = selectedVoice.value;
    isSettingsOpen.value = false;
  } catch (e) {
    console.error('Failed to update settings', e);
  } finally {
    isSavingSettings.value = false;
  }
};

// ===== Основные методы лаунчера =====
const openUrl = async (url: string) => {
  console.log('Клик прошел! Отправляем в Go:', url);
  if (!url) return;
  try {
    await window.go.main.App.OpenBrowser(url);
  } catch (e) {
    console.error('Ошибка вызова Go:', e);
  }
};

const openBrowser = (url: string) => {
  console.log('Попытка открыть ссылку:', url);
  if (!url) return;
  window.go.main.App.BrowserOpenURL(url);
};

const openDiscord = async () => {
  console.log('Открываем Discord:', config.discord);
  window.go.main.App.BrowserOpenURL(config.discord);
};

const openTelegram = async () => {
  console.log('Открываем Telegram:', config.telegram);
  window.go.main.App.BrowserOpenURL(config.telegram);
};

const openVk = async () => {
  console.log('Открываем VK:', config.vk);
  window.go.main.App.BrowserOpenURL(config.vk);
};

const openBoosty = async () => {
  console.log('Открываем Boosty:', config.boosty);
  window.go.main.App.BrowserOpenURL(config.boosty);
};

const openDb = async () => {
  console.log('Открываем Базу Знаний:', config.db);
  window.go.main.App.BrowserOpenURL(config.db);
};

const openExplorer = async () => {
  await window.go.main.App.OpenExplorer();
};

const openMo2 = async () => {
  await window.go.main.App.OpenMO2();
};

const startGame = async () => {
  isGameStarting.value = true;
  await window.go.main.App.StartGame();
  await wait(30000);
  isGameStarting.value = false;
};

const update = async (isFirstStart: boolean = false) => {
  if (!isFirstStart && !showConfirmation.value) {
    showConfirmation.value = true;
    return;
  }
  showConfirmation.value = false;

  updateDownloadPercentage.value = 0;
  updateUnpackPercentage.value = 0;
  additionalProgress.value = 0;
  updateDownloadSpeed.value = '0';
  updateDownloaded.value = false;
  updateUnpacked.value = false;
  updateStarted.value = true;

  // Подписка на события прогресса (Wails Events)
  const unlistenDownload = EventsOn('download-progress', (data: any) => {
    if (data.fileName && data.fileName.includes('update')) {
      updateDownloadSpeed.value = (data.speedBytesPerSec / 1024 / 1024).toFixed(1);
      updateDownloadPercentage.value = data.percentage;
    }
  });

  const unlistenUnpack = EventsOn('unpack-progress', (data: any) => {
    updateUnpackPercentage.value = data.percentage;
  });

  const unlistenUpdateStatus = EventsOn('update-status', (data: any) => {
    if (data.status === 'download-started') updateDownloadStarted.value = true;
    if (data.status === 'download-finished') {
      updateDownloaded.value = true;
      updateDownloadStarted.value = false;
      unlistenDownload();
    }
    if (data.status === 'unpack-started') updateUnpackStarted.value = true;
    if (data.status === 'unpack-finished') {
      updateUnpacked.value = true;
      updateUnpackStarted.value = false;
      unlistenUnpack();
    }
    if (data.status === 'load-order-update-started') additionalProgress.value += 1;
    if (data.status === 'load-order-update-finished') additionalProgress.value += 1;
  });

  await window.go.main.App.Update();

  await wait(300);
  localStorage.setItem('lastUpdate', Date.now().toString());
  firstStart.value = !localStorage.getItem('lastUpdate');
  updateStarted.value = false;

  // Обновляем версии
  const local = await window.go.main.App.GetLocalVersion();
  localVersion.value = local === 'NoPatch' ? '0.0' : local;
  const remote = await window.go.main.App.GetRemoteVersion();
  remoteVersion.value = remote === 'NoPatch' ? '0.0' : remote;
  updateAvailable.value = remoteVersion.value !== localVersion.value;

  unlistenUpdateStatus();
};

const processButtonClick = async () => {
  console.log('processButtonClick called, needsFirstInstall=', needsFirstInstall.value);
  if (!isPathExist.value) {
    showInstallModal.value = true;
    return;
  }
  if (needsFirstInstall.value) {
  isGameStarting.value = true;
  updateStarted.value = true;
  updateDownloadStarted.value = true;
  updateDownloadPercentage.value = 0;
  try {
    await window.go.main.App.FirstInstall();
    needsFirstInstall.value = false;
    await loadVersions();
  } catch (e) {
    console.error('FirstInstall error:', e);
  } finally {
    isGameStarting.value = false;
    updateStarted.value = false;
    updateDownloadStarted.value = false;
  }
  return;
}
  await startGame();
};

const checkUpdates = async () => {
  const req = await fetch(
    `https://raw.githubusercontent.com/Amirust/rfad-launcher/main/lversions.json?t=${Date.now()}`,
    { cache: 'no-store' }
  );
  const { version, downloadUrl } = (await req.json()) as { version: string; downloadUrl: string };
  const currentVersion = await window.go.main.App.GetLauncherVersion();
  launcherVersion.value = currentVersion;
  if (currentVersion !== version) {
    launcherUpdate.value = true;
    await window.go.main.App.UpdateLauncher(downloadUrl);
    await window.go.main.App.StartNewLauncher();
  }
};

// ===== Жизненный цикл =====
onMounted(async () => {
  // Управление окном (Wails)
  document.getElementById('titlebar-minimize')?.addEventListener('click', () => window.runtime.WindowMinimise());
  document.getElementById('titlebar-close')?.addEventListener('click', () => window.runtime.WindowClose());

  firstStart.value = !localStorage.getItem('lastUpdate');

  const exist = await window.go.main.App.IsPathExist();
  isPathExist.value = exist;
  dirError.value = !exist;
  if (exist) {
    await loadSettings();
  } else {
    showInstallModal.value = true;
  }
  EventsOn('install-progress', (data: any) => {
  updateDownloadPercentage.value = data.percentage * 100;
  // если нужно, обновляйте статус
  updateDownloadStarted.value = true;
  if (data.percentage >= 1) {
    updateDownloadStarted.value = false;
  }
});

  const local = await window.go.main.App.GetLocalVersion();
  localVersion.value = local === 'NoPatch' ? '0.0' : local;

  const remote = await window.go.main.App.GetRemoteVersion();
  if (remote === 'NoDir') {
    remoteVersion.value = '0.0';
    dirError.value = true;
  } else if (['NetError', 'AuthError', 'DriveError'].includes(remote)) {
    remoteVersion.value = '0.0';
    updateAvailable.value = false;
    if (isPathExist.value) {
      if (remote === 'NetError') netError.value = true;
      else remoteError.value = true;
    }
  } else {
    remoteVersion.value = remote === 'NoPatch' ? '0.0' : remote;
    if (remoteVersion.value === '0.0') {
      if (isPathExist.value) remoteError.value = true;
      updateAvailable.value = false;
    } else {
      updateAvailable.value = remoteVersion.value !== localVersion.value;
    }
  }

  if (exist) {
    await loadSettings();
      needsFirstInstall.value = await window.go.main.App.GetFirstInstallStatus();
  } else {
    showInstallModal.value = true;
  }

  const patchesJson = await window.go.main.App.LoadPatches();
  patches.value = JSON.parse(patchesJson) as PatchComponentProps[];
  await wait(50);
  observeScrollability('patches');

  await checkUpdates();
});
</script>

<template>
  <!-- Шаблон остаётся без изменений, так как он не зависит от бэкенда -->
  <div class="absolute bottom-0 right-0 opacity-10 hover:opacity-60 transition-opacity z-[100000]">
    <span class="text-primary font-semibold tracking-wide">{{ launcherVersion }}</span>
  </div>
  
  <Transition name="fade-modal" appear>
  <div v-if="showInstallModal" class="fixed inset-0 flex items-center justify-center bg-black bg-opacity-50 z-[100001]">
    <div class="bg-gray-900 p-6 rounded-lg shadow-lg max-w-md w-full border border-gray-700">
      <h2 class="text-2xl font-bold text-primary mb-4">Установка игры</h2>
      <div v-if="!isInstalling">
        <div class="mb-4">
          <label class="block text-secondary mb-1">Путь к установщику (.exe/.msi):</label>
          <div class="flex">
            <input v-model="installerPath" class="flex-1 bg-gray-800 text-primary p-2 rounded-l border border-gray-700" placeholder="Выберите файл..." readonly />
            <button @click="selectInstaller" class="bg-primary text-gray-900 px-4 py-2 rounded-r hover:bg-opacity-80 transition font-bold">Обзор</button>
          </div>
        </div>
        <div class="mb-4">
          <label class="block text-secondary mb-1">Папка для установки:</label>
          <div class="flex">
            <input v-model="installPath" class="flex-1 bg-gray-800 text-primary p-2 rounded-l border border-gray-700" placeholder="Выберите папку..." readonly />
            <button @click="selectInstallDir" class="bg-primary text-gray-900 px-4 py-2 rounded-r hover:bg-opacity-80 transition font-bold">Обзор</button>
          </div>
        </div>
        <button @click="startInstall" class="w-full bg-primary text-gray-900 font-bold py-2 rounded hover:bg-opacity-80 transition">Установить</button>
        <button @click="showInstallModal = false" class="mt-3 w-full text-secondary hover:text-primary transition">Отмена</button>
      </div>
      <div v-else>
        <div class="mb-2 text-secondary">{{ installStatus }}</div>
        <div class="w-full bg-gray-700 rounded-full h-2.5">
          <div class="bg-primary h-2.5 rounded-full transition-all duration-300" :style="{ width: installProgress + '%' }"></div>
        </div>
        <div class="mt-3 text-sm text-secondary text-center">{{ Math.round(installProgress) }}%</div>
      </div>
    </div>
  </div>
</Transition>
  <div data-tauri-drag-region class="titlebar z-[100000]">
    <div class="titlebar-button" id="titlebar-minimize">
      <Minus class="text-primary w-5" />
    </div>
    <div class="titlebar-button opacity-70 pointer-events-none cursor-not-allowed" id="titlebar-maximize">
      <Expand class="text-primary w-4" />
    </div>
    <div class="titlebar-button" id="titlebar-close">
      <IconsX class="text-primary w-5" />
    </div>
  </div>
  <div class="px-10 py-10 flex flex-row w-full h-full min-h-svh relative overflow-hidden">
    <div class="flex flex-row gap-6 z-40 w-full">
      <div class="flex flex-col justify-between min-h-full relative z-[100005]">
        
        <CircleButton @click.left.prevent="openDiscord()">
          <DiscordIcon class="w-9 text-secondary pointer-events-none" />
        </CircleButton>
        
        <CircleButton @click.left.prevent="openTelegram()">
          <Telegram class="w-7 mr-1 mt-[2px] text-secondary pointer-events-none" />
        </CircleButton>
        
        <CircleButton @click.left.prevent="openVk()">
          <Vk class="w-8 text-secondary pointer-events-none" />
        </CircleButton>
        
        <CircleButton @click.left.prevent="openBoosty()">
          <Boosty class="w-8 mb-[2px] ml-[2px] text-secondary pointer-events-none" />
        </CircleButton>
        
        <CircleButton @click.left.prevent="openDb()">
          <OpenBook class="w-8 text-secondary pointer-events-none" />
        </CircleButton>

        <CircleButton @click="openExplorer">
          <FolderSmallStroke class="w-8 h-8 text-secondary" />
        </CircleButton>

        <CircleButton @click="openMo2">
          <MO2 class="w-10 ml-[1px] text-secondary" />
        </CircleButton>
        
      </div>
      <div class="horizontal-divider"></div>
      <div class="flex flex-col justify-between h-full">
        <h1 class="text-5xl text-gradient font-semibold">RFAD SE 6.2</h1>
        <div class="flex flex-col gap-4 relative">
          <transition-group name="fade" tag="div" class="relative flex flex-col gap-4">
            <UpdateConfirmationMessage v-if="showConfirmation" class="w-full">
              <div class="flex flex-row justify-between w-full mt-2.5">
                <div class="font-bold hover:opacity-80 transition-opacity cursor-pointer" @click="update()">
                  Продолжить
                </div>
                <div class="font-bold text-secondary hover:opacity-80 transition-opacity cursor-pointer" @click="showConfirmation = false">
                  Отменить
                </div>
              </div>
            </UpdateConfirmationMessage>
            <DirErrorMessage v-if="dirError" class="w-full" />
            <DirErrorMessage v-if="googleDriveDirError" class="w-full" />
            <RemoteErrorMessage v-if="remoteError && !updateStarted" class="w-full" />
            <NetworkErrorMessage v-if="netError && !updateStarted" class="w-full" />
            <UpdatingMessage :percentage="updatePercentage" v-if="updateStarted" class="w-full" />
            <UnpackingMessage :percentage="updateUnpackPercentage" v-if="updateUnpackStarted" class="w-full" />
            <DownloadingMessage :speed="updateDownloadSpeed" :percentage="updateDownloadPercentage" v-if="updateDownloadStarted" class="w-full" />
            <UpdateAvailableMessage :version="remoteVersion" v-if="updateAvailable && !updateStarted && !hideUpdate" class="w-full">
              <div class="flex flex-row justify-between w-full mt-2.5">
                <div class="font-bold hover:opacity-80 transition-opacity cursor-pointer" @click="update()">
                  Обновить
                </div>
                <div class="font-bold text-secondary hover:opacity-80 transition-opacity cursor-pointer" @click="hideUpdate = true">
                  Скрыть
                </div>
              </div>
            </UpdateAvailableMessage>
            <LauncherUpdatingMessage class="w-full" v-if="launcherUpdate" />
          </transition-group>
          <div class="flex flex-row gap-2.5">
            <Button
              @click="processButtonClick"
              class="font-bold text-4xl text-primary tracking-wider uppercase min-w-73"
              :class="{
                'cursor-pointer': !isGameStarting && !updateStarted,
                'cursor-not-allowed text-secondary pointer-events-none': isGameStarting || updateStarted,
              }"
            >
              {{ needsFirstInstall ? 'Обновить' : (firstStart && !hideUpdate ? 'Обновить' : 'Играть') }}
            </Button>
            <DropdownButton
              :same-padding="true"
              :hide-update="hideUpdate"
              class="font-bold text-4xl text-primary"
              @update="update(false)"
              @open-mo2="openMo2"
              @open-explorer="openExplorer"
              @open-settings="openSettings"
              @start_game="startGame"
            >
              <Cog class="w-11 text-primary" />
            </DropdownButton>
          </div>
          <div class="flex flex-col w-full">
            <div class="flex flex-row w-full">
              <span class="text-secondary font-medium w-24 mr-2 tracking-wide">Установлена:</span>
              <span class="text-primary font-semibold tracking-wide">{{ localVersion }}</span>
            </div>
            <div class="flex flex-row w-full">
              <span class="text-secondary font-medium w-24 mr-2 tracking-wide">Актуальная:</span>
              <span class="text-primary font-semibold tracking-wide">{{ remoteVersion }}</span>
            </div>
          </div>
        </div>
      </div>
      <div class="w-full flex flex-row justify-end">
        <div
          id="patches"
          :class="{
            'fade-bought': modsScrollableToTop && modsScrollableToDown,
            'fade-top': modsScrollableToTop && !modsScrollableToDown,
            'fade-down': modsScrollableToDown && !modsScrollableToTop,
          }"
          class="flex flex-col gap-4 text-primary max-h-[88vh] overflow-auto scrollbar-hide"
        >
          <transition-group name="fade">
            <PatchComponent
              v-for="patch in patches"
              :key="patch.version"
              :version="patch.version"
              :date="patch.date"
              :author="patch.author"
              :name="patch.name"
              :description="patch.description"
              :url="patch.url"
            />
          </transition-group>
        </div>
      </div>
    </div>
    <img alt="Matrona" src="assets/image/Matrona.webp" class="matrona z-10" />
    <Transition name="fade-modal">
      <SettingsModal v-if="isSettingsOpen" @close="closeSettings" />
    </Transition>
  </div>
</template>

<style lang="scss">
@use 'assets/css/global' as *;

.horizontal-divider {
  background-image: radial-gradient(circle, theme('colors.secondaryDarker'), #000000);
  width: 1.5px;
  @apply h-full;
}

.text-gradient {
  background:
    linear-gradient(120deg, rgba(13, 12, 10, 0) 30%, #0d0c0a 100%),
    linear-gradient(#ffeabf, #ffeabf);
  -webkit-background-clip: text;
  -webkit-text-fill-color: transparent;
  text-shadow: 0 0 2px rgba(255, 234, 191, 0.5);
}

.matrona {
  opacity: 50%;
  position: absolute;
  bottom: 0;
  left: 120px;
  width: 620px;
  height: 620px;
}

.fade-down {
  @include mask-image(0deg, 0rem, 3rem);
}
.fade-top {
  @include mask-image(180deg, 0rem, 3rem);
}
.fade-bought {
  mask-composite: intersect;
  mask-image:
    linear-gradient(0deg, transparent 0%, transparent 0rem, black 3rem),
    linear-gradient(180deg, transparent 0%, transparent 0rem, black 3rem);
}

.fade-modal-enter-active,
.fade-modal-leave-active {
  transition: opacity 0.2s ease;
}
.fade-modal-enter-from,
.fade-modal-leave-to {
  opacity: 0;
}
.fade-modal-leave-from,
.fade-modal-enter-to {
  opacity: 1;
}
</style>