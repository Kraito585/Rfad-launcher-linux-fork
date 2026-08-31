<script setup lang="ts">
import { ref } from 'vue';

const emit = defineEmits<{
  (e: 'close'): void
}>();

// Состояние загрузки, чтобы блокировать кнопки от двойных нажатий
const processing = ref<string | null>(null);

const handleAction = async (target: string, action: string) => {
  if (processing.value) return;
  
  processing.value = `${target}-${action}`;
  const isForceReinstall = action === 'reinstall';
  
  try {
    console.log(`Запрос к Go: RecoverComponent(key: '${target}', force: ${isForceReinstall})`);
    
    // Вызываем нашу новую универсальную функцию в бэкенде
    await window.go.main.App.RecoverComponent(target, isForceReinstall);
    
    // Опционально: можно добавить уведомление об успешном завершении
    console.log(`Восстановление ${target} успешно завершено`);
  } catch (e) {
    console.error(`Ошибка при ${action} ${target}:`, e);
    // Здесь можно вызвать window.go.main.App.ShowMessageDialog для отображения ошибки
  } finally {
    processing.value = null;
  }
};
</script>

<template>
  <div class="fixed inset-0 flex items-center justify-center bg-black/60 backdrop-blur-sm z-[100005]" @click.self="emit('close')">
    <div class="bg-gray-900 border border-gray-700 rounded-2xl p-8 w-[600px] flex flex-col gap-6 text-primary shadow-2xl">
      <h2 class="text-3xl font-bold text-center tracking-wider">Восстановление</h2>
      
      <div class="flex flex-col gap-4">
        
        <!-- Proton/Wine -->
        <div class="flex flex-row justify-between items-center bg-white/5 p-4 rounded-xl border border-white/10">
          <div class="font-semibold text-xl tracking-wide">Proton/Wine</div>
          <div class="flex gap-3">
            <button 
              @click="handleAction('proton', 'reinstall')"
              :disabled="processing !== null"
              class="bg-white/10 hover:bg-white/20 disabled:opacity-50 px-4 py-2 rounded-lg transition font-medium"
            >
              Переустановить
            </button>
          </div>
        </div>

        <!-- Prefix -->
        <div class="flex flex-row justify-between items-center bg-white/5 p-4 rounded-xl border border-white/10">
          <div class="font-semibold text-xl tracking-wide">Prefix</div>
          <div class="flex gap-3">
            <button 
              @click="handleAction('prefix', 'reinstall')"
              :disabled="processing !== null"
              class="bg-white/10 hover:bg-white/20 disabled:opacity-50 px-4 py-2 rounded-lg transition font-medium"
            >
              Переустановить
            </button>
            <button 
              @click="handleAction('prefix', 'heal')"
              :disabled="processing !== null"
              class="bg-primary/20 text-primary hover:bg-primary/30 disabled:opacity-50 px-4 py-2 rounded-lg transition font-medium"
            >
              Вылечить
            </button>
          </div>
        </div>

        <!-- Steam Fix -->
        <div class="flex flex-row justify-between items-center bg-white/5 p-4 rounded-xl border border-white/10">
          <div class="font-semibold text-xl tracking-wide">Steam Fix</div>
          <div class="flex gap-3">
            <button 
              @click="handleAction('steamfix', 'heal')"
              :disabled="processing !== null"
              class="bg-primary/20 text-primary hover:bg-primary/30 disabled:opacity-50 px-4 py-2 rounded-lg transition font-medium"
            >
              Вылечить
            </button>
          </div>
        </div>

      </div>

      <!-- Кнопка возврата -->
      <button 
        @click="emit('close')" 
        class="mt-2 w-full bg-white/5 hover:bg-white/10 text-secondary hover:text-primary font-bold text-xl py-3 rounded-xl transition border border-transparent hover:border-white/10"
      >
        Вернуться
      </button>
    </div>
  </div>
</template>