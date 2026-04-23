import { defineConfig } from 'vite';
import vue from '@vitejs/plugin-vue';
import { fileURLToPath, URL } from 'node:url';

const proxyTarget = process.env.LLM_GATEWAY_PROXY_TARGET || 'http://localhost:8090';

export default defineConfig({
  base: './',
  plugins: [vue()],
  resolve: {
    alias: {
      '@': fileURLToPath(new URL('./src', import.meta.url)),
    },
  },
  server: {
    host: '0.0.0.0',
    port: 3001,
    proxy: {
      '/api': { target: proxyTarget, changeOrigin: true },
      '/v1': { target: proxyTarget, changeOrigin: true },
      '/chat': { target: proxyTarget, changeOrigin: true },
    },
  },
  build: {
    outDir: 'dist',
    emptyOutDir: true,
  },
});
