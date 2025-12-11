// Copyright (c) 2025 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.
import { computedAsync } from '@vueuse/core'
import { compareLoose } from 'semver'
import { computed } from 'vue'

import { showError } from '@/notification'

interface TalosctlDownloadsResponse {
  status: string
  release_data: {
    /**
     * NOTE: We don't use this response value as it is not correct.
     * The backend pops the top of a sorted list, but it is sorted alphabetically,
     * which does not correctly sort versions. For example, this sorting:
     * - 1.1
     * - 1.11
     * - 1.2
     *
     * Giving 1.2 as a more recent version than 1.11.
     *
     * @deprecated Don't use this as it is not the latest version.
     */
    default_version: string
    available_versions: Record<string, { name: string; url: string }[]>
  }
}

export function useTalosctlDownloads() {
  const downloads = computedAsync(async () => {
    try {
      const response = await fetch('/talosctl/downloads')

      const {
        release_data: { available_versions },
      }: TalosctlDownloadsResponse = await response.json()

      return new Map(Object.entries(available_versions).sort(([a], [b]) => compareLoose(a, b)))
    } catch (e) {
      showError('Error getting latest talos releases', e?.message ?? String(e))
    }
  })

  const defaultVersion = computed(() => downloads.value && Array.from(downloads.value.keys()).pop())

  return { downloads, defaultVersion }
}
