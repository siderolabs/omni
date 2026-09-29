// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.
import { fileURLToPath, URL } from 'node:url'

import tokens from '@siderolabs/talos-design-system/tokens.json' with { type: 'json' }
import { build, type Plugin } from 'vite'

const srcDir = fileURLToPath(new URL('./src', import.meta.url))

// Only what the pre-paint script needs, not the whole tokens file.
const define = {
  __THEME_COLORS__: JSON.stringify(
    Object.fromEntries(
      (['light', 'dark'] as const).map((theme) => [
        theme,
        {
          page: tokens.semantic[theme]['surface-page'].value,
          chrome: tokens.semantic[theme]['surface-chrome'].value,
        },
      ]),
    ),
  ),
}

/**
 * Inlines src/theme-init.ts as a classic script in <head>, so the theme is set
 * before first paint. Also defines `__THEME_COLORS__`.
 */
export function themeInit(): Plugin {
  return {
    name: 'theme-init',
    config: () => ({ define }),
    async transformIndexHtml() {
      const result = await build({
        configFile: false,
        logLevel: 'silent',
        define,
        resolve: { alias: { '@': srcDir } },
        build: {
          write: false,
          copyPublicDir: false,
          rolldownOptions: {
            input: `${srcDir}/theme-init.ts`,
            output: { format: 'iife' },
          },
        },
      })

      const [output] = Array.isArray(result) ? result : [result]

      if (!('output' in output)) throw new Error('theme-init: unexpected build result')

      return [
        {
          tag: 'script',
          attrs: { nonce: '{{.Nonce}}' },
          children: output.output[0].code,
          injectTo: 'head-prepend',
        },
      ]
    },
  }
}
