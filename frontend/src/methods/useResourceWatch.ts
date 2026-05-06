// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.
import { type MaybeRefOrGetter, type Ref, ref, toValue } from 'vue'

import type { Code } from '@/api/google/rpc/code.pb'
import type { Resource } from '@/api/grpc'
import type {
  Callback,
  WatchJoinOptions,
  WatchOptions,
  WatchOptionsMulti,
  WatchOptionsSingle,
} from '@/api/watch'
import Watch, { WatchJoin } from '@/api/watch'

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

interface WatchMultiJoin<TSpec, TStatus> extends WatchBase {
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

export function useResourceWatch<TSpec = unknown, TStatus = unknown>(
  opts: MaybeRefOrGetter<WatchJoinOptions[]>,
): WatchMultiJoin<TSpec, TStatus>

export function useResourceWatch<TSpec = unknown, TStatus = unknown>(
  opts: MaybeRefOrGetter<WatchOptions | WatchJoinOptions[]>,
  callback?: Callback<Resource<TSpec, TStatus>>,
): WatchSingle<TSpec, TStatus> | WatchMulti<TSpec, TStatus> | WatchMultiJoin<TSpec, TStatus>

export function useResourceWatch<TSpec, TStatus>(
  opts: MaybeRefOrGetter<WatchOptions | WatchJoinOptions[]>,
  callback?: Callback<Resource<TSpec, TStatus>>,
) {
  if (isWatchJoinOptions(opts)) return useWatchJoin<TSpec, TStatus>(opts)

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

  const watch = new Watch(data, callback)
  watch.setup(opts)

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

  const watch = new Watch(data, callback)
  watch.setup(opts)

  return {
    data,
    err: watch.err,
    errCode: watch.errCode,
    loading: watch.loading,
    total: watch.total,
  }
}

function useWatchJoin<TSpec = unknown, TStatus = unknown>(
  opts: MaybeRefOrGetter<WatchJoinOptions[]>,
): WatchMultiJoin<TSpec, TStatus> {
  const data: Ref<Resource<TSpec, TStatus>[], Resource<TSpec, TStatus>[]> = ref([])

  const watch = new WatchJoin(data)
  watch.setup(
    () => toValue(opts)[0],
    () => {
      const [, ...rest] = toValue(opts)
      return rest
    },
  )

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

function isWatchJoinOptions(
  opts: MaybeRefOrGetter<WatchOptions | WatchJoinOptions[]>,
): opts is MaybeRefOrGetter<WatchJoinOptions[]> {
  return Array.isArray(toValue(opts))
}
