export default defineNuxtConfig({
  compatibilityDate: "2024-11-01",
  devtools: { enabled: true },

  postcss: {
    plugins: {
      tailwindcss: {},
      autoprefixer: {},
    },
  },

  // Отключаем серверный рендеринг
  ssr: false,

  // Указываем Nuxt собирать чистую статику и класть её в папку dist (для Wails)
  nitro: {
    preset: "static",
    output: {
      publicDir: "dist",
    },
  },

  // Включаем хэш-роутинг (надежнее для десктопных SPA-приложений)
  router: {
    options: {
      hashMode: true,
    },
  },

  devServer: { host: "127.0.0.1" },
  css: ["~/assets/css/global.scss"],

  vite: {
    plugins: [],
    clearScreen: false,
    // Убрали TAURI_, оставили дефолтный префикс Vite
    envPrefix: ["VITE_"],
    server: {
      strictPort: true,
      host: "127.0.0.1",
    },
  },
});
