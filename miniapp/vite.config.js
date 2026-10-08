import { defineConfig } from 'vite'
import uni from '@dcloudio/vite-plugin-uni'

// uni-app vite 工程配置（与 HBuilderX 编译共用同一份 src/ 源码）
export default defineConfig({
  plugins: [uni()],
  build: {
    // App 资源构建产物较大，限制告警
    chunkSizeWarningLimit: 2000,
  },
})
