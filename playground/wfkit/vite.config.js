import { defineConfig } from 'vite'
import { resolve } from 'path'
import webflowVitePlugin from './build/webflow-vite-plugin'

export default defineConfig({
  plugins: [
    webflowVitePlugin({
      pagesDir: 'src/pages',
      globalEntry: 'src/global/index.ts'
    })
  ],
  resolve: {
    alias: {
      '@': resolve(__dirname, 'src')
    }
  },
  build: {
    outDir: 'dist/assets',
    manifest: false,
    minify: 'terser',
    terserOptions: {
      format: { comments: false }
    }
  }
})
