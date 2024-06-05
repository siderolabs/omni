// Copyright (c) 2024 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

// THIS FILE WAS AUTOMATICALLY GENERATED, PLEASE DO NOT EDIT.
//
// Generated on 2024-06-06T10:10:11Z by kres 827a05c-dirty.

// run bun install eslint-plugin-vue typescript-eslint -d for each frontend
// to make the linter work
//@ts-check
import pluginVue from 'eslint-plugin-vue'
import tseslint from 'typescript-eslint';

export default [
  ...tseslint.configs.recommended,
  ...pluginVue.configs['flat/essential'],
  {
    rules: {
      "@typescript-eslint/no-explicit-any": "off",
      "no-console": "off",
      "vue/multi-word-component-names": "off",
      "vue/no-unused-vars": "error"
    },
    plugins: {
      'typescript-eslint': tseslint.plugin,
    },
    languageOptions: {
      parserOptions: {
        parser: tseslint.parser,
        project: './tsconfig.json',
        extraFileExtensions: ['.vue'],
        sourceType: 'module',
      },
    },
  }
]

