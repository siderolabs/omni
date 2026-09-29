// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.
import { createSharedComposable, useLocalStorage, usePreferredDark } from '@vueuse/core'
import { computed } from 'vue'

import { resolveTheme, THEME_STORAGE_KEY, type ThemePreference } from '@/methods/theme-core'

export {
  applyTheme,
  colorScheme,
  isTheme,
  type Theme,
  type ThemePreference,
} from '@/methods/theme-core'

export const useTheme = createSharedComposable(() => {
  const preference = useLocalStorage<ThemePreference>(THEME_STORAGE_KEY, 'system')
  const prefersDark = usePreferredDark()

  const theme = computed(() => resolveTheme(preference.value, prefersDark.value))

  return { preference, theme }
})
