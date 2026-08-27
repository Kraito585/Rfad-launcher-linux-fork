<script setup lang="ts">
import MessageBox from '~/components/base/MessageBox.vue';
import config from '~/config';
</script>

<template>
  <MessageBox>
    <div class="flex flex-col w-full justify-between text-primary font-semibold text-sm tracking-wide">
      <div>
        ОШИБКА!
      </div>
      <div class="font-normal">
        Не удалось получить актуальную версию патча. Обратитесь на наш
        <a :href="config.discord" target="_blank" class="font-semibold underline hover:opacity-80 transition-opacity">Discord-сервер</a>.
      </div>
    </div>
  </MessageBox>
</template>

<style scoped>

</style>
