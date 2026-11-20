import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'
import tailwindcss from '@tailwindcss/vite'
import { fileURLToPath, URL } from 'node:url'

// 只代理 /system/api 与 /user/api（不代理整个 /system，否则深链接刷新会被
// 网关守卫 401）。B 端 Host=api.devops.local，C 端 Host=user-api.devops.local。
const gatewayTarget = process.env.VITE_GATEWAY_TARGET || 'http://127.0.0.1:8080'
const systemHost = process.env.VITE_GATEWAY_SYSTEM_HOST || 'api.devops.local'
const userHost = process.env.VITE_GATEWAY_USER_HOST || 'user-api.devops.local'
const direct = process.env.VITE_DIRECT_SERVICE === '1'

export default defineConfig({
  plugins: [react(), tailwindcss()],
  resolve: {
    alias: {
      '@': fileURLToPath(new URL('./src', import.meta.url)),
    },
  },
  server: {
    port: 5173,
    host: '127.0.0.1',
    proxy: {
      '/system/api': direct
        ? {
            target: process.env.VITE_API_PROXY_TARGET || 'http://127.0.0.1:9999',
            changeOrigin: true,
          }
        : {
            target: gatewayTarget,
            changeOrigin: true,
            headers: { Host: systemHost },
          },
      '/user/api': direct
        ? {
            target: process.env.VITE_USER_PROXY_TARGET || 'http://127.0.0.1:10000',
            changeOrigin: true,
          }
        : {
            target: gatewayTarget,
            changeOrigin: true,
            headers: { Host: userHost },
          },
    },
  },
  build: {
    chunkSizeWarningLimit: 1600,
    rollupOptions: {
      input: {
        main: fileURLToPath(new URL('./index.html', import.meta.url)),
        portal: fileURLToPath(new URL('./portal.html', import.meta.url)),
      },
    },
  },
  test: {
    environment: 'jsdom',
    globals: true,
    setupFiles: ['./src/test/setup.ts'],
    css: false,
  },
})
