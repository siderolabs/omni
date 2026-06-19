// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.
import { type MaybeRefOrGetter, onWatcherCleanup, type Ref, ref, toValue, watch } from 'vue'

import type { Code } from '@/api/google/rpc/code.pb'
import type { Resource } from '@/api/grpc'
import type { Callback, WatchOptions, WatchOptionsMulti, WatchOptionsSingle } from '@/api/watch'
import Watch from '@/api/watch'

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

  const watch = useWatch(data, opts, callback)

  return {
    data,
    err: watch.err,
    errCode: watch.errCode,
    loading: watch.loading,
  }
}

function useWatchMulti<TSpec = unknown, TStatus = unknown>(
  opts: MaybeRefOrGetter<WatchOptionsMulti>,
  callback?: Callback<Resource<TSpec, TStatus>>,
): WatchMulti<TSpec, TStatus> {
  const data: Ref<Resource<TSpec, TStatus>[], Resource<TSpec, TStatus>[]> = ref([])

  const watch = useWatch(data, opts, callback)

  return {
    data,
    err: watch.err,
    errCode: watch.errCode,
    loading: watch.loading,
    total: watch.total,
  }
}

function isWatchOptionsSingle(
  opts: MaybeRefOrGetter<WatchOptions>,
): opts is MaybeRefOrGetter<WatchOptionsSingle> {
  return 'id' in toValue(opts).resource
}

function useWatch<T extends Resource>(
  data: Ref<T[]> | Ref<T | undefined>,
  opts: MaybeRefOrGetter<WatchOptions>,
  callback?: Callback<T>,
) {
  const resWatch = new Watch(data, callback)

  watch(
    () => JSON.stringify(toValue(opts)),
    () => {
      const watchOptions = toValue(opts)

      if (watchOptions.skip) return

      resWatch.start(watchOptions)
      onWatcherCleanup(() => resWatch.stop())
    },
    { immediate: true },
  )

  return resWatch
}
