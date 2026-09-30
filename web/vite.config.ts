import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'
import { fileURLToPath, URL } from 'node:url'

// dev 服务默认 5173，与 conf/server.json 的 cors.allowed_origins 保持一致；
// /api 代理到 Go 服务，前端代码不感知后端地址。
export default defineConfig({
  plugins: [vue()],
  resolve: {
    alias: {
      '@': fileURLToPath(new URL('./src', import.meta.url)),
    },
  },
  server: {
    host: '127.0.0.1', // 固定 IPv4，与 dev.ps1 端口检测/curl 一致（默认 localhost 会解析到 ::1）
    port: 5173,
    proxy: {
      '/api': {
        target: 'http://127.0.0.1:8080',
        changeOrigin: true,
      },
    },
  },
})
