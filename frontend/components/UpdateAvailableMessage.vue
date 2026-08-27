<script setup lang="ts">
import MessageBox from '~/components/base/MessageBox.vue';

const props = defineProps<{
  version: string
}>();
</script>

<template>
  <MessageBox>
    <div class="flex flex-col w-full justify-between text-primary font-semibold text-sm tracking-wide">
      <div class="flex flex-row w-full justify-between items-center">
        <div>
          Доступно обновление!
        </div>
        <div>
          {{ props.version }}
        </div>
      </div>
      <slot/>
    </div>
  </MessageBox>
</template>

<style scoped>

</style>