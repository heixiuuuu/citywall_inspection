import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'

// dev 服务默认 5173，与 conf/server.json 的 cors.allowed_origins 保持一致。
export default defineConfig({
  plugins: [vue()],
  server: {
    port: 5173,
    proxy: {
      '/api': {
        target: 'http://127.0.0.1:8080',
        changeOrigin: true,
      },
    },
  },
})
