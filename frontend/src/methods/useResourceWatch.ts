// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.
import { type MaybeRefOrGetter, onWatcherCleanup, type Ref, ref, toValue, watch } from 'vue'

import { RequestError } from '@/api/fetch.pb'
import type { Code } from '@/api/google/rpc/code.pb'
import { type Resource, ResourceService } from '@/api/grpc'
import { EventType, type WatchResponse } from '@/api/omni/resources/resources.pb'
import { type GRPCMetadata, withContext, withMetadata, withRuntime } from '@/api/options'
import type { WatchOptions, WatchOptionsMulti, WatchOptionsSingle } from '@/api/watch'
import { itemID, WatchItems } from '@/api/watch'

interface WatchBase {
  err: Ref<string | null>
  errCode: Ref<Code | null>
  loading: Ref<boolean>
}

interface WatchSingle<TSpec, TStatus> extends WatchBase {
  data: Ref<Resource<TSpec, TStatus> | undefined>
}

interface WatchMulti<TSpec, TStatus> extends WatchBase {
  data: Ref<Resource<TSpec, TStatus>[]>
  total: Ref<number>
}

export function useResourceWatch<TSpec = unknown, TStatus = unknown>(
  opts: MaybeRefOrGetter<WatchOptionsSingle>,
  callback?: Callback<Resource<TSpec, TStatus>>,
): WatchSingle<TSpec, TStatus>

export function useResourceWatch<TSpec = unknown, TStatus = unknown>(
  opts: MaybeRefOrGetter<WatchOptionsMulti>,
  callback?: Callback<Resource<TSpec, TStatus>>,
): WatchMulti<TSpec, TStatus>

export function useResourceWatch<TSpec, TStatus>(
  opts: MaybeRefOrGetter<WatchOptions>,
  callback?: Callback<Resource<TSpec, TStatus>>,
) {
  // Type guards unfortunately don't narrow generic types
  return isWatchOptionsSingle(opts as MaybeRefOrGetter<WatchOptions>)
    ? useWatchSingle<TSpec, TStatus>(opts as MaybeRefOrGetter<WatchOptionsSingle>, callback)
    : useWatchMulti<TSpec, TStatus>(opts as MaybeRefOrGetter<WatchOptionsMulti>, callback)
}

function useWatchSingle<TSpec = unknown, TStatus = unknown>(
  opts: MaybeRefOrGetter<WatchOptionsSingle>,
  callback?: Callback<Resource<TSpec, TStatus>>,
): WatchSingle<TSpec, TStatus> {
  const data = ref<Resource<TSpec, TStatus>>()

  const { err, errCode, loading } = useWatchStream<Resource<TSpec, TStatus>>(opts, {
    onMessage(message, spec) {
      callback?.(message, spec)

      switch (message.event?.event_type) {
        case EventType.UPDATED:
        case EventType.CREATED:
          data.value = spec.res
          break
        case EventType.DESTROYED:
          data.value = undefined
          break
      }
    },
    onStop() {
      data.value = undefined
    },
  })

  return {
    data,
    err,
    errCode,
    loading,
  }
}

function useWatchMulti<TSpec = unknown, TStatus = unknown>(
  opts: MaybeRefOrGetter<WatchOptionsMulti>,
  callback?: Callback<Resource<TSpec, TStatus>>,
): WatchMulti<TSpec, TStatus> {
  const data: Ref<Resource<TSpec, TStatus>[], Resource<TSpec, TStatus>[]> = ref([])
  const total = ref(0)

  let watchItems: WatchItems<Resource<TSpec, TStatus>> | undefined

  const { err, errCode, loading } = useWatchStream<Resource<TSpec, TStatus>>(opts, {
    onMessage(message, spec) {
      callback?.(message, spec)

      switch (message.event?.event_type) {
        case EventType.BOOTSTRAPPED:
          watchItems?.bootstrap()

          // Bootstrap means that all initial items have arrived
          total.value = message.total ?? 0

          break
        case EventType.UPDATED:
        case EventType.CREATED:
          watchItems?.createOrUpdate(
            { ...spec.res!, sortFieldData: message.sort_field_data },
            message.sort_descending ?? false,
            spec.old,
          )

          // Wait for all initial items to arrive
          if (watchItems?.bootstrapped) {
            total.value = message.total ?? 0
          }

          break
        case EventType.DESTROYED:
          watchItems?.remove(itemID(spec.res!))

          // Wait for all initial items to arrive
          if (watchItems?.bootstrapped) {
            total.value = message.total ?? 0
          }

          break
      }
    },
    onStart() {
      watchItems = new WatchItems(data.value)
    },
    onStop() {
      watchItems?.reset()

      total.value = 0
      data.value.splice(0, data.value.length)
    },
  })

  return {
    data,
    err,
    errCode,
    loading,
    total,
  }
}

function isWatchOptionsSingle(
  opts: MaybeRefOrGetter<WatchOptions>,
): opts is MaybeRefOrGetter<WatchOptionsSingle> {
  return 'id' in toValue(opts).resource
}

interface Callback<T extends Resource> {
  (message: WatchResponse, spec: WatchEventSpec<T>): void
}

interface WatchEventSpec<T extends Resource> {
  res?: T
  old?: T
}

function useWatchStream<T extends Resource>(
  opts: MaybeRefOrGetter<WatchOptions>,
  {
    onMessage,
    onStart,
    onStop,
  }: {
    onMessage?: Callback<T>
    onStart?(): void
    onStop?(): void
  } = {},
) {
  const loading = ref(false)
  const err = ref<string | null>(null)
  const errCode = ref<Code | null>(null)

  watch(
    () => JSON.stringify(toValue(opts)),
    () => {
      const {
        skip,
        resource,
        runtime,
        context,
        selectors = [],
        selectUsingOR,
        searchFor,
        sortByField,
        sortDescending,
        limit,
        offset,
        tailEvents,
      } = toValue(opts)

      if (skip) {
        loading.value = false
        return
      }

      const metadata: GRPCMetadata = {}

      if (selectors.length) {
        metadata.selectors = selectors.join(selectUsingOR ? ';' : ',')
      }

      const fetchOptions = [withRuntime(runtime), withMetadata(metadata)]

      if (context) {
        fetchOptions.push(withContext(context))
      }

      const stream = ResourceService.Watch(
        {
          id: 'id' in resource ? resource.id : undefined,
          namespace: resource.namespace,
          type: resource.type,
          tail_events: tailEvents,
          limit,
          sort_by_field: sortByField,
          sort_descending: sortDescending,
          search_for: searchFor,
          offset,
        },
        onWatchMessage,
        fetchOptions,
        onWatchStart,
        onWatchError,
      )

      onWatcherCleanup(() => {
        stream.shutdown()
        onStop?.()
      })
    },
    { immediate: true },
  )

  function onWatchMessage(message: WatchResponse) {
    const spec: WatchEventSpec<T> = {}

    if (message.event) {
      const { resource, old, event_type } = message.event

      switch (event_type) {
        case EventType.BOOTSTRAPPED:
          loading.value = false
          break
        case EventType.CREATED:
        case EventType.UPDATED:
        case EventType.DESTROYED:
          if (!resource) {
            throw new Error(`malformed ${event_type} event: no resource defined`)
          }

          spec.res = JSON.parse(resource)

          if (old) spec.old = JSON.parse(old)
      }
    }

    onMessage?.(message, spec)
  }

  function onWatchStart() {
    err.value = null
    errCode.value = null
    loading.value = true

    onStart?.()
  }

  function onWatchError(error: Error) {
    err.value = error.message || String(error)
    errCode.value = error instanceof RequestError ? ((error.code as Code) ?? null) : null
    loading.value = false
  }

  return { loading, err, errCode }
}
