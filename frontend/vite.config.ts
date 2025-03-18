// Copyright (c) 2025 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

/// <reference types="vitest" />

import { defineConfig, UserConfig } from 'vite'
import { fileURLToPath, URL } from 'node:url'
import { nodePolyfills } from 'vite-plugin-node-polyfills'
import monacoEditorPlugin from 'vite-plugin-monaco-editor'

import vue from '@vitejs/plugin-vue'

// https://vitejs.dev/config/
export default defineConfig(({ command }) => {
  const config: UserConfig = {
    plugins: [
      vue(),
      nodePolyfills({ include: ['stream'] }),
    ],
    resolve: {
      alias: {
        '@': fileURLToPath(new URL('./src', import.meta.url)),
      }
    },
    server: {
      port: 8121,
      host: "127.0.0.1"
    },
  };

  if (command === 'serve') {
    config.plugins?.push(monacoEditorPlugin({
      languageWorkers: ['editorWorkerService'],
      customWorkers: [
        {
          label: 'yaml',
          entry: 'monaco-yaml/yaml.worker'
        }
      ]
    }));
  }

  return config;
})
