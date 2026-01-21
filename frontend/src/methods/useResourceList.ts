// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.
import { type MaybeRefOrGetter, ref, toValue, watchEffect } from 'vue'

import { Runtime } from '@/api/common/omni.pb'
import type { fetchOption } from '@/api/fetch.pb'
import { type Resource, ResourceService } from '@/api/grpc'
import type { ListRequest } from '@/api/omni/resources/resources.pb'
import { withAbortController, withContext, withRuntime, withSelectors } from '@/api/options'
import type { WatchContext } from '@/api/watch'

export interface ListOptions {
  resource: ListRequest
  runtime: Runtime
  context?: WatchContext
  selectors?: string[]
  skip?: boolean
}

export function useResourceList<TSpec = unknown, TStatus = unknown>(
  opts: MaybeRefOrGetter<ListOptions>,
) {
  const data = ref<Resource<TSpec, TStatus>[]>()
  const loading = ref(true)
  const error = ref<Error>()

  watchEffect(async () => {
    const options = toValue(opts)

    if (options.skip) {
      loading.value = false
      return
    }

    await loadData()
  })

  return { data, loading, error, loadData }

  async function loadData(abortController?: AbortController) {
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

    if (abortController) {
      fetchOptions.push(withAbortController(abortController))
    }

    try {
      data.value = await ResourceService.List<Resource<TSpec, TStatus>>(
        options.resource,
        ...fetchOptions,
      )
    } catch (e) {
      error.value = e instanceof Error ? e : new Error(JSON.stringify(e))
    } finally {
      loading.value = false
    }

    return data.value
  }
}
