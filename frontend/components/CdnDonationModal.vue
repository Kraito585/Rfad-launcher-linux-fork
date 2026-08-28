<script setup lang="ts">
import MessageBox from '~/components/base/MessageBox.vue';

const emit = defineEmits<{
  (e: 'close'): void
}>();

// Открытие YooMoney в системном браузере
const openDonation = () => {
  const url = "https://yoomoney.ru/to/4100119587829305";
  
  // Вызываем системный браузер через Wails Runtime API
  if (window.runtime && window.runtime.BrowserOpenURL) {
    window.runtime.BrowserOpenURL(url);
  } else {
    // Фолбэк, если runtime недоступен
    window.open(url, '_blank');
  }

  // Закрываем модалку после того, как пользователь нажал кнопку
  emit('close');
};
</script>

<template>
  <div class="fixed inset-0 z-[100010] bg-black/70 backdrop-blur-sm flex items-center justify-center px-4">
    <!-- MessageBox выступает как фон/рамка нашей модалки -->
    <MessageBox class="w-full max-w-md relative p-6">
      
      <div class="flex flex-col gap-6 items-center text-center">
        <!-- Текст и значок с тултипом -->
        <div class="flex items-center justify-center gap-2 relative">
          <p class="text-primary text-lg font-medium">
            Копеечку на сервера — вам не дорого, мне приятно.
          </p>
          
          <!-- Значок информации с Tooltip -->
          <div class="relative group flex items-center justify-center cursor-help">
            <svg xmlns="http://www.w3.org/2000/svg" class="w-5 h-5 text-secondary hover:text-primary transition-colors" fill="none" viewBox="0 0 24 24" stroke="currentColor">
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M13 16h-1v-4h-1m1-4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z" />
            </svg>
            
            <div class="absolute bottom-full mb-3 left-1/2 -translate-x-1/2 hidden group-hover:block w-64 bg-block border border-blockBorder text-primary text-sm p-3 rounded-xl shadow-xl z-20 pointer-events-none">
              Полная загрузка файлов игры расходует ~7р.
              <div class="absolute top-full left-1/2 -translate-x-1/2 border-4 border-transparent border-t-blockBorder"></div>
            </div>
          </div>
        </div>

        <!-- Кнопки -->
        <div class="flex flex-col gap-4 items-center w-full mt-2">
          
          <!-- Нативная красивая кнопка вместо iframe -->
          <button 
            @click="openDonation"
            class="flex items-center justify-center gap-2 w-full max-w-[330px] bg-[#8a2be2] hover:bg-[#7a1fd1] text-white py-3 px-4 rounded-xl font-medium transition-all shadow-lg hover:shadow-purple-500/25 active:scale-[0.98]"
          >
            <!-- Иконка кошелька -->
            <svg xmlns="http://www.w3.org/2000/svg" class="w-5 h-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M3 10h18M7 15h1m4 0h1m-7 4h12a3 3 0 003-3V8a3 3 0 00-3-3H6a3 3 0 00-3 3v8a3 3 0 003 3z" />
            </svg>
            Поддержать проект
          </button>

          <!-- Кнопка Отмена -->
          <button 
            @click="emit('close')"
            class="px-4 py-2 text-secondary hover:text-primary hover:bg-white/5 rounded-lg transition-colors font-medium text-sm"
          >
            Отмена
          </button>

        </div>
      </div>
      
    </MessageBox>
  </div>
</template>