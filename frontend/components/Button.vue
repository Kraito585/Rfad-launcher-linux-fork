<script setup lang="ts">
const props = defineProps<{
  samePadding?: boolean
}>();

const samePadding = props.samePadding ?? false;
</script>

<template>
  <div
    class="bg-blockTransparent border-blockBorder border-1 rounded-2xl w-fit h-20 py-4 flex items-center justify-center backdrop-blur-sm cursor-pointer"
    :class="{
      'px-10': !samePadding,
      'px-4': samePadding
    }"
  >
    <slot />
  </div>
</template>

<style scoped>

</style>