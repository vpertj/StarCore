import { defineConfig } from 'vite'
import { svelte } from '@sveltejs/vite-plugin-svelte'
import tailwindcss from '@tailwindcss/vite'

export default defineConfig({
  plugins: [svelte(), tailwindcss()],
  resolve: {
    alias: {
      '@': '/src'
    }
  },
  build: {
    chunkSizeWarningLimit: 800,
    rollupOptions: {
      output: {
        manualChunks: {
          'vendor-codemirror': [
            'codemirror',
            '@codemirror/state',
            '@codemirror/view',
            '@codemirror/language',
            '@codemirror/commands',
            '@codemirror/lint',
            '@codemirror/matchbrackets'
          ],
          'vendor-codemirror-langs': [
            '@codemirror/lang-javascript',
            '@codemirror/lang-go',
            '@codemirror/lang-python',
            '@codemirror/lang-json',
            '@codemirror/lang-html',
            '@codemirror/lang-css',
            '@codemirror/lang-markdown',
            '@codemirror/lang-xml',
            '@codemirror/lang-yaml',
            '@codemirror/lang-sql',
            '@codemirror/lang-rust',
            '@codemirror/lang-java',
            '@codemirror/lang-cpp',
            '@codemirror/lang-php'
          ],
          'vendor-xterm': ['@xterm/xterm', '@xterm/addon-fit', '@xterm/addon-web-links']
        }
      }
    }
  }
})