// Copyright (c) 2025 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

/// <reference types="vitest/config" />

import { fileURLToPath, URL } from 'node:url'

import tailwindcss from '@tailwindcss/vite'
import vue from '@vitejs/plugin-vue'
import { defineConfig, type UserConfig } from 'vite'
import monacoEditorPlugin from 'vite-plugin-monaco-editor-esm'
import { nodePolyfills } from 'vite-plugin-node-polyfills'
import vueDevTools from 'vite-plugin-vue-devtools'

// https://vitejs.dev/config/
export default defineConfig(({ command }) => {
  const config: UserConfig = {
    plugins: [vue(), vueDevTools(), tailwindcss(), nodePolyfills({ include: ['stream'] })],
    resolve: {
      alias: {
        '@': fileURLToPath(new URL('./src', import.meta.url)),
      },
    },
    test: {
      environment: 'jsdom',
      root: fileURLToPath(new URL('./', import.meta.url)),
    },
    server: {
      port: 8121,
      host: '127.0.0.1',
    },
  }

  if (command === 'serve') {
    config.plugins?.push(
      monacoEditorPlugin({
        languageWorkers: ['editorWorkerService'],
        customWorkers: [
          {
            label: 'yaml',
            entry: 'monaco-yaml/yaml.worker',
          },
        ],
      }),
    )
  }

  return config
})
