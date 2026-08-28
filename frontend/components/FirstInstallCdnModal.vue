<script setup lang="ts">
import { ref } from 'vue';
import MessageBox from '~/components/base/MessageBox.vue';

const emit = defineEmits<{
  (e: 'close'): void
  (e: 'toggleCdn', value: boolean): void
}>();

const props = defineProps<{
  initialCdnState: boolean
}>();

const isCdnEnabled = ref(props.initialCdnState);

// Переключение CDN
const toggleCdn = async () => {
  isCdnEnabled.value = !isCdnEnabled.value;
  emit('toggleCdn', isCdnEnabled.value);
  
  try {
    await window.go.main.App.UpdateSetting('cdn', isCdnEnabled.value);
  } catch (e) {
    console.error('Ошибка при сохранении CDN:', e);
  }
};

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

  // Наша умная логика: если CDN включен, закрываем модалку после клика на донат
  if (isCdnEnabled.value) {
    emit('close');
  }
};
</script>

<template>
  <div class="fixed inset-0 z-[100010] bg-black/70 backdrop-blur-sm flex items-center justify-center px-4">
    <MessageBox class="w-full max-w-md relative p-6">
      
      <div class="flex flex-col gap-6 w-full">
        
        <!-- ВЕРХНЯЯ ЧАСТЬ: Информация и ползунок -->
        <div class="flex flex-col gap-4 items-center text-center">
          <p class="text-primary text-sm font-medium leading-relaxed px-2">
            Загрузка с зеркала от 2x до 8x быстрее в зависимости от нагрузки на google аккаунт
          </p>
          
          <div class="flex items-center justify-between w-full bg-block/50 px-4 py-3 rounded-xl border border-transparent hover:border-blockBorder transition-colors">
            <span class="text-primary font-medium tracking-wide">Загрузка CDN</span>
            <button 
              @click="toggleCdn" 
              :class="['w-11 h-6 rounded-full relative transition-colors duration-300 ease-in-out focus:outline-none', isCdnEnabled ? 'bg-primary' : 'bg-blockBorder']"
            >
              <span :class="['block w-4 h-4 bg-gray-900 rounded-full mx-1 absolute left-0 top-1 transition-transform duration-300 ease-in-out', isCdnEnabled ? 'translate-x-5' : 'translate-x-0']"></span>
            </button>
          </div>
        </div>

        <div class="h-px w-full bg-blockBorder my-1"></div>

        <!-- НИЖНЯЯ ЧАСТЬ: Донат и кнопка "Назад" -->
        <div class="flex flex-col gap-4 items-center text-center">
          
          <div class="flex items-center justify-center gap-2 relative">
            <p class="text-primary text-lg font-medium">
              Копеечку на сервера — вам не дорого, мне приятно.
            </p>
            
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

          <!-- Нативная красивая кнопка вместо iframe -->
          <button 
            @click="openDonation"
            class="flex items-center justify-center gap-2 w-full max-w-[330px] bg-[#8a2be2] hover:bg-[#7a1fd1] text-white py-3 px-4 rounded-xl font-medium transition-all shadow-lg hover:shadow-purple-500/25 active:scale-[0.98] mt-2"
          >
            <!-- Иконка кошелька -->
            <svg xmlns="http://www.w3.org/2000/svg" class="w-5 h-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M3 10h18M7 15h1m4 0h1m-7 4h12a3 3 0 003-3V8a3 3 0 00-3-3H6a3 3 0 00-3 3v8a3 3 0 003 3z" />
            </svg>
            Поддержать проект
          </button>

          <button 
            @click="emit('close')"
            class="px-4 py-2 mt-2 text-secondary hover:text-primary hover:bg-white/5 rounded-lg transition-colors font-medium text-sm"
          >
            Назад
          </button>
          
        </div>
        
      </div>
      
    </MessageBox>
  </div>
</template>