<script setup lang="ts">
import MessageBox from '~/components/base/MessageBox.vue';
import CloseIcon from '~/components/icons/X.vue';
import Cog from '~/components/icons/Cog.vue';

const props = defineProps<{
  fpsOptions: number[]
  selectedFps: number | null
  selectedVoice: 'ru' | 'en' | null
  isDirty: boolean
  isSaving: boolean
}>();

const emit = defineEmits<{
  (e: 'update:fps', value: number): void
  (e: 'update:voice', value: 'ru' | 'en'): void
  (e: 'close'): void
  (e: 'save'): void
}>();

const voiceOptions = [
  { value: 'ru', label: 'Русская' },
  { value: 'en', label: 'Английская' }
] as const;

const isFpsOpen = ref(false);
const isVoiceOpen = ref(false);
const modalRef = ref<HTMLElement | null>(null);

const fpsLabel = computed(() => props.selectedFps ?? 'Выберите FPS');
const voiceLabel = computed(() => {
  if (props.selectedVoice === 'ru') return 'Русская';
  if (props.selectedVoice === 'en') return 'Английская';
  return 'Выберите озвучку';
});

const toggleFps = () => {
  isVoiceOpen.value = false;
  isFpsOpen.value = !isFpsOpen.value;
};

const toggleVoice = () => {
  isFpsOpen.value = false;
  isVoiceOpen.value = !isVoiceOpen.value;
};

const selectFps = (value: number) => {
  emit('update:fps', value);
  isFpsOpen.value = false;
};

const selectVoice = (value: 'ru' | 'en') => {
  emit('update:voice', value);
  isVoiceOpen.value = false;
};

const closeDropdowns = () => {
  isFpsOpen.value = false;
  isVoiceOpen.value = false;
};

const handleClickOutside = (event: MouseEvent) => {
  const target = event.target as Node | null;
  if (modalRef.value && target && !modalRef.value.contains(target)) {
    closeDropdowns();
  }
};

onMounted(() => {
  document.addEventListener('click', handleClickOutside);
});

onUnmounted(() => {
  document.removeEventListener('click', handleClickOutside);
});
</script>

<template>
  <div class="fixed inset-0 z-[100000] bg-black/70 backdrop-blur-sm flex items-center justify-center px-4">
    <MessageBox ref="modalRef">
      <div class="flex flex-col gap-5 text-primary min-w-[360px]">
        <div class="flex items-start justify-between gap-3">
          <div class="flex flex-col gap-1">
            <div class="flex items-center gap-2">
              <Cog class="w-5 h-5 text-primary" />
              <h2 class="text-2xl font-semibold">Настройки</h2>
            </div>
            <p class="text-secondary text-sm">Выберите опции</p>
          </div>
          <button
            type="button"
            class="text-secondary hover:text-primary transition-colors"
            @click="emit('close')"
          >
            <CloseIcon class="w-5 h-5" />
          </button>
        </div>

        <div class="flex flex-col gap-3 relative">
          <label class="text-secondary text-sm">Ограничение FPS</label>
          <div class="relative">
            <button
              type="button"
              class="dropdown-button"
              :class="{ 'ring-1 ring-primary/70': isFpsOpen }"
              @click.stop="toggleFps"
            >
              <span>{{ fpsLabel }}</span>
              <span class="dropdown-caret" :class="{ 'rotate-180': isFpsOpen }">▾</span>
            </button>
            <Transition name="fade-scale">
              <div v-if="isFpsOpen" class="dropdown-menu">
                <div
                  v-for="fps in props.fpsOptions"
                  :key="fps"
                  class="dropdown-item"
                  :class="{ 'active': fps === props.selectedFps }"
                  @click.stop="selectFps(fps)"
                >
                  {{ fps }}
                </div>
              </div>
            </Transition>
          </div>
        </div>

        <div class="flex flex-col gap-3 relative">
          <label class="text-secondary text-sm">Озвучка</label>
          <div class="relative">
            <button
              type="button"
              class="dropdown-button"
              :class="{ 'ring-1 ring-primary/70': isVoiceOpen }"
              @click.stop="toggleVoice"
            >
              <span>{{ voiceLabel }}</span>
              <span class="dropdown-caret" :class="{ 'rotate-180': isVoiceOpen }">▾</span>
            </button>
            <Transition name="fade-scale">
              <div v-if="isVoiceOpen" class="dropdown-menu">
                <div
                  v-for="voice in voiceOptions"
                  :key="voice.value"
                  class="dropdown-item"
                  :class="{ 'active': voice.value === props.selectedVoice }"
                  @click.stop="selectVoice(voice.value)"
                >
                  {{ voice.label }}
                </div>
              </div>
            </Transition>
          </div>
        </div>

        <div class="flex items-center justify-end gap-3 pt-2">
          <button
            type="button"
            class="px-4 py-2 rounded-xl border border-blockBorder text-secondary hover:text-primary transition-colors"
            @click="emit('close')"
          >
            Закрыть
          </button>
          <button
            type="button"
            class="px-5 py-2 rounded-xl border border-blockBorder text-primary bg-blockTransparent backdrop-blur-sm transition-opacity"
            :class="{
              'opacity-50 cursor-not-allowed': props.isSaving || !props.isDirty,
              'hover:opacity-80': !props.isSaving && props.isDirty
            }"
            :disabled="props.isSaving || !props.isDirty"
            @click="emit('save')"
          >
            {{ props.isSaving ? 'Сохранение...' : 'Сохранить' }}
          </button>
        </div>
      </div>
    </MessageBox>
  </div>
</template>

<style scoped>
.dropdown-button {
  @apply bg-block border border-blockBorder rounded-xl px-3 py-2.5 text-primary w-full flex items-center justify-between gap-3 transition-colors;
}

.dropdown-button:hover {
  @apply border-primary;
}

.dropdown-menu {
  @apply absolute mt-2 w-full bg-block border border-blockBorder rounded-xl shadow-lg overflow-hidden z-10;
  backdrop-filter: blur(12px);
}

.dropdown-item {
  @apply px-3 py-2 text-primary hover:bg-white/5 cursor-pointer transition-colors;
}

.dropdown-item.active {
  @apply text-primary font-semibold bg-primary/10;
}

.dropdown-caret {
  @apply text-secondary transition-transform;
}

.fade-scale-enter-active,
.fade-scale-leave-active {
  transition: opacity 0.15s ease, transform 0.15s ease;
}

.fade-scale-enter-from,
.fade-scale-leave-to {
  opacity: 0;
  transform: translateY(-4px) scale(0.98);
}

.fade-scale-leave-from,
.fade-scale-enter-to {
  opacity: 1;
  transform: translateY(0) scale(1);
}

</style>
