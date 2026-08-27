import { defineConfig } from 'vitest/config'
import vue from '@vitejs/plugin-vue'
import { resolve } from 'path'

export default defineConfig({
  plugins: [vue()],
  base: '/app/',
  resolve: {
    alias: {
      '@': resolve(import.meta.dirname, 'src'),
    },
  },
  build: {
    outDir: resolve(import.meta.dirname, '../internal/server/webapp'),
    emptyOutDir: true,
  },
  server: {
    proxy: {
      '/api': {
        target: process.env.DIPOLE_WEB_PROXY_TARGET || 'http://localhost:80',
        changeOrigin: true,
        ws: true,
      },
    },
  },
  test: {
    environment: 'jsdom',
    globals: true,
    include: ['src/**/*.test.ts'],
  },
})
