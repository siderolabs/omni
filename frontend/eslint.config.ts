// Copyright (c) 2025 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

import { fileURLToPath } from 'node:url'

import { includeIgnoreFile } from '@eslint/compat'
import pluginVitest from '@vitest/eslint-plugin'
import skipFormatting from '@vue/eslint-config-prettier/skip-formatting'
import { defineConfigWithVueTs, vueTsConfigs } from '@vue/eslint-config-typescript'
import { globalIgnores } from 'eslint/config'
import pluginPlaywright from 'eslint-plugin-playwright'
import simpleImportSort from 'eslint-plugin-simple-import-sort'
import storybook from 'eslint-plugin-storybook'
import pluginVue from 'eslint-plugin-vue'

export default defineConfigWithVueTs(
  includeIgnoreFile(fileURLToPath(new URL('.gitignore', import.meta.url))),
  globalIgnores(['src/api/resources.ts']),

  pluginVue.configs['flat/essential'],
  pluginVue.configs['flat/strongly-recommended'],
  pluginVue.configs['flat/recommended'],
  vueTsConfigs.recommended,

  {
    ...pluginVitest.configs.recommended,
    files: ['src/**/*.spec.*'],
  },
  {
    ...pluginPlaywright.configs['flat/recommended'],
    files: ['e2e/**/*'],
    rules: {
      ...pluginPlaywright.configs['flat/recommended'].rules,
      '@typescript-eslint/no-floating-promises': 'error',
    },
  },
  // By default, ESLint ignores all dot-files
  { ignores: ['!.storybook', '.storybook/public/mockServiceWorker.js'] },
  ...storybook.configs['flat/recommended'],
  ...storybook.configs['flat/csf-strict'],

  skipFormatting,

  {
    name: 'Custom lint rules',
    files: ['**/*.{ts,vue}'],
    plugins: {
      'simple-import-sort': simpleImportSort,
    },
    rules: {
      'simple-import-sort/imports': 'error',
      'simple-import-sort/exports': 'error',
      'vue/block-lang': ['error', { script: { lang: 'ts' } }],
      'vue/block-order': ['error', { order: ['script', 'template', 'style'] }],
      'vue/component-api-style': 'error',
      'vue/component-name-in-template-casing': [
        'error',
        'PascalCase',
        { registeredComponentsOnly: false },
      ],
      'vue/component-options-name-casing': 'error',
      'vue/dot-notation': 'error',
      'vue/eqeqeq': 'error',
      'vue/match-component-file-name': 'error',
      'vue/match-component-import-name': 'error',
      'vue/next-tick-style': 'error',
      'vue/no-boolean-default': 'error',
      'vue/no-template-target-blank': 'error',
      'vue/no-useless-mustaches': 'error',

      // Temporarily disabled rules
      'vue/multi-word-component-names': 'warn',
      '@typescript-eslint/no-explicit-any': 'warn',
    },
  },
)
