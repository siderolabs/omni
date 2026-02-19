// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.
import { type Config } from 'prettier'
import { type PluginOptions } from 'prettier-plugin-tailwindcss'

const config: Config & PluginOptions = {
  semi: false,
  singleQuote: true,
  printWidth: 100,
  plugins: ['prettier-plugin-tailwindcss'],
  htmlWhitespaceSensitivity: 'ignore',
  tailwindFunctions: ['cn', 'clsx', 'twMerge'],
  tailwindAttributes: ['toast-options', '/.*-class/'],
  tailwindStylesheet: './src/index.css',
}

export default config
