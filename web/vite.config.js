import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'

// In compose the API container is reachable as http://api:8080 (set
// via API_PROXY); outside Docker it defaults to a locally running server.
export default defineConfig({
  plugins: [vue()],
  server: {
    host: true,
    port: 5173,
    proxy: {
      '/api': {
        target: process.env.API_PROXY || 'http://localhost:8080',
        changeOrigin: true,
      },
    },
  },
})
