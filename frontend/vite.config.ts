// Copyright (c) 2025 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

import { fileURLToPath, URL } from 'node:url'

import tailwindcss from '@tailwindcss/vite'
import vue from '@vitejs/plugin-vue'
import dotenv from 'dotenv'
import { defineConfig, type UserConfig } from 'vite'
import monacoEditorPlugin from 'vite-plugin-monaco-editor-esm'
import { nodePolyfills } from 'vite-plugin-node-polyfills'
import vueDevTools from 'vite-plugin-vue-devtools'
import { configDefaults } from 'vitest/config'

dotenv.config({ quiet: true })

// https://vitejs.dev/config/
export default defineConfig(({ command }) => {
  const isTest = process.env.NODE_ENV === 'test' || process.env.VITEST

  const config: UserConfig = {
    plugins: [vue(), tailwindcss(), nodePolyfills({ include: ['stream'] })],
    resolve: {
      alias: {
        '@': fileURLToPath(new URL('./src', import.meta.url)),
        // The lightweight package can't get resolved in tests
        ...(isTest && { 'openpgp/lightweight': 'openpgp' }),
      },
    },
    test: {
      setupFiles: ['vitest.setup.ts'],
      environment: 'jsdom',
      exclude: [...configDefaults.exclude, 'e2e/**'],
      root: fileURLToPath(new URL('./', import.meta.url)),
      alias: {
        '@msw': fileURLToPath(new URL('./msw', import.meta.url)),
      },
    },
    server: {
      port: 8121,
      host: '127.0.0.1',
    },
  }

  if (process.env.ENABLE_DEVTOOLS) {
    config.plugins?.push(vueDevTools())
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
