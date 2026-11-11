import { fileURLToPath, URL } from 'node:url';
import { defineConfig, transformWithEsbuild } from 'vite';
import react from '@vitejs/plugin-react';
import tailwindcss from '@tailwindcss/vite';

export default defineConfig({
  resolve: {
    // "@/components/ui/button" is frontend/src/components/ui/button (shadcn's convention).
    alias: { '@': fileURLToPath(new URL('./src', import.meta.url)) },
  },
  plugins: [
    tailwindcss(),
    {
      name: 'load-js-files-as-jsx',
      async transform(code, id) {
        if (!id.match(/src\/.*\.js$/)) {
          return null;
        }
        return transformWithEsbuild(code, id, {
          loader: 'jsx',
          jsx: 'automatic',
        });
      },
    },
    react(),
  ],
  optimizeDeps: {
    esbuildOptions: {
      loader: {
        '.js': 'jsx',
      },
    },
  },
  server: {
    host: '127.0.0.1',
    port: 4173,
    strictPort: true,
    proxy: {
      '/api': {
        target: 'http://127.0.0.1:8080',
        changeOrigin: true,
        ws: true,
        timeout: 120000,
        proxyTimeout: 120000,
      },
    },
  },
});
