// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.
import { type MaybeRefOrGetter, ref, toValue, watchEffect } from 'vue'

import { b64Decode } from '@/api/fetch.pb'
import { ManagementService, type ReadAuditLogRequest } from '@/api/omni/management/management.pb'
import { withAbortController } from '@/api/options'

interface UseReadAuditLogOptions {
  options: ReadAuditLogRequest
}

export function useReadAuditLog(opts: MaybeRefOrGetter<UseReadAuditLogOptions>) {
  const data = ref<Uint8Array<ArrayBuffer>[]>([])
  const loading = ref(false)
  const error = ref<Error>()
  const downloadedBytes = ref(0)

  watchEffect(async (onCleanup) => {
    const abortController = new AbortController()
    onCleanup(() => abortController.abort())

    await loadData(abortController)
  })

  return { data, loading, error, downloadedBytes }

  async function loadData(abortController: AbortController) {
    try {
      const { options } = toValue(opts)

      loading.value = true
      error.value = undefined
      downloadedBytes.value = 0

      const result: Uint8Array<ArrayBuffer>[] = []

      await ManagementService.ReadAuditLog(
        options,
        (resp) => {
          if (!resp.audit_log) return

          const data = resp.audit_log.toString()
          const chunk = b64Decode(data) as Uint8Array<ArrayBuffer>

          downloadedBytes.value += chunk.byteLength
          result.push(chunk)
        },
        withAbortController(abortController),
      )

      data.value = result

      return result
    } catch (e) {
      if (abortController.signal.aborted) return

      error.value = e instanceof Error ? e : new Error(JSON.stringify(e))
    } finally {
      loading.value = false
    }
  }
}
