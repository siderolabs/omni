// Copyright (c) 2025 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

import pluginVitest from '@vitest/eslint-plugin'
import skipFormatting from '@vue/eslint-config-prettier/skip-formatting'
import { defineConfigWithVueTs, vueTsConfigs } from '@vue/eslint-config-typescript'
import { globalIgnores } from 'eslint/config'
import simpleImportSort from 'eslint-plugin-simple-import-sort'
import pluginVue from 'eslint-plugin-vue'

export default defineConfigWithVueTs(
  globalIgnores(['**/dist/**', '**/dist-ssr/**', '**/coverage/**', 'src/api/resources.ts']),

  pluginVue.configs['flat/essential'],
  pluginVue.configs['flat/strongly-recommended'],
  pluginVue.configs['flat/recommended'],
  vueTsConfigs.recommended,

  {
    ...pluginVitest.configs.recommended,
    files: ['test/*'],
  },
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
