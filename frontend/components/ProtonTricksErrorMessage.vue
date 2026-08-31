<script setup lang="ts">
import MessageBox from '~/components/base/MessageBox.vue';
</script>

<template>
  <MessageBox>
    <div class="flex flex-col w-full gap-1.5 items-center text-primary font-semibold text-sm tracking-wide">
      <div class="text-center w-full">
        ProtonTricks не найден
      </div>
      <slot />
    </div>
  </MessageBox>
</template>