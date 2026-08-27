<script setup lang="ts">
import Box from '~/components/base/Box.vue';
import Clock from '~/components/icons/Clock.vue';
import User from '~/components/icons/User.vue';

export interface ModComponentProps {
  image: string
  date: string
  author: string
  name: string
  description: string
  url: string
}

const props = defineProps<ModComponentProps>()
</script>

<template>
  <NuxtLink :to="props.url" target="_blank">
    <Box>
      <div class="flex flex-col w-86 gap-4 items-center text-secondary font-semibold text-sm tracking-wide">
        <div class="flex flex-row w-full items-center gap-4">
          <img
            :src="props.image"
            alt=""
            class="rounded-2xl w-20 h-20"
          >
          <div class="flex flex-col gap-1.5">
            <div class="font-bold text-xl">
              {{ props.name }}
            </div>
            <div class="flex flex-row gap-3 [&>div]:flex [&>div]:flex-row [&>div]:gap-2 [&>div]:items-center [&>div]:font-medium">
              <div>
                <Clock class="w-3.5 h-3.5"/>
                {{ props.date }}
              </div>
              <div>
                <User class="w-3.5 h-3.5"/>
                {{ props.author }}
              </div>
            </div>
          </div>
        </div>
        <div class="font-normal">
          {{ props.description }}
        </div>
      </div>
    </Box>
  </NuxtLink>

</template>

<style scoped>

</style>