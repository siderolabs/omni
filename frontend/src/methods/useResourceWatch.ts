// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.
import {
  computed,
  type MaybeRefOrGetter,
  onWatcherCleanup,
  type Ref,
  ref,
  toValue,
  watch,
} from 'vue'

import { Runtime } from '@/api/common/omni.pb'
import { RequestError } from '@/api/fetch.pb'
import type { Code } from '@/api/google/rpc/code.pb'
import { type Resource, ResourceService } from '@/api/grpc'
import { EventType, type WatchResponse } from '@/api/omni/resources/resources.pb'
import type { RuntimeContext } from '@/api/options'
import { type GRPCMetadata, withContext, withMetadata, withRuntime } from '@/api/options'
import type { Metadata } from '@/api/v1alpha1/resource.pb'
import { itemID } from '@/api/watch'

type WatchOptionsBase = {
  selectors?: string[]
  selectUsingOR?: boolean
  tailEvents?: number
  offset?: number
  limit?: number
  sortByField?: string
  sortDescending?: boolean
  searchFor?: string[]
  /**
   * Disables watch while true
   */
  skip?: boolean
} & (
  | { runtime: Runtime.Omni; context?: never }
  | { runtime: Exclude<Runtime, Runtime.Omni>; context: RuntimeContext }
)

export type WatchOptionsSingle = WatchOptionsBase & {
  resource: Metadata & { id: string }
}

export type WatchOptionsMulti = WatchOptionsBase & {
  resource: Omit<Metadata, 'id'>
}

export type WatchOptions = WatchOptionsSingle | WatchOptionsMulti

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
  const data: Ref<ResourceSort<Resource<TSpec, TStatus>>[]> = ref([])
  const total = ref(0)
  const bootstrapped = ref(false)

  const { err, errCode, loading } = useWatchStream<Resource<TSpec, TStatus>>(opts, {
    onMessage(message, spec) {
      callback?.(message, spec)

      switch (message.event?.event_type) {
        case EventType.BOOTSTRAPPED:
          bootstrapped.value = true
          total.value = message.total ?? 0
          break
        case EventType.UPDATED:
        case EventType.CREATED:
          const item: ResourceSort<Resource<TSpec, TStatus>> = {
            ...spec.res!,
            sortFieldData: message.sort_field_data,
          }

          const itemIndex = data.value.findIndex((r) => itemID(r) === itemID(spec.old ?? item))
          const itemExists = itemIndex > -1
          const sortValueChanged = data.value[itemIndex]?.sortFieldData !== item.sortFieldData

          if (itemExists && !sortValueChanged) {
            // Updates that don't affect sorting
            data.value[itemIndex] = item
          } else {
            // Update affects sorting, remove first
            if (itemExists) data.value.splice(itemIndex, 1)

            const index = getInsertionIndex(data.value, item, message.sort_descending)

            // Insert into sorted position
            data.value.splice(index, 0, item)
          }

          total.value = message.total ?? 0

          break
        case EventType.DESTROYED:
          data.value = data.value.filter((r) => itemID(r) !== itemID(spec.res!))
          total.value = message.total ?? 0

          break
      }
    },
    onStart() {
      bootstrapped.value = false
      total.value = 0
      data.value = []
    },
    onStop() {
      total.value = 0
      data.value = []
    },
  })

  return {
    data: computed(() => (bootstrapped.value ? data.value : [])),
    total: computed(() => (bootstrapped.value ? total.value : 0)),
    err,
    errCode,
    loading,
  }
}

type ResourceSort<T> = T & { sortFieldData?: string }

function compareFn<T extends Resource>(
  left: ResourceSort<T>,
  right: ResourceSort<T>,
  sortDescending?: boolean,
) {
  const inv = sortDescending ? -1 : 1

  if (left.sortFieldData && right.sortFieldData) {
    if (left.sortFieldData > right.sortFieldData) return 1 * inv
    if (left.sortFieldData < right.sortFieldData) return -1 * inv
  }

  const leftID = itemID(left)
  const rightID = itemID(right)

  if (leftID > rightID) return 1
  if (leftID < rightID) return -1

  return 0
}

function getInsertionIndex<T extends Resource>(
  arr: ResourceSort<T>[],
  item: ResourceSort<T>,
  sortDescending?: boolean,
): number {
  const itemsCount = arr.length
  if (!itemsCount) return 0

  const lastItem = arr[itemsCount - 1]

  if (compareFn(item, lastItem, sortDescending) >= 0) {
    return itemsCount
  }

  const getMidPoint = (start: number, end: number) => Math.floor((end - start) / 2) + start
  let start = 0
  let end = itemsCount - 1
  let index = getMidPoint(start, end)

  while (start < end) {
    const curItem = arr[index]

    const comparison = compareFn(item, curItem, sortDescending)

    if (comparison === 0) {
      break
    } else if (comparison < 0) {
      end = index
    } else {
      start = index + 1
    }
    index = getMidPoint(start, end)
  }

  return index
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
