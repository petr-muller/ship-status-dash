import react from '@vitejs/plugin-react'
import { defineConfig } from 'vite'

export default defineConfig({
  plugins: [react()],
  server: {
    port: 3030,
  },
  build: {
    outDir: 'build',
    chunkSizeWarningLimit: 2000,
  },
})
