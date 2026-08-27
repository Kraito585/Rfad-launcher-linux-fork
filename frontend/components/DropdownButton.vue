<script setup lang="ts">
import Package from '~/components/icons/Package.vue';
import Download from '~/components/icons/Download.vue';
import Folder from '~/components/icons/Folder.vue';
import GamepadIcon from '~/components/icons/Gamepad.vue';
import Cog from '~/components/icons/Cog.vue';

interface Events {
  (e: 'update'): void,
  (e: 'openMo2'): void,
  (e: 'openExplorer'): void
  (e: 'start_game'): void
  (e: 'openSettings'): void
}

const props = defineProps<{
  samePadding?: boolean
  hideUpdate?: boolean
}>();

const emit = defineEmits<Events>();

const samePadding = props.samePadding ?? false;

const isDropdownOpen = ref(false);
const firstStart = ref(true)

onMounted(() => {
  firstStart.value = !localStorage.getItem('lastUpdate')
})

const processClick = (e: 'update' | 'openMo2' | 'openExplorer' | 'start_game' | 'openSettings') => {
  emit(e as any);
  isDropdownOpen.value = false;
}
</script>

<template>
  <div class="relative">
    <div
      @click="isDropdownOpen = !isDropdownOpen"
      class="bg-blockTransparent border-blockBorder border-1 rounded-2xl w-fit h-20 py-4 flex items-center justify-center backdrop-blur-sm cursor-pointer"
      :class="{
        'px-10': !samePadding,
        'px-4': samePadding
      }"
    >
      <slot />
    </div>

    <transition name="fade-align">
      <div v-if="isDropdownOpen" class="bg-blockTransparent border-blockBorder border-1 rounded-md w-fit flex items-center justify-center backdrop-blur-sm cursor-pointer absolute bottom-24 right-0">
        <div class="flex flex-col px-4 py-1.5 gap-1.5 min-w-max font-semibold text-base">
          <div
            class="flex flex-row gap-2 items-center cursor-pointer hover:opacity-75 transition-opacity"
            @click="processClick('openSettings')"
          >
            <Cog class="w-4 h-4"/>
            Настройки
          </div>
          <div
            v-if="!firstStart || hideUpdate"
            class="flex flex-row gap-2 items-center cursor-pointer hover:opacity-75 transition-opacity"
            @click="processClick('update')"
          >
            <Download class="w-4 h-4"/>
            Обновить игру
          </div>
          <div
            v-else-if="firstStart && !hideUpdate"
            class="flex flex-row gap-2 items-center cursor-pointer hover:opacity-75 transition-opacity"
            @click="processClick('start_game')"
          >
            <GamepadIcon class="w-4 h-4"/>
            Запустить игру
          </div>
          <div
            class="flex flex-row gap-2 items-center cursor-pointer hover:opacity-75 transition-opacity"
            @click="processClick('openMo2')"
          >
            <Package class="w-4 h-4"/>
            Открыть МО2
          </div>
          <div
            class="flex flex-row gap-2 items-center cursor-pointer hover:opacity-75 transition-opacity"
            @click="processClick('openExplorer')"
          >
            <Folder class="w-4 h-4"/>
            Открыть папку
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
    transition: opacity 0.3s, transform 0.3s;
  }
  .fade-align-leave-active {
    @apply bottom-24 right-0 absolute
  }
  .fade-align-enter-from,
  .fade-align-leave-to {
    opacity: 0;
  }
}
</style>