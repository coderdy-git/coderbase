import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'
import tailwindcss from '@tailwindcss/vite'

// https://vite.dev/config/
export default defineConfig({
  plugins: [vue(), tailwindcss()],
  server: {
    proxy: {
      '/dashboard/api': {
        target: 'http://localhost:8080',
        changeOrigin: true,
      },
      '/dashboard/login': {
        target: 'http://localhost:8080',
        changeOrigin: true,
      },
      '/dashboard/logout': {
        target: 'http://localhost:8080',
        changeOrigin: true,
      },
      '/api/v1/realtime': {
        target: 'ws://localhost:8080',
        ws: true,
        changeOrigin: true,
      }
    }
  }
})
