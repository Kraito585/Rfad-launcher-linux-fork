<script setup lang="ts">
import { ref, computed, onMounted } from "vue";
import { Events } from "@wailsio/runtime";
import * as App from "~/bindings/rfad-launcher-linux/app";

import config from "~/config";
import type { PatchComponentProps } from "~/components/PatchComponent.vue";
import GamescopeErrorMessage from "~/components/GamescopeErrorMessage.vue";

// ===== Состояния для модалки установки =====
const showInstallModal = ref(false);
const installStep = ref(0);
const installerPath = ref("");
const installPath = ref("");
const mo2Path = ref("");
const portMo2 = ref(false);
const isInstalling = ref(false);
const installProgress = ref(0);
const installStatus = ref("");
const needsFirstInstall = ref(false);

// Компоненты
import DiscordIcon from "~/components/icons/Discord.vue";
import Cog from "~/components/icons/Cog.vue";
import Telegram from "~/components/icons/Telegram.vue";
import Vk from "~/components/icons/Vk.vue";
import Boosty from "~/components/icons/Boosty.vue";
import FolderSmallStroke from "~/components/icons/FolderSmallStroke.vue";
import UpdateConfirmationMessage from "~/components/UpdateConfirmationMessage.vue";
import OpenBook from "~/components/icons/OpenBook.vue";
import MO2 from "~/components/icons/MO2.vue";
import Minus from "~/components/icons/Minus.vue";
import Expand from "~/components/icons/Expand.vue";
import SettingsModal from "~/components/SettingsModal.vue";
import RemoteErrorMessage from "~/components/RemoteErrorMessage.vue";
import NetworkErrorMessage from "~/components/NetworkErrorMessage.vue";
import ProtonTricksErrorMessage from "~/components/ProtonTricksErrorMessage.vue";

// ===== Модалка установки =====
const selectMo2Path = async () => {
  // Выбираем EXE файл
  const path = await App.SelectFile();
  if (path) mo2Path.value = path;
};

const selectInstaller = async () => {
  const path = await App.SelectFile();
  if (path) installerPath.value = path;
};

const selectInstallDir = async () => {
  const path = await App.SelectDirectory();
  if (path) installPath.value = path;
};

const acceptExistingPath = async () => {
  if (!mo2Path.value) {
    await App.ShowMessageDialog("Ошибка", "Укажите путь к modorganizer.exe");
    return;
  }

  try {
    await App.SaveGlobalPath(mo2Path.value);

    showInstallModal.value = false;
    installStep.value = 0;

    // Заново проверяем статус файлов после сохранения

    await loadVersions();
  } catch (e) {
    console.error("Ошибка при сохранении пути:", e);
    await App.ShowMessageDialog("Ошибка", "Не удалось сохранить путь к игре");
  }
};

// ===== Ошибки среды =====
const gamescopeError = ref(false);
const protonTricksError = ref(false);

const startInstall = async () => {
  if (!installerPath.value || !installPath.value) {
    await App.ShowMessageDialog(
      "Ошибка",
      "Выберите установщик и папку для установки",
    );
    return;
  }
  isInstalling.value = true;
  installProgress.value = 0;
  installStatus.value = "Начинаем установку...";

  // Локальная подписка на прогресс
  const uninstallProgress = Events.On("install-progress", (event: any) => {
    installProgress.value =
      event.data[0]?.percentage * 100 || event.data.percentage * 100;
    installStatus.value =
      event.data[0]?.message || event.data.message || "Установка...";
  });

  try {
    const mo2ToPort = portMo2.value ? mo2Path.value : "";
    await App.InstallGame(installerPath.value, installPath.value, mo2ToPort);

    showInstallModal.value = false;
    installStep.value = 0;

    await checkPathStatus();
    await loadVersions();
  } catch (e) {
    console.error("Installation failed", e);
    installStatus.value = "Ошибка установки";
    await App.ShowMessageDialog("Ошибка", "Не удалось установить игру");
  } finally {
    isInstalling.value = false;
    uninstallProgress();
  }
};

const openProtontricks = async () => {
  try {
    await App.OpenProtonTrics();
  } catch (e) {
    console.error("Ошибка вызова Protontricks:", e);
  }
};

// ===== Вспомогательная функция загрузки версий =====
const loadVersions = async () => {
  const local = await App.GetLocalVersion();
  localVersion.value = local === "NoPatch" ? "0.0" : local;
  const remote = await App.GetRemoteVersion();
  remoteVersion.value = remote === "NoPatch" ? "0.0" : remote;
  updateAvailable.value = remoteVersion.value !== localVersion.value;
};

// ===== Реактивные переменные =====
const firstStart = ref(true);
const localVersion = ref("Загружаем...");
const remoteVersion = ref("Загружаем...");
const launcherVersion = ref("Загружаем...");

const updateStarted = ref(false);
const hideUpdate = ref(false);
const isPathExist = ref(true);

const updateDownloadStarted = ref(false);
const updateDownloadSpeed = ref("0");
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

const patches = ref<PatchComponentProps[]>([]);

const isSettingsOpen = ref(false);
const showRecoveryModal = ref(false);

const showConfirmation = ref(false);

const wait = (ms = 1000) => new Promise((resolve) => setTimeout(resolve, ms));

// ===== Вспомогательные функции =====
const observeScrollability = (id: string) => {
  const element = document.getElementById(id);
  if (!element) return;
  const observer = new IntersectionObserver(
    (entries) => {
      entries.forEach((entry) => {
        if (entry.target === element.firstElementChild)
          modsScrollableToTop.value = !entry.isIntersecting;
        if (entry.target === element.lastElementChild)
          modsScrollableToDown.value = !entry.isIntersecting;
      });
    },
    { root: element, threshold: 0.9 },
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

const goToInstall = (port: boolean) => {
  portMo2.value = port;
  installStep.value = 3;
};

// ===== Загрузка настроек =====
const openSettings = () => {
  if (!isPathExist.value) {
    dirError.value = true;
    return;
  }

  isSettingsOpen.value = true;
};

const closeSettings = () => {
  isSettingsOpen.value = false;
};

// ===== Основные методы лаунчера =====
const openDiscord = async () => App.BrowserOpenURL(config.discord);
const openTelegram = async () => App.BrowserOpenURL(config.telegram);
const openVk = async () => App.BrowserOpenURL(config.vk);
const openBoosty = async () => App.BrowserOpenURL(config.boosty);
const openDb = async () => App.BrowserOpenURL(config.db);
const openExplorer = async () => await App.OpenExplorer();

const openMo2 = async () => {
  isGameStarting.value = true;
  try {
    await App.OpenMO2();
  } catch (e) {
    console.error("Ошибка вызова OpenMO2:", e);
    isGameStarting.value = false;
  }
};

const startGame = async () => {
  isGameStarting.value = true;
  try {
    await App.StartGame();
  } catch (e) {
    console.error("Ошибка вызова StartGame:", e);
    isGameStarting.value = false;
  }
};

const minimizeWindow = () => {
  try {
    App.Minimize();
  } catch (e) {
    console.error("Ошибка при сворачивании окна:", e);
  }
};

const closeWindow = () => {
  try {
    App.Quit();
  } catch (e) {
    console.error("Ошибка при закрытии окна:", e);
  }
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
  updateDownloadSpeed.value = "0";
  updateDownloaded.value = false;
  updateUnpacked.value = false;

  updateStarted.value = false;
  updateDownloadStarted.value = true;
  updateUnpackStarted.value = false;

  const unlistenDownload = Events.On("download-progress", (event: any) => {
    // Безопасное извлечение данных (иногда Wails оборачивает их в массив)
    const payload = event.data[0] || event.data;
    if (typeof payload.speedBytesPerSec === "number") {
      updateDownloadSpeed.value = (
        payload.speedBytesPerSec /
        1024 /
        1024
      ).toFixed(1);
    } else {
      updateDownloadSpeed.value = payload.speedBytesPerSec;
    }
    updateDownloadPercentage.value = payload.percentage;
  });

  const unlistenUnpack = Events.On("unpack-progress", (event: any) => {
    const payload = event.data[0] || event.data;
    updateUnpackPercentage.value = payload.percentage;
  });

  const unlistenUpdateStatus = Events.On("update-status", (event: any) => {
    const payload = event.data[0] || event.data;
    if (payload.status === "download-started") {
      updateDownloadStarted.value = true;
      updateUnpackStarted.value = false;
    }
    if (payload.status === "download-finished") {
      updateDownloaded.value = true;
      updateDownloadStarted.value = false;
    }
    if (payload.status === "unpack-started") {
      updateUnpackStarted.value = true;
      updateDownloadStarted.value = false;
    }
    if (payload.status === "unpack-finished") {
      updateUnpacked.value = true;
      updateUnpackStarted.value = false;
    }
    if (payload.status === "load-order-update-started")
      additionalProgress.value += 1;
    if (payload.status === "load-order-update-finished")
      additionalProgress.value += 1;
  });

  try {
    await App.Update();
  } catch (e) {
    console.error("Ошибка при обновлении:", e);
  }

  await wait(300);
  localStorage.setItem("lastUpdate", Date.now().toString());
  firstStart.value = !localStorage.getItem("lastUpdate");

  updateStarted.value = false;
  updateDownloadStarted.value = false;
  updateUnpackStarted.value = false;

  unlistenDownload();
  unlistenUnpack();
  unlistenUpdateStatus();

  await loadVersions();
};

const startFirstInstallFlow = async () => {
  isGameStarting.value = true;
  updateStarted.value = false;
  updateDownloadStarted.value = true;
  updateUnpackStarted.value = false;
  updateDownloadPercentage.value = 0;
  updateDownloadSpeed.value = "0";

  const unlistenDownload = Events.On("download-progress", (event: any) => {
    const payload = event.data[0] || event.data;
    if (typeof payload.speedBytesPerSec === "number") {
      updateDownloadSpeed.value = (
        payload.speedBytesPerSec /
        1024 /
        1024
      ).toFixed(1);
    } else {
      updateDownloadSpeed.value = payload.speedBytesPerSec;
    }
    updateDownloadPercentage.value = payload.percentage;
  });

  const unlistenUnpack = Events.On("unpack-progress", (event: any) => {
    const payload = event.data[0] || event.data;
    updateUnpackPercentage.value = payload.percentage;
  });

  const unlistenUpdateStatus = Events.On("update-status", (event: any) => {
    const payload = event.data[0] || event.data;
    if (payload.status === "download-started") {
      updateDownloadStarted.value = true;
      updateUnpackStarted.value = false;
    }
    if (payload.status === "download-finished") {
      updateDownloadStarted.value = false;
    }
    if (payload.status === "unpack-started") {
      updateUnpackStarted.value = true;
      updateDownloadStarted.value = false;
    }
    if (payload.status === "unpack-finished") {
      updateUnpackStarted.value = false;
    }
  });

  try {
    await App.FirstInstall();
    needsFirstInstall.value = false;
    await loadVersions();
  } catch (e) {
    console.error("FirstInstall error:", e);
  } finally {
    isGameStarting.value = false;
    updateDownloadStarted.value = false;
    updateUnpackStarted.value = false;

    unlistenDownload();
    unlistenUnpack();
    unlistenUpdateStatus();
  }
};

const processButtonClick = async () => {
  if (!isPathExist.value) {
    showInstallModal.value = true;
    return;
  }

  if (needsFirstInstall.value) {
    await startFirstInstallFlow();
    return;
  }

  if (firstStart.value && !hideUpdate.value && updateAvailable.value) {
    await update(true);
    return;
  }

  // Приоритет 3: Запуск игры
  await startGame();
};

const checkPathStatus = async () => {
  const exist = await App.IsPathExist();
  isPathExist.value = exist;
  dirError.value = !exist;

  if (exist) {
    needsFirstInstall.value = await App.GetFirstInstallStatus();
  }
};

// ===== Жизненный цикл =====
onMounted(async () => {
  firstStart.value = !localStorage.getItem("lastUpdate");

  await checkPathStatus();

  if (!isPathExist.value) {
    showInstallModal.value = true;
  }

  const local = await App.GetLocalVersion();
  localVersion.value = local === "NoPatch" ? "0.0" : local;

  const remote = await App.GetRemoteVersion();
  if (remote === "NoDir") {
    remoteVersion.value = "0.0";
    dirError.value = true;
  } else if (["NetError", "AuthError", "DriveError"].includes(remote)) {
    remoteVersion.value = "0.0";
    updateAvailable.value = false;
    if (isPathExist.value) {
      if (remote === "NetError") netError.value = true;
      else remoteError.value = true;
    }
  } else {
    remoteVersion.value = remote === "NoPatch" ? "0.0" : remote;
    if (remoteVersion.value === "0.0") {
      if (isPathExist.value) remoteError.value = true;
      updateAvailable.value = false;
    } else {
      updateAvailable.value = remoteVersion.value !== localVersion.value;
    }
  }

  const patchesJson = await App.LoadPatches();
  patches.value = JSON.parse(patchesJson) as PatchComponentProps[];
  await wait(50);
  observeScrollability("patches");

  // ОСТАВЛЯЕМ ТОЛЬКО ТЕ СЛУШАТЕЛИ, КОТОРЫХ НЕТ В ЛОКАЛЬНЫХ ФУНКЦИЯХ
  Events.On("game-exit", () => {
    isGameStarting.value = false;
  });

  Events.On("gamescope-missing", () => {
    gamescopeError.value = true;
  });

  // ИСПРАВЛЕНИЕ: Получаем данные из объекта события WailsEvent
  Events.On("game-error", (event: any) => {
    // В зависимости от того, как бэкенд шлет данные, они могут быть в event.data или event.data[0]
    const errPayload = event.data[0] || event.data;
    console.error("Процесс завершился с ошибкой:", errPayload);
    isGameStarting.value = false;
  });
});
</script>

<template>
  <div
    class="absolute bottom-0 right-0 opacity-10 hover:opacity-60 transition-opacity z-[100000]"
  >
    <span class="text-primary font-semibold tracking-wide">{{
      launcherVersion
    }}</span>
  </div>

  <Transition name="fade-modal" appear>
    <div
      v-if="showInstallModal"
      class="fixed inset-0 flex items-center justify-center bg-black bg-opacity-50 z-[100001]"
    >
      <div
        class="bg-gray-900 p-6 rounded-lg shadow-lg max-w-md w-full border border-gray-700"
      >
        <!-- ШАГ 0: Выбор действия (Не удалось найти игру) -->
        <div v-if="installStep === 0">
          <h2 class="text-2xl font-bold text-primary mb-4">
            Не удалось найти игру
          </h2>
          <p class="text-secondary mb-6 text-sm leading-relaxed">
            Если игра установлена не через предыдущую версию лаунчера
            рекомендуется установка
          </p>
          <div class="flex gap-4">
            <button
              @click="installStep = 1"
              class="w-full bg-gray-700 text-primary font-bold py-2 rounded hover:bg-opacity-80 transition"
            >
              Указать путь
            </button>
            <button
              @click="installStep = 2"
              class="w-full bg-primary text-gray-900 font-bold py-2 rounded hover:bg-opacity-80 transition"
            >
              Установить игру
            </button>
          </div>
        </div>

        <!-- ШАГ 1: Указать путь -->
        <div v-else-if="installStep === 1">
          <h2 class="text-2xl font-bold text-primary mb-4">Указать путь</h2>
          <div class="mb-6">
            <label class="block text-secondary mb-1"
              >Путь к modorganizer.exe:</label
            >
            <div class="flex">
              <input
                v-model="mo2Path"
                class="flex-1 bg-gray-800 text-primary p-2 rounded-l border border-gray-700"
                placeholder="Выберите файл..."
                readonly
              />
              <button
                @click="selectMo2Path"
                class="bg-primary text-gray-900 px-4 py-2 rounded-r hover:bg-opacity-80 transition font-bold"
              >
                Обзор
              </button>
            </div>
          </div>
          <button
            @click="acceptExistingPath"
            class="w-full bg-primary text-gray-900 font-bold py-2 rounded hover:bg-opacity-80 transition"
          >
            Принять
          </button>
          <button
            @click="installStep = 0"
            class="mt-3 w-full text-secondary hover:text-primary transition"
          >
            Назад
          </button>
        </div>

        <!-- ШАГ 2: Портировать MO2? -->
        <div v-else-if="installStep === 2">
          <div class="flex items-start gap-2 mb-4">
            <h2 class="text-2xl font-bold text-primary leading-tight">
              Портировать MO2 из установленой игры
            </h2>
            <!-- Иконка вопроса с подсказкой (Tooltip) -->
            <div
              class="relative flex items-center group cursor-help text-secondary hover:text-primary mt-1"
            >
              <svg
                xmlns="http://www.w3.org/2000/svg"
                class="h-6 w-6"
                fill="none"
                viewBox="0 0 24 24"
                stroke="currentColor"
              >
                <path
                  stroke-linecap="round"
                  stroke-linejoin="round"
                  stroke-width="2"
                  d="M8.228 9c.549-1.165 2.03-2 3.772-2 2.21 0 4 1.343 4 3 0 1.4-1.278 2.575-3.006 2.907-.542.104-.994.54-.994 1.093m0 4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z"
                />
              </svg>
              <div
                class="absolute bottom-full left-1/2 transform -translate-x-1/2 mb-2 hidden group-hover:block bg-gray-800 text-sm text-white px-3 py-1.5 rounded shadow-lg whitespace-nowrap z-50"
              >
                Перенос модов и сохранений
              </div>
            </div>
          </div>
          <div class="flex gap-4 mt-6">
            <button
              @click="goToInstall(true)"
              class="w-full bg-primary text-gray-900 font-bold py-2 rounded hover:bg-opacity-80 transition"
            >
              Да
            </button>
            <button
              @click="goToInstall(false)"
              class="w-full bg-gray-700 text-primary font-bold py-2 rounded hover:bg-opacity-80 transition"
            >
              Нет
            </button>
          </div>
          <button
            @click="installStep = 0"
            class="mt-4 w-full text-secondary hover:text-primary transition"
          >
            Назад
          </button>
        </div>

        <!-- ШАГ 3: Сама установка -->
        <div v-else-if="installStep === 3">
          <h2 class="text-2xl font-bold text-primary mb-4">Установка игры</h2>
          <div v-if="!isInstalling">
            <!-- Дополнительный инпут, если выбрали "Да" на портирование -->
            <div v-if="portMo2" class="mb-4">
              <label class="block text-secondary mb-1"
                >Путь к старому modorganizer.exe:</label
              >
              <div class="flex">
                <input
                  v-model="mo2Path"
                  class="flex-1 bg-gray-800 text-primary p-2 rounded-l border border-gray-700"
                  placeholder="Выберите файл..."
                  readonly
                />
                <button
                  @click="selectMo2Path"
                  class="bg-primary text-gray-900 px-4 py-2 rounded-r hover:bg-opacity-80 transition font-bold"
                >
                  Обзор
                </button>
              </div>
            </div>

            <div class="mb-4">
              <label class="block text-secondary mb-1"
                >Путь к установщику (.exe/.msi):</label
              >
              <div class="flex">
                <input
                  v-model="installerPath"
                  class="flex-1 bg-gray-800 text-primary p-2 rounded-l border border-gray-700"
                  placeholder="Выберите файл..."
                  readonly
                />
                <button
                  @click="selectInstaller"
                  class="bg-primary text-gray-900 px-4 py-2 rounded-r hover:bg-opacity-80 transition font-bold"
                >
                  Обзор
                </button>
              </div>
            </div>

            <div class="mb-4">
              <label class="block text-secondary mb-1"
                >Папка для установки:</label
              >
              <div class="flex">
                <input
                  v-model="installPath"
                  class="flex-1 bg-gray-800 text-primary p-2 rounded-l border border-gray-700"
                  placeholder="Выберите папку..."
                  readonly
                />
                <button
                  @click="selectInstallDir"
                  class="bg-primary text-gray-900 px-4 py-2 rounded-r hover:bg-opacity-80 transition font-bold"
                >
                  Обзор
                </button>
              </div>
            </div>

            <button
              @click="startInstall"
              class="w-full bg-primary text-gray-900 font-bold py-2 rounded hover:bg-opacity-80 transition"
            >
              Установить
            </button>
            <button
              @click="installStep = 0"
              class="mt-3 w-full text-secondary hover:text-primary transition"
            >
              В начало
            </button>
          </div>

          <!-- Прогресс бар установки -->
          <div v-else>
            <div class="mb-2 text-secondary">{{ installStatus }}</div>
            <div class="w-full bg-gray-700 rounded-full h-2.5">
              <div
                class="bg-primary h-2.5 rounded-full transition-all duration-300"
                :style="{ width: installProgress + '%' }"
              ></div>
            </div>
            <div class="mt-3 text-sm text-secondary text-center">
              {{ Math.round(installProgress) }}%
            </div>
          </div>
        </div>
      </div>
    </div>
  </Transition>
  <div style="--wails-draggable: drag" class="titlebar z-[100000]">
    <div
      style="--wails-draggable: no-drag"
      class="titlebar-button"
      @click="minimizeWindow"
    >
      <Minus class="text-primary w-5 pointer-events-none" />
    </div>
    <div
      style="--wails-draggable: no-drag"
      class="titlebar-button opacity-70 cursor-not-allowed"
    >
      <Expand class="text-primary w-4 pointer-events-none" />
    </div>
    <div
      style="--wails-draggable: no-drag"
      class="titlebar-button"
      @click="closeWindow"
    >
      <IconsX class="text-primary w-5 pointer-events-none" />
    </div>
  </div>
  <div
    class="px-10 py-10 flex flex-row w-full h-full min-h-svh relative overflow-hidden"
  >
    <div class="flex flex-row gap-6 z-40 w-full">
      <div class="flex flex-col justify-between min-h-full relative z-[100005]">
        <CircleButton @click.left.prevent="openDiscord()">
          <DiscordIcon class="w-9 text-secondary pointer-events-none" />
        </CircleButton>

        <CircleButton @click.left.prevent="openTelegram()">
          <Telegram
            class="w-7 mr-1 mt-[2px] text-secondary pointer-events-none"
          />
        </CircleButton>

        <CircleButton @click.left.prevent="openVk()">
          <Vk class="w-8 text-secondary pointer-events-none" />
        </CircleButton>

        <CircleButton @click.left.prevent="openBoosty()">
          <Boosty
            class="w-8 mb-[2px] ml-[2px] text-secondary pointer-events-none"
          />
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
          <transition-group
            name="fade"
            tag="div"
            class="relative flex flex-col gap-4"
          >
            <UpdateConfirmationMessage v-if="showConfirmation" class="w-full">
              <div class="flex flex-row justify-between w-full mt-2.5">
                <div
                  class="font-bold hover:opacity-80 transition-opacity cursor-pointer"
                  @click="update()"
                >
                  Продолжить
                </div>
                <div
                  class="font-bold text-secondary hover:opacity-80 transition-opacity cursor-pointer"
                  @click="showConfirmation = false"
                >
                  Отменить
                </div>
              </div>
            </UpdateConfirmationMessage>
            <DirErrorMessage v-if="dirError" class="w-full" />
            <DirErrorMessage v-if="googleDriveDirError" class="w-full" />
            <RemoteErrorMessage
              v-if="remoteError && !updateStarted"
              class="w-full"
            />
            <NetworkErrorMessage
              v-if="netError && !updateStarted"
              class="w-full"
            />
            <UpdatingMessage
              :percentage="updatePercentage"
              v-if="updateStarted"
              class="w-full"
            />
            <UnpackingMessage
              :percentage="updateUnpackPercentage"
              v-if="updateUnpackStarted"
              class="w-full"
            />
            <DownloadingMessage
              :speed="updateDownloadSpeed"
              :percentage="updateDownloadPercentage"
              v-if="updateDownloadStarted"
              class="w-full"
            />
            <ProtonTricksErrorMessage v-if="protonTricksError" class="w-full">
              <div class="flex flex-row justify-end w-full mt-2.5">
                <div
                  class="font-bold text-secondary hover:opacity-80 transition-opacity cursor-pointer"
                  @click="protonTricksError = false"
                >
                  Скрыть
                </div>
              </div>
            </ProtonTricksErrorMessage>
            <GamescopeErrorMessage v-if="gamescopeError" class="w-full">
              <div class="flex flex-row justify-end w-full mt-2.5">
                <div
                  class="font-bold text-secondary hover:opacity-80 transition-opacity cursor-pointer"
                  @click="gamescopeError = false"
                >
                  Скрыть
                </div>
              </div>
            </GamescopeErrorMessage>
            <UpdateAvailableMessage
              :version="remoteVersion"
              v-if="updateAvailable && !updateStarted && !hideUpdate"
              class="w-full"
            >
              <div class="flex flex-row justify-between w-full mt-2.5">
                <div
                  class="font-bold hover:opacity-80 transition-opacity cursor-pointer"
                  @click="update()"
                >
                  Обновить
                </div>
                <div
                  class="font-bold text-secondary hover:opacity-80 transition-opacity cursor-pointer"
                  @click="hideUpdate = true"
                >
                  Скрыть
                </div>
              </div>
            </UpdateAvailableMessage>
          </transition-group>
          <div class="flex flex-row gap-2.5">
            <Button
              @click="processButtonClick"
              class="font-bold text-4xl text-primary tracking-wider uppercase min-w-73"
              :class="{
                'cursor-pointer':
                  !isGameStarting &&
                  !updateStarted &&
                  !updateDownloadStarted &&
                  !updateUnpackStarted,
                'cursor-not-allowed text-secondary pointer-events-none':
                  isGameStarting ||
                  updateStarted ||
                  updateDownloadStarted ||
                  updateUnpackStarted,
              }"
            >
              {{
                isGameStarting
                  ? "Запущено"
                  : needsFirstInstall
                    ? "Обновить"
                    : firstStart && !hideUpdate && updateAvailable
                      ? "Обновить"
                      : "Играть"
              }}
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
              @open-recovery="showRecoveryModal = true"
              @open-protontricks="openProtontricks"
            >
              <Cog class="w-11 text-primary" />
            </DropdownButton>
          </div>
          <div class="flex flex-col w-full">
            <div class="flex flex-row w-full">
              <span class="text-secondary font-medium w-24 mr-2 tracking-wide"
                >Установлена:</span
              >
              <span class="text-primary font-semibold tracking-wide">{{
                localVersion
              }}</span>
            </div>
            <div class="flex flex-row w-full">
              <span class="text-secondary font-medium w-24 mr-2 tracking-wide"
                >Актуальная:</span
              >
              <span class="text-primary font-semibold tracking-wide">{{
                remoteVersion
              }}</span>
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
    <Transition name="fade-modal">
      <RecoveryModal
        v-if="showRecoveryModal"
        @close="showRecoveryModal = false"
      />
    </Transition>
  </div>
</template>

<style lang="scss">
@use "assets/css/global" as *;

.horizontal-divider {
  background-image: radial-gradient(
    circle,
    theme("colors.secondaryDarker"),
    #000000
  );
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
