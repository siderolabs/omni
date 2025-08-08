// Copyright (c) 2025 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

import { ref, watch } from 'vue'

const systemTheme = ref(
  window.matchMedia('(prefers-color-scheme: dark)').matches ? 'dark' : 'light',
)
const theme = ref(localStorage.theme || 'system')

const match = window.matchMedia('(prefers-color-scheme: dark)')

if (match.addEventListener) {
  match.addEventListener('change', (e) => {
    systemTheme.value = e.matches ? 'dark' : 'light'
  })
}

watch(theme, (v) => {
  localStorage.theme = v
})

export function isDark(mode: string): boolean {
  for (let i = 0; i < 2; i++) {
    switch (mode) {
      case 'system':
        mode = systemTheme.value
        break
      case 'dark':
        return true
      case 'light':
        return false
    }
  }

  return false
}

export { theme, systemTheme }
