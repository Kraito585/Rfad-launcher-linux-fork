<script setup lang="ts">
import Box from '~/components/base/Box.vue';
import Clock from '~/components/icons/Clock.vue';
import User from '~/components/icons/User.vue';

export interface PatchComponentProps {
  version: string
  date: string
  author: string
  name: string
  description: string
  url: string
}

const props = defineProps<PatchComponentProps>()
</script>

<template>
  <NuxtLink :to="props.url" target="_blank">
    <Box>
      <div class="flex flex-col w-86 gap-4 items-center text-secondary font-semibold text-sm tracking-wide">
        <div class="flex flex-row w-full items-center gap-4">
          <div class="whitespace-pre-wrap font-black italic font-hubot text-4xl version-gradient tabular-nums tracking-wider leading-8 w-14">
            {{ props.version.split('.').join('\n') }}
          </div>
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
        <div class="font-normal whitespace-pre-line min-w-full">
          {{ props.description }}
        </div>
      </div>
    </Box>
  </NuxtLink>

</template>

<style scoped>
.version-gradient {
  background: linear-gradient(133.781deg, #FFEABF 0%, rgba(255, 234, 191, 0%) 100%);
  -webkit-background-clip: text;
  -webkit-text-fill-color: transparent;
  background-clip: text;
}
</style>