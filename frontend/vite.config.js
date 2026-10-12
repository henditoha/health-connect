import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'

// https://vitejs.dev/config/
export default defineConfig({
  plugins: [react()],
  publicDir: 'assets',
  build: {
    outDir: '../backend-go/dist',
    emptyOutDir: true,
  },
  server: {
    port: 3000,
  }
})
