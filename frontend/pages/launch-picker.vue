<script setup lang="ts">
import { ref, onMounted, onUnmounted } from "vue";
import * as App from "~/bindings/rfad-launcher-linux/app";
import GamepadIcon from "~/components/icons/Gamepad.vue";
import Cog from "~/components/icons/Cog.vue";
import Package from "~/components/icons/Package.vue";
import CloseIcon from "~/components/icons/X.vue";

const remember = ref(false);
const isLaunching = ref(false);

const selectMode = async (mode: "game" | "launcher" | "mo2") => {
  if (isLaunching.value) return;
  isLaunching.value = true;
  try {
    await App.SelectLaunchMode(mode, remember.value);
  } catch (e) {
    console.error("Ошибка выбора режима запуска:", e);
    isLaunching.value = false;
  }
};

const quit = () => {
  App.Quit();
};

const handleGamepadBack = (e: Event) => {
  if (e.defaultPrevented) return;
  e.preventDefault();
  quit();
};

onMounted(() => {
  window.addEventListener("gp-back", handleGamepadBack);
});

onUnmounted(() => {
  window.removeEventListener("gp-back", handleGamepadBack);
});
</script>

<template>
  <div
    class="w-screen h-screen bg-gray-950/95 border border-blockBorder rounded-2xl flex flex-col justify-between p-5 text-primary select-none overflow-hidden"
  >
    <!-- Верхняя панель (перетаскиваемая) -->
    <div
      style="--wails-draggable: drag"
      class="flex items-start justify-between gap-3 border-b border-blockBorder pb-3"
    >
      <div class="flex flex-col">
        <h1 class="text-xl font-bold tracking-wide text-primary">RFAD SE</h1>
        <p class="text-secondary text-xs">Выберите вариант запуска</p>
      </div>
      <button
        type="button"
        data-gp-ignore
        style="--wails-draggable: no-drag"
        class="text-secondary hover:text-primary transition-colors p-1 rounded-full flex items-center justify-center"
        @click="quit"
      >
        <CloseIcon class="w-5 h-5" />
      </button>
    </div>

    <!-- Варианты запуска -->
    <div class="flex flex-col gap-2.5 my-3">
      <button
        type="button"
        data-gp-autofocus
        :disabled="isLaunching"
        class="picker-option bg-primary/15 border-primary/40 hover:bg-primary/25"
        @click="selectMode('game')"
      >
        <GamepadIcon class="w-6 h-6 text-primary shrink-0" />
        <div class="flex flex-col text-left">
          <span class="font-bold text-base text-primary">Играть в RFAD SE</span>
          <span class="text-xs text-secondary"
            >Запустить игру напрямую без окна лаунчера</span
          >
        </div>
      </button>

      <button
        type="button"
        :disabled="isLaunching"
        class="picker-option"
        @click="selectMode('launcher')"
      >
        <Cog class="w-6 h-6 text-primary shrink-0" />
        <div class="flex flex-col text-left">
          <span class="font-bold text-base text-primary">Открыть лаунчер</span>
          <span class="text-xs text-secondary"
            >Обновления, настройки графики и восстановление</span
          >
        </div>
      </button>

      <button
        type="button"
        :disabled="isLaunching"
        class="picker-option"
        @click="selectMode('mo2')"
      >
        <Package class="w-6 h-6 text-primary shrink-0" />
        <div class="flex flex-col text-left">
          <span class="font-bold text-base text-primary">Mod Organizer 2</span>
          <span class="text-xs text-secondary"
            >Управление модами и порядком загрузки</span
          >
        </div>
      </button>
    </div>

    <!-- Запомнить выбор -->
    <div
      class="pt-3 border-t border-blockBorder flex items-center justify-between gap-3"
    >
      <div class="flex flex-col">
        <span class="text-sm font-medium text-primary">Запомнить выбор</span>
        <span class="text-xs text-secondary"
          >Всегда можно сбросить в настройках лаунчера</span
        >
      </div>
      <button
        type="button"
        role="switch"
        :aria-checked="remember"
        :disabled="isLaunching"
        @click="remember = !remember"
        :class="[
          'w-11 h-6 rounded-full relative transition-colors duration-200 shrink-0',
          remember ? 'bg-primary' : 'bg-blockBorder',
        ]"
      >
        <span
          :class="[
            'block w-4 h-4 bg-gray-900 rounded-full mx-1 absolute left-0 top-1 transition-transform duration-200',
            remember ? 'translate-x-5' : 'translate-x-0',
          ]"
        ></span>
      </button>
    </div>
  </div>
</template>

<style scoped>
.picker-option {
  @apply w-full flex items-center gap-3.5 px-4 py-3 rounded-xl bg-white/5 border border-blockBorder hover:border-primary/60 hover:bg-white/10 disabled:opacity-50 transition-colors cursor-pointer;
}
</style>
