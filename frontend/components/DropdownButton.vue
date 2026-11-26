<script setup lang="ts">
import { ref, onMounted, onUnmounted } from "vue";
import Package from "~/components/icons/Package.vue";
import Download from "~/components/icons/Download.vue";
import Folder from "~/components/icons/Folder.vue";
import GamepadIcon from "~/components/icons/Gamepad.vue";
import Cog from "~/components/icons/Cog.vue";
import Wrench from "~/components/icons/Wrench.vue";
import Terminal from "~/components/icons/Terminal.vue";
import Steam from "~/components/icons/Steam.vue";

interface Events {
  (e: "update"): void;
  (e: "open-mo2"): void;
  (e: "open-explorer"): void;
  (e: "open-settings"): void;
  (e: "start_game"): void;
  (e: "open-recovery"): void;
  (e: "open-protontricks"): void;
  (e: "add-to-steam"): void;
}

const props = defineProps<{
  samePadding?: boolean;
  hideUpdate?: boolean;
}>();

const emit = defineEmits<Events>();

const samePadding = props.samePadding ?? false;

const isDropdownOpen = ref(false);
const firstStart = ref(true);

const handleGamepadBack = (e: Event) => {
  if (isDropdownOpen.value) {
    isDropdownOpen.value = false;
    e.preventDefault();
  }
};

onMounted(() => {
  firstStart.value = !localStorage.getItem("lastUpdate");
  window.addEventListener("gp-back", handleGamepadBack);
});

onUnmounted(() => {
  window.removeEventListener("gp-back", handleGamepadBack);
});

const processClick = (
  e:
    | "update"
    | "openMo2"
    | "openExplorer"
    | "start_game"
    | "openSettings"
    | "open-recovery"
    | "open-protontricks"
    | "add-to-steam",
) => {
  emit(e as any);
  isDropdownOpen.value = false;
};
</script>

<template>
  <div class="relative">
    <div
      data-gp-focusable
      @click="isDropdownOpen = !isDropdownOpen"
      class="bg-blockTransparent border-blockBorder border-1 rounded-2xl w-fit h-20 py-4 flex items-center justify-center backdrop-blur-sm cursor-pointer"
      :class="{
        'px-10': !samePadding,
        'px-4': samePadding,
      }"
    >
      <slot />
    </div>

    <transition name="fade-align">
      <div
        v-if="isDropdownOpen"
        data-gp-modal
        class="bg-blockTransparent border-blockBorder border-1 rounded-md w-fit flex items-center justify-center backdrop-blur-sm cursor-pointer absolute bottom-24 right-0"
      >
        <div
          class="flex flex-col px-3 py-1.5 gap-1 min-w-max font-semibold text-base"
        >
          <div
            data-gp-focusable
            data-gp-autofocus
            class="flex flex-row gap-2 items-center cursor-pointer hover:opacity-75 transition-opacity rounded px-1.5 py-0.5"
            @click="processClick('openSettings')"
          >
            <Cog class="w-4 h-4" />
            Настройки
          </div>
          <div
            v-if="!firstStart || hideUpdate"
            data-gp-focusable
            class="flex flex-row gap-2 items-center cursor-pointer hover:opacity-75 transition-opacity rounded px-1.5 py-0.5"
            @click="processClick('update')"
          >
            <Download class="w-4 h-4" />
            Обновить игру
          </div>
          <div
            v-else-if="firstStart && !hideUpdate"
            data-gp-focusable
            class="flex flex-row gap-2 items-center cursor-pointer hover:opacity-75 transition-opacity rounded px-1.5 py-0.5"
            @click="processClick('start_game')"
          >
            <GamepadIcon class="w-4 h-4" />
            Запустить игру
          </div>
          <div
            data-gp-focusable
            class="flex flex-row gap-2 items-center cursor-pointer hover:opacity-75 transition-opacity rounded px-1.5 py-0.5"
            @click="processClick('openMo2')"
          >
            <Package class="w-4 h-4" />
            Открыть МО2
          </div>
          <div
            data-gp-focusable
            class="flex flex-row gap-2 items-center cursor-pointer hover:opacity-75 transition-opacity rounded px-1.5 py-0.5"
            @click="processClick('openExplorer')"
          >
            <Folder class="w-4 h-4" />
            Открыть папку
          </div>
          <div
            data-gp-focusable
            class="flex flex-row gap-2 items-center cursor-pointer hover:opacity-75 transition-opacity rounded px-1.5 py-0.5"
            @click="processClick('open-recovery')"
          >
            <Wrench class="w-4 h-4" />
            Восстановление
          </div>
          <div
            data-gp-focusable
            class="flex flex-row gap-2 items-center cursor-pointer hover:opacity-75 transition-opacity rounded px-1.5 py-0.5"
            @click="processClick('open-protontricks')"
          >
            <Terminal class="w-4 h-4" />
            Protontricks
          </div>
          <div
            data-gp-focusable
            class="flex flex-row gap-2 items-center cursor-pointer hover:opacity-75 transition-opacity rounded px-1.5 py-0.5"
            @click="processClick('add-to-steam')"
          >
            <Steam class="w-4 h-4" />
            Добавить в Steam
          </div>
        </div>
      </div>
    </transition>
  </div>
</template>

<style lang="scss">
@media (prefers-reduced-motion: no-preference) {
  .fade-align-enter-active,
  .fade-align-leave-active,
  .fade-align-move {
    transition:
      opacity 0.3s,
      transform 0.3s;
  }
  .fade-align-leave-active {
    @apply bottom-24 right-0 absolute;
  }
  .fade-align-enter-from,
  .fade-align-leave-to {
    opacity: 0;
  }
}
</style>
