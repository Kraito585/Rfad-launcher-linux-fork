<script setup lang="ts">
import { ref, onMounted, onUnmounted, watch } from 'vue';
import MessageBox from '~/components/base/MessageBox.vue';
import CloseIcon from '~/components/icons/X.vue';
import Cog from '~/components/icons/Cog.vue';

const emit = defineEmits<{
  (e: 'close'): void
}>();

const modalRef = ref<HTMLElement | null>(null);
const isLoading = ref(true);

// Дефолтное значение для сброса
const DEFAULT_WINE_OVERRIDES = 'concrt140=n;xaudio2_7=n,b;d3d11=n,b;dxgi=n,b;d3dx9_42=n,b;d3dcompiler_47=n,b;dinput8=n,b;mscoree=n';

// Реактивное состояние всех настроек
const settings = ref({
  mangoHud: false,
  fsr: false,
  shaderCache: false,
  hdr: false,
  steamFix: false,
  cdn: false,
  fpsLimit: '60',
  wineDllOverrides: '',
  grafikMod: 'Нету',
  fsrLvl: '95'
});

onMounted(async () => {
  document.addEventListener('click', handleClickOutside);
  try {
    const data = await window.go.main.App.GetGameSettings();
    if (data) {
      const parsed = typeof data === 'string' ? JSON.parse(data) : data;
      settings.value = { ...settings.value, ...parsed };
    }
  } catch (e) {
    console.error('Ошибка загрузки настроек:', e);
  } finally {
    isLoading.value = false;
  }
});

onUnmounted(() => {
  document.removeEventListener('click', handleClickOutside);
});

const handleClickOutside = (event: MouseEvent) => {
  const target = event.target as Node | null;
  if (modalRef.value && target && !modalRef.value.contains(target)) {
    emit('close');
  }
};

// --- УМНАЯ ЛОГИКА ЗАВИСИМОСТЕЙ ---
// Если выключают CDN, а выбран CommunityShader -> сбрасываем на "Нету"
watch(() => settings.value.cdn, async (newCdn) => {
  if (!newCdn && settings.value.grafikMod === 'CommunityShader') {
    settings.value.grafikMod = 'Нету';
    await sendToBackend('grafikMod', 'Нету');
  }
});

// --- ФУНКЦИИ ОТПРАВКИ И ОБРАБОТКИ ---
const toggleSetting = async (key: 'mangoHud' | 'fsr' | 'shaderCache' | 'hdr' | 'steamFix' | 'cdn') => {
  settings.value[key] = !settings.value[key];
  await sendToBackend(key, settings.value[key]);
};

const saveInputSetting = async (key: 'fpsLimit' | 'wineDllOverrides') => {
  await sendToBackend(key, String(settings.value[key]));
};

// Функция сброса WINEDLLOVERRIDES
const resetWineDllOverrides = async () => {
  settings.value.wineDllOverrides = DEFAULT_WINE_OVERRIDES;
  await saveInputSetting('wineDllOverrides');
};

const setGrafikMod = async (mod: string) => {
  if (!settings.value.cdn && mod === 'CommunityShader') return;
  settings.value.grafikMod = mod;
  await sendToBackend('grafikMod', mod);
};

const setFsrLvl = async (lvl: string) => {
  settings.value.fsrLvl = lvl;
  await sendToBackend('fsrLvl', lvl);
};

const sendToBackend = async (key: string, value: boolean | string) => {
  try {
    await window.go.main.App.UpdateSetting(key, value);
    console.log(`[Settings] ${key} сохранено:`, value);
  } catch (e) {
    console.error(`[Settings] Ошибка при сохранении ${key}:`, e);
  }
};
</script>

<template>
  <div class="fixed inset-0 z-[100000] bg-black/70 backdrop-blur-sm flex items-center justify-center px-4">
    <MessageBox ref="modalRef" class="w-full max-w-2xl">
      <div class="flex flex-col gap-6 text-primary w-full relative">
        
        <!-- Заголовок -->
        <div class="flex items-start justify-between gap-3 border-b border-blockBorder pb-4">
          <div class="flex flex-col gap-1">
            <div class="flex items-center gap-2">
              <Cog class="w-6 h-6 text-primary" />
              <h2 class="text-2xl font-semibold tracking-wide">Настройки</h2>
            </div>
            <p class="text-secondary text-sm">Конфигурация параметров запуска</p>
          </div>
          <button
            type="button"
            class="text-secondary hover:text-primary transition-colors p-1"
            @click="emit('close')"
          >
            <CloseIcon class="w-6 h-6" />
          </button>
        </div>

        <div v-if="isLoading" class="flex justify-center items-center py-10">
          <span class="text-secondary animate-pulse font-medium">Загрузка настроек...</span>
        </div>

        <!-- Список настроек -->
        <div v-else class="flex flex-col gap-5">
          
          <!-- Ползунки (Boolean) -->
          <div class="grid grid-cols-2 gap-4">
            <div class="setting-row">
              <span class="setting-label">MangoHud</span>
              <button @click="toggleSetting('mangoHud')" :class="['toggle-btn', settings.mangoHud ? 'bg-primary' : 'bg-blockBorder']">
                <span :class="['toggle-thumb', settings.mangoHud ? 'translate-x-5' : 'translate-x-0']"></span>
              </button>
            </div>
            <div class="setting-row">
              <span class="setting-label">FSR</span>
              <button @click="toggleSetting('fsr')" :class="['toggle-btn', settings.fsr ? 'bg-primary' : 'bg-blockBorder']">
                <span :class="['toggle-thumb', settings.fsr ? 'translate-x-5' : 'translate-x-0']"></span>
              </button>
            </div>
            <div class="setting-row">
              <span class="setting-label">Shader Cache</span>
              <button @click="toggleSetting('shaderCache')" :class="['toggle-btn', settings.shaderCache ? 'bg-primary' : 'bg-blockBorder']">
                <span :class="['toggle-thumb', settings.shaderCache ? 'translate-x-5' : 'translate-x-0']"></span>
              </button>
            </div>
            <div class="setting-row">
              <span class="setting-label">HDR</span>
              <button @click="toggleSetting('hdr')" :class="['toggle-btn', settings.hdr ? 'bg-primary' : 'bg-blockBorder']">
                <span :class="['toggle-thumb', settings.hdr ? 'translate-x-5' : 'translate-x-0']"></span>
              </button>
            </div>
            <div class="setting-row">
              <span class="setting-label">Steam Fix</span>
              <button @click="toggleSetting('steamFix')" :class="['toggle-btn', settings.steamFix ? 'bg-primary' : 'bg-blockBorder']">
                <span :class="['toggle-thumb', settings.steamFix ? 'translate-x-5' : 'translate-x-0']"></span>
              </button>
            </div>
            <!-- Загрузка CDN на месте бывшей заглушки -->
            <div class="setting-row">
              <span class="setting-label">Загрузка CDN</span>
              <button @click="toggleSetting('cdn')" :class="['toggle-btn', settings.cdn ? 'bg-primary' : 'bg-blockBorder']">
                <span :class="['toggle-thumb', settings.cdn ? 'translate-x-5' : 'translate-x-0']"></span>
              </button>
            </div>
          </div>

          <div class="h-px w-full bg-blockBorder my-1"></div>

          <!-- Текстовые поля (Input) -->
          <div class="flex flex-col gap-4">
            
            <div class="flex items-center justify-between">
              <span class="setting-label">Ограничение FPS</span>
              <input 
                v-model="settings.fpsLimit" 
                type="number" 
                class="input-field w-20 text-center"
                @blur="saveInputSetting('fpsLimit')"
                @keyup.enter="saveInputSetting('fpsLimit')"
              />
            </div>
            <div class="flex items-center justify-between">
              <span class="setting-label">WINEDLLOVERRIDES</span>
              <!-- Контейнер для инпута и кнопки -->
              <div class="flex items-center gap-2 w-2/3">
                <input 
                  v-model="settings.wineDllOverrides" 
                  type="text" 
                  placeholder="dxgi=n,b"
                  class="input-field w-full"
                  @blur="saveInputSetting('wineDllOverrides')"
                  @keyup.enter="saveInputSetting('wineDllOverrides')"
                />
                <!-- Кнопка сброса (Квадратная с SVG иконкой) -->
                <button 
                  @click="resetWineDllOverrides"
                  class="w-10 h-10 shrink-0 bg-block border border-blockBorder rounded-xl flex items-center justify-center text-secondary hover:text-primary hover:border-primary transition-colors"
                  title="Сбросить по умолчанию"
                >
                  <svg xmlns="http://www.w3.org/2000/svg" class="w-5 h-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                    <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 4v5h.582m15.356 2A8.001 8.001 0 004.582 9m0 0H9m11 11v-5h-.581m0 0a8.003 8.003 0 01-15.357-2m15.357 2H15" />
                  </svg>
                </button>
              </div>
            </div>
          </div>

          <div class="h-px w-full bg-blockBorder my-1"></div>

          <!-- Специфичные переключатели -->
          <div class="flex flex-col gap-5">
            <!-- Графический Мод -->
            <div class="flex flex-col gap-3">
              <span class="setting-label">Графический мод</span>
              <div class="flex bg-block border border-blockBorder rounded-xl p-1 gap-1">
                <!-- Добавлена опция "Нету" -->
                <button
                  v-for="mod in ['Нету', 'ENB', 'ReShade', 'CommunityShader']"
                  :key="mod"
                  @click="setGrafikMod(mod)"
                  :disabled="mod === 'CommunityShader' && !settings.cdn"
                  :class="[
                    'flex-1 py-2 rounded-lg font-medium transition-colors text-sm',
                    settings.grafikMod === mod ? 'bg-primary text-gray-900' : 'text-secondary hover:text-primary hover:bg-white/5',
                    mod === 'CommunityShader' && !settings.cdn ? 'opacity-30 cursor-not-allowed hover:bg-transparent hover:text-secondary' : ''
                  ]"
                >
                  {{ mod }}
                </button>
              </div>
            </div>  

            <!-- FSR LVL -->
            <!-- FSR LVL (Убрана опция Custom) -->
            <div class="flex flex-col gap-3">
              <span class="setting-label">Уровень FSR</span>
              <div class="flex bg-block border border-blockBorder rounded-xl p-1 gap-1">
                <button
                  v-for="lvl in ['95', '75', '50', '25']"
                  :key="lvl"
                  @click="setFsrLvl(lvl)"
                  :class="[
                    'flex-1 py-2 rounded-lg font-medium transition-colors text-sm',
                    settings.fsrLvl === lvl ? 'bg-primary text-gray-900' : 'text-secondary hover:text-primary hover:bg-white/5'
                  ]"
                >
                  {{ lvl }}%
                </button>
              </div>
            </div>
          </div>

        </div>
      </div>
    </MessageBox>
  </div>
</template>

<style scoped>
.setting-row {
  @apply flex items-center justify-between bg-block/50 px-4 py-3 rounded-xl border border-transparent hover:border-blockBorder transition-colors;
}

.setting-label {
  @apply text-primary font-medium tracking-wide;
}

.toggle-btn {
  @apply w-11 h-6 rounded-full relative transition-colors duration-300 ease-in-out focus:outline-none;
}

.toggle-thumb {
  @apply block w-4 h-4 bg-gray-900 rounded-full mx-1 absolute left-0 top-1 transition-transform duration-300 ease-in-out;
}

.input-field {
  @apply bg-block border border-blockBorder rounded-xl px-3 py-2 text-primary font-medium focus:outline-none focus:border-primary transition-colors;
}

.input-field[type=number]::-webkit-inner-spin-button, 
.input-field[type=number]::-webkit-outer-spin-button { 
  -webkit-appearance: none; 
  margin: 0; 
}
</style>