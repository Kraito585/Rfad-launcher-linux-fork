<template>
  <Html lang="ru-RU">
    <Head>
      <Meta name="viewport" content="width=device-width, initial-scale=1, maximum-scale=1" />
    </Head>
    <Body class="bg-black background w-full h-full">
      <NuxtPage />
    </Body>
  </Html>
</template>

<style>
.background {
  background-image: url("assets/image/bg.webp");
  background-size: contain;
  background-position: 215px;
  background-repeat: no-repeat;
}
</style>