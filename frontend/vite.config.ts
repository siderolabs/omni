// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.
import { fileURLToPath, URL } from 'node:url'

import { faker } from '@faker-js/faker'
import tailwindcss from '@tailwindcss/vite'
import vue from '@vitejs/plugin-vue'
import dotenv from 'dotenv'
import { defineConfig, type UserConfig } from 'vite'
import monacoEditorPlugin from 'vite-plugin-monaco-editor-esm'
import vueDevTools from 'vite-plugin-vue-devtools'
import { configDefaults } from 'vitest/config'

dotenv.config({ quiet: true })

// https://vitejs.dev/config/
export default defineConfig(({ command }) => {
  const isTest = process.env.NODE_ENV === 'test' || process.env.VITEST

  const config: UserConfig = {
    plugins: [vue(), tailwindcss()],
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
      // See customizing the dev server in the readme for allowedHosts
    },
  }

  if (process.env.ENABLE_DEVTOOLS) {
    config.plugins?.push(vueDevTools())
  }

  if (command === 'serve') {
    const cspNonce = faker.string.alphanumeric(14)

    // Inject CSP for dev server for testing.
    // Actual CSP in production is set in ../internal/frontend/handler.go
    // Ideally these sources should match.
    config.server ||= {}
    config.server.headers ||= {}
    config.server.headers['content-security-policy'] = [
      'upgrade-insecure-requests',
      "default-src 'self'",
      `script-src 'self' 'nonce-${cspNonce}' https://*.userpilot.io`,
      "media-src 'self' https://js.userpilot.io",
      'img-src * data:',
      "connect-src 'self' https://factory.staging.talos.dev https://factory.talos.dev https://*.auth0.com https://*.userpilot.io wss://*.userpilot.io",
      "font-src 'self' data: https://fonts.googleapis.com https://fonts.gstatic.com https://fonts.userpilot.io",
      "style-src 'self' 'unsafe-inline' data: https://fonts.googleapis.com",
      'frame-src https://*.auth0.com',
      "worker-src 'self' blob:", // "worker-src blob:" only required for vite dev server
    ].join(';')

    // Adds nonce for dev server inline scripts.
    // Note that it also adds an extra meta tag,
    // but we should only rely on the one present in index.html
    // as it is the only one present in production.
    config.html ||= {}
    config.html.cspNonce = cspNonce

    config.plugins ||= []
    config.plugins.push(
      {
        // Injects nonce into the placeholder as the backend handler usually would.
        name: 'transformIndexHtml',
        transformIndexHtml: (html) => html.replaceAll(/{{.Nonce}}/g, cspNonce),
      },
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
