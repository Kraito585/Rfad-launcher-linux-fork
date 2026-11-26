<script setup lang="ts">
import { useGamepadNav } from "./composables/useGamepadNav";
useGamepadNav();
</script>

<template>
  <Html lang="ru-RU">
    <Head>
      <Meta
        name="viewport"
        content="width=device-width, initial-scale=1, maximum-scale=1"
      />
    </Head>
    <Body class="bg-black background w-full h-full">
      <NuxtPage />
    </Body>
  </Html>
</template>

<style>
:focus:not(:focus-visible) {
  outline: none;
}

html.gamepad-mode {
  cursor: none;
}

html.gamepad-mode button:focus,
html.gamepad-mode a:focus,
html.gamepad-mode input:focus,
html.gamepad-mode select:focus,
html.gamepad-mode textarea:focus,
html.gamepad-mode [data-gp-focusable]:focus,
html.gamepad-mode [tabindex]:focus {
  outline: none !important;
  box-shadow:
    0 0 0 2px rgba(0, 0, 0, 0.7),
    0 0 0 4px #ffeabf,
    0 0 14px rgba(255, 234, 191, 0.45) !important;
  transition: box-shadow 0.12s ease;
}
</style>
