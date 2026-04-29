// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.
import { computedAsync } from '@vueuse/core'
import { type MaybeRefOrGetter, ref, toValue } from 'vue'

import { DefaultTalosVersion } from '@/api/resources'

export interface TalosctlDownloadsResponse {
  status: string
  downloads?: string[]
}

interface Options {
  skip?: boolean
}

export function useTalosctlDownloads(
  talosVersionMaybeRef?: MaybeRefOrGetter<string | undefined>,
  options?: MaybeRefOrGetter<Options>,
) {
  const loading = ref(false)
  const err = ref<Error>()

  const data = computedAsync(
    async () => {
      try {
        if (toValue(options)?.skip) return []

        const talosVersion = toValue(talosVersionMaybeRef) ?? DefaultTalosVersion

        const response = await fetch(`/talosctl/downloads/${talosVersion}`)

        const { downloads }: TalosctlDownloadsResponse = await response.json()

        return downloads ?? []
      } catch (e) {
        err.value = e instanceof Error ? e : new Error(String(e))

        return []
      }
    },
    [],
    loading,
  )

  return { data, loading, err }
}
