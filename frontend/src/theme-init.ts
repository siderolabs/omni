// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

// Applies the theme before first paint. Inlined into <head> by
// vite-plugins/theme-init.ts, as module scripts run too late.

import { applyTheme, resolveTheme, THEME_STORAGE_KEY } from '@/methods/theme-core'

let preference: string | null = null

try {
  preference = localStorage.getItem(THEME_STORAGE_KEY)
} catch {
  // Blocked storage: follow the system.
}

applyTheme(resolveTheme(preference, window.matchMedia('(prefers-color-scheme: dark)').matches))
