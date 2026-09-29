// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.
import { createSharedComposable, useLocalStorage, usePreferredDark } from '@vueuse/core'
import { computed } from 'vue'

export type ThemePreference = 'light' | 'dark' | 'system'
export type Theme = Exclude<ThemePreference, 'system'>

/**
 * The storage key and values are read before first paint by the inline script
 * in index.html, so change them there too.
 */
export const THEME_STORAGE_KEY = 'theme'

export const useTheme = createSharedComposable(() => {
  const preference = useLocalStorage<ThemePreference>(THEME_STORAGE_KEY, 'system')
  const prefersDark = usePreferredDark()

  // Anything other than light or dark follows the system, including a stored
  // value this build doesn't recognise.
  const theme = computed<Theme>(() => {
    if (preference.value === 'light' || preference.value === 'dark') return preference.value

    return prefersDark.value ? 'dark' : 'light'
  })

  return { preference, theme }
})

/**
 * Puts the theme on <html>, where the design system's `[data-theme="light"]`
 * block is scoped, so dialogs and menus teleported to <body> follow it too.
 */
export function applyTheme(theme: Theme) {
  const root = document.documentElement

  root.dataset.theme = theme
  root.style.colorScheme = theme

  document
    .querySelector<HTMLMetaElement>('meta[name="theme-color"]')
    ?.setAttribute('content', getComputedStyle(root).getPropertyValue('--talos-surface-chrome'))
}
