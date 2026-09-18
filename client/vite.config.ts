import { defineConfig } from 'vitest/config';
import react from '@vitejs/plugin-react';

// D-42: client always calls relative /api/*; Vite proxies to the API in dev.
export default defineConfig({
  plugins: [react()],
  server: {
    port: 5173,
    proxy: {
      '/api': {
        target: 'http://localhost:3000',
        changeOrigin: true,
      },
    },
  },
  test: {
    environment: 'jsdom',
    setupFiles: ['./tests/setup.ts'],
  },
});
