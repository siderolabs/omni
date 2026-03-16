// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.
import { type MaybeRefOrGetter, ref, toValue, watchEffect } from 'vue'

import { Runtime } from '@/api/common/omni.pb'
import type { fetchOption } from '@/api/fetch.pb'
import { type Resource, ResourceService } from '@/api/grpc'
import type { GetRequest } from '@/api/omni/resources/resources.pb'
import { withAbortController, withContext, withRuntime, withSelectors } from '@/api/options'
import type { WatchContext } from '@/api/watch'

interface GetOptionsCommon {
  resource: GetRequest
  selectors?: string[]
  skip?: boolean
}

export type GetOptions = GetOptionsCommon &
  (
    | { runtime: Runtime.Omni; context?: never }
    | { runtime: Exclude<Runtime, Runtime.Omni>; context: WatchContext }
  )

export function useResourceGet<TSpec = unknown, TStatus = unknown>(
  opts: MaybeRefOrGetter<GetOptions>,
) {
  const data = ref<Resource<TSpec, TStatus>>()
  const loading = ref(true)
  const error = ref<Error>()

  watchEffect(async (onCleanup) => {
    const options = toValue(opts)

    if (options.skip) {
      loading.value = false
      return
    }

    const abortController = new AbortController()

    await loadData(abortController)

    onCleanup(() => abortController.abort())
  })

  return { data, loading, error, loadData }

  async function loadData(abortController: AbortController) {
    const options = toValue(opts)

    loading.value = true
    error.value = undefined

    const fetchOptions: fetchOption[] = []

    if (options.runtime) {
      fetchOptions.push(withRuntime(options.runtime))
    }

    if (options.selectors) {
      fetchOptions.push(withSelectors(options.selectors))
    }

    if (options.context) {
      fetchOptions.push(withContext(options.context))
    }

    try {
      const newData = await ResourceService.Get<Resource<TSpec, TStatus>>(
        options.resource,
        withAbortController(abortController),
        ...fetchOptions,
      )

      data.value = newData

      return newData
    } catch (e) {
      if (abortController.signal.aborted) return

      error.value = e instanceof Error ? e : new Error(JSON.stringify(e))
    } finally {
      loading.value = false
    }
  }
}
