<script setup lang="ts">
import MessageBox from '~/components/base/MessageBox.vue';
</script>

<template>
  <MessageBox>
    <div class="flex flex-col w-full gap-1.5 items-center text-primary font-semibold text-sm tracking-wide">
      <div class="text-center w-full">
        <span class="text-red-400">Внимание:</span> Утилита Gamescope не найдена в системе.<br>
        <span class="text-secondary font-normal mt-1 block">Без неё Wine FSR может масштабироваться некорректно. Рекомендуется установить пакет gamescope через ваш пакетный менеджер.</span>
      </div>
      <slot />
    </div>
  </MessageBox>
</template>