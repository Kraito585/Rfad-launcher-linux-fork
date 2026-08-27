<script setup lang="ts">
import MessageBox from '~/components/base/MessageBox.vue';
</script>

<template>
  <MessageBox>
    <div class="flex flex-col w-full justify-between text-primary font-semibold text-sm tracking-wide">
      <div class="font-bold">
        ВНИМАНИЕ!
      </div>
      <div class="font-normal mt-1">
        При обновлении, скорее всего, вам придется начать<br>новую игру.
      </div>
      <slot/>
    </div>
  </MessageBox>
</template>

<style scoped>

</style>