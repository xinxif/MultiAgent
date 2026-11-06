// Vite 配置：开发代理 / 路径别名 / 构建产物切分。
import { defineConfig } from 'vite';
import react from '@vitejs/plugin-react';
import path from 'node:path';

// 后端 Hertz 默认监听 8080（configs/config.yaml: http.port）。
const BACKEND = process.env.AGENT_BACKEND_URL ?? 'http://localhost:8080';

export default defineConfig({
  plugins: [react()],
  resolve: {
    alias: {
      // 统一以 @ 指向 src，避免 ../../../ 地狱
      '@': path.resolve(__dirname, 'src'),
    },
  },
  server: {
    port: 5173,
    // 开发期把 /api 与 /healthz 代理到后端，前后端同源，免 CORS 配置
    proxy: {
      '/api': { target: BACKEND, changeOrigin: true },
      '/healthz': { target: BACKEND, changeOrigin: true },
    },
  },
  build: {
    sourcemap: false,
    chunkSizeWarningLimit: 1200,
    rollupOptions: {
      output: {
        // 大依赖单独分包：GSAP 动画库与 Markdown 渲染器按需加载
        manualChunks: {
          gsap: ['gsap'],
          markdown: ['react-markdown', 'remark-gfm'],
          vendor: ['react', 'react-dom', 'react-router-dom', 'zustand'],
        },
      },
    },
  },
});
