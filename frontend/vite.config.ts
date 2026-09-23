import { defineConfig } from 'vite';
import react from '@vitejs/plugin-react';

export default defineConfig({
  root: import.meta.dirname,
  publicDir: false,
  plugins: [react()],
  build: {
    outDir: '../backend/static',
    emptyOutDir: false,
  },
  server: {
    host: '127.0.0.1',
    proxy: {
      '/api': { target: 'http://127.0.0.1:8080', ws: true },
    },
  },
});
