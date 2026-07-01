// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.
import { type MaybeRefOrGetter, onWatcherCleanup, type Ref, ref, toValue, watch } from 'vue'

import type { Code } from '@/api/google/rpc/code.pb'
import type { Resource } from '@/api/grpc'
import { EventType } from '@/api/omni/resources/resources.pb'
import type { Callback, WatchOptions, WatchOptionsMulti, WatchOptionsSingle } from '@/api/watch'
import Watch, { itemID, WatchItems } from '@/api/watch'

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
  const loading = ref(false)

  const watch = useWatch<Resource<TSpec, TStatus>>(loading, opts, {
    onMessage(message, spec) {
      callback?.(message, spec)

      const eventType = message.event?.event_type ?? EventType.UNKNOWN

      switch (eventType) {
        case EventType.BOOTSTRAPPED:
          loading.value = false
          break
        case EventType.UPDATED:
        case EventType.CREATED:
          if (!spec.res) {
            throw new Error(`malformed ${eventType} event: no resource defined`)
          }

          data.value = spec.res
          break
        case EventType.DESTROYED:
          if (!spec.res) {
            throw new Error(`malformed ${eventType} event: no resource defined`)
          }

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
    err: watch.err,
    errCode: watch.errCode,
    loading,
  }
}

function useWatchMulti<TSpec = unknown, TStatus = unknown>(
  opts: MaybeRefOrGetter<WatchOptionsMulti>,
  callback?: Callback<Resource<TSpec, TStatus>>,
): WatchMulti<TSpec, TStatus> {
  const data: Ref<Resource<TSpec, TStatus>[], Resource<TSpec, TStatus>[]> = ref([])
  const loading = ref(false)
  const total = ref(0)

  let watchItems: WatchItems<Resource<TSpec, TStatus>> | undefined

  const watch = useWatch<Resource<TSpec, TStatus>>(loading, opts, {
    onMessage(message, spec) {
      callback?.(message, spec)

      const eventType = message.event?.event_type ?? EventType.UNKNOWN

      switch (eventType) {
        case EventType.BOOTSTRAPPED:
          loading.value = false
          watchItems?.bootstrap()

          break
        case EventType.UPDATED:
        case EventType.CREATED:
          if (!spec.res) {
            throw new Error(`malformed ${eventType} event: no resource defined`)
          }

          watchItems?.createOrUpdate(
            { ...spec.res, sortFieldData: message.sort_field_data },
            message.sort_descending ?? false,
            spec.old,
          )

          break
        case EventType.DESTROYED:
          if (!spec.res) {
            throw new Error(`malformed ${eventType} event: no resource defined`)
          }

          watchItems?.remove(itemID(spec.res))

          break
      }

      if (
        message.total !== undefined ||
        ![EventType.BOOTSTRAPPED, EventType.UNKNOWN].includes(eventType)
      ) {
        if (watchItems?.bootstrapped) {
          total.value = message.total ?? 0
        }
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
    err: watch.err,
    errCode: watch.errCode,
    loading,
    total,
  }
}

function isWatchOptionsSingle(
  opts: MaybeRefOrGetter<WatchOptions>,
): opts is MaybeRefOrGetter<WatchOptionsSingle> {
  return 'id' in toValue(opts).resource
}

function useWatch<T extends Resource>(
  loading: Ref<boolean>,
  opts: MaybeRefOrGetter<WatchOptions>,
  {
    onMessage,
    onStart,
    onStop,
    onError,
  }: {
    onMessage?: Callback<T>
    onStart?(): void
    onStop?(): void
    onError?(e: Error): void
  } = {},
) {
  const resWatch = new Watch<T>(loading, onMessage, onStart, onError)

  watch(
    () => JSON.stringify(toValue(opts)),
    () => {
      const watchOptions = toValue(opts)

      if (watchOptions.skip) return

      resWatch.start(watchOptions)
      onWatcherCleanup(() => {
        resWatch.stop()
        onStop?.()
      })
    },
    { immediate: true },
  )

  return resWatch
}
