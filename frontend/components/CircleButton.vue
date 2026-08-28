<script setup lang="ts">
defineEmits<{
  (e: 'click', event: MouseEvent): void;
}>();
</script>

<template>
  <div
    @click="$emit('click', $event)"
    class="bg-block border-blockBorder border-1 rounded-full w-16 h-16 flex items-center justify-center cursor-pointer hover:opacity-80 transition-opacity"
  >
    <div class="pointer-events-none flex items-center justify-center">
      <slot />
    </div>
  </div>
</template>

<style scoped>
</style>