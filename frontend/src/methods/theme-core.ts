// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

// No imports: this is inlined into index.html via src/theme-init.ts.

export type ThemePreference = 'light' | 'dark' | 'dim' | 'system'
export type Theme = Exclude<ThemePreference, 'system'>

export const THEME_STORAGE_KEY = 'theme'

/** Dim counts as dark. */
export const colorScheme = (theme: Theme): 'light' | 'dark' =>
  theme === 'light' ? 'light' : 'dark'

export const isTheme = (value: unknown): value is Theme =>
  value === 'light' || value === 'dark' || value === 'dim'

/** Unrecognised values follow the system. */
export const resolveTheme = (preference: unknown, prefersDark: boolean): Theme => {
  if (isTheme(preference)) return preference

  return prefersDark ? 'dark' : 'light'
}

/**
 * Sets the theme on <html>, so teleported content follows it. The background is
 * set inline so it paints before the stylesheet loads.
 */
export function applyTheme(theme: Theme) {
  const root = document.documentElement
  const colors = __THEME_COLORS__[theme]

  root.dataset.theme = theme
  root.style.colorScheme = colorScheme(theme)
  root.style.backgroundColor = colors.page

  document
    .querySelector<HTMLMetaElement>('meta[name="theme-color"]')
    ?.setAttribute('content', colors.chrome)
}
