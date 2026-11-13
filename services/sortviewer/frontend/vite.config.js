import { fileURLToPath, URL } from 'node:url';
import { defineConfig, transformWithEsbuild } from 'vite';
import react from '@vitejs/plugin-react';
import tailwindcss from '@tailwindcss/vite';

// The sort viewer's pages. In development the API (and Gangway sign-in) go through the
// proxy to the Go server on :8090.
export default defineConfig({
  resolve: {
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
        return transformWithEsbuild(code, id, { loader: 'jsx', jsx: 'automatic' });
      },
    },
    react(),
  ],
  optimizeDeps: {
    esbuildOptions: { loader: { '.js': 'jsx' } },
  },
  server: {
    host: '0.0.0.0',
    port: 4180,
    strictPort: true,
    proxy: {
      '/api': { target: 'http://127.0.0.1:8090', changeOrigin: false },
      '/auth': { target: 'http://127.0.0.1:8090', changeOrigin: false },
    },
  },
});
