import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'

export default defineConfig({
  plugins: [react()],
  build: { outDir: '../internal/httpapi/assets', emptyOutDir: true },
  server: { proxy: { '/admin': 'http://127.0.0.1:8041', '/healthz': 'http://127.0.0.1:8041' } },
})
