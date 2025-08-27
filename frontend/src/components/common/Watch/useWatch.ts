// Copyright (c) 2025 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.
import { type MaybeRefOrGetter, type Ref, ref, toRef, toValue } from 'vue'

import type { Resource } from '@/api/grpc'
import type {
  WatchJoinOptions,
  WatchOptions,
  WatchOptionsMulti,
  WatchOptionsSingle,
} from '@/api/watch'
import Watch, { WatchJoin } from '@/api/watch'

interface WatchBase {
  err: Ref<string | null>
  loading: Ref<boolean>
}

interface WatchSingle<TSpec, TStatus> extends WatchBase {
  data: Ref<Resource<TSpec, TStatus> | undefined>
}

interface WatchMulti<TSpec, TStatus> extends WatchBase {
  data: Ref<Resource<TSpec, TStatus>[]>
}

export function useWatch<TSpec = unknown, TStatus = unknown>(
  opts: MaybeRefOrGetter<WatchOptionsSingle>,
): WatchSingle<TSpec, TStatus>

export function useWatch<TSpec = unknown, TStatus = unknown>(
  opts: MaybeRefOrGetter<WatchOptionsMulti>,
): WatchMulti<TSpec, TStatus>

export function useWatch<TSpec = unknown, TStatus = unknown>(
  opts: MaybeRefOrGetter<WatchJoinOptions[]>,
): WatchMulti<TSpec, TStatus>

export function useWatch<TSpec = unknown, TStatus = unknown>(
  opts: MaybeRefOrGetter<WatchOptions | WatchJoinOptions[]>,
): WatchSingle<TSpec, TStatus> | WatchMulti<TSpec, TStatus>

export function useWatch<TSpec, TStatus>(
  opts: MaybeRefOrGetter<WatchOptions | WatchJoinOptions[]>,
) {
  if (isWatchJoinOptions(opts)) return useWatchJoin<TSpec, TStatus>(opts)

  // Type guards unfortunately don't narrow generic types
  return isWatchOptionsSingle(opts as MaybeRefOrGetter<WatchOptions>)
    ? useWatchSingle<TSpec, TStatus>(opts as MaybeRefOrGetter<WatchOptionsSingle>)
    : useWatchMulti<TSpec, TStatus>(opts as MaybeRefOrGetter<WatchOptionsMulti>)
}

function useWatchSingle<TSpec = unknown, TStatus = unknown>(
  opts: MaybeRefOrGetter<WatchOptionsSingle>,
) {
  const data = ref<Resource<TSpec, TStatus>>()

  const watch = new Watch(data)
  watch.setup(toRef(opts))

  return {
    data,
    err: watch.err,
    loading: watch.loading,
  }
}

function useWatchMulti<TSpec = unknown, TStatus = unknown>(
  opts: MaybeRefOrGetter<WatchOptionsMulti>,
) {
  const data = ref<Resource<TSpec, TStatus>[]>([])

  const watch = new Watch(data)
  watch.setup(toRef(opts))

  return {
    data,
    err: watch.err,
    loading: watch.loading,
  }
}

function useWatchJoin<TSpec = unknown, TStatus = unknown>(
  opts: MaybeRefOrGetter<WatchJoinOptions[]>,
) {
  const data = ref<Resource<TSpec, TStatus>[]>([])

  const watch = new WatchJoin(data)
  watch.setup(
    toRef(() => toValue(opts)[0]),
    toRef(() => {
      const [, ...rest] = toValue(opts)
      return rest
    }),
  )

  return {
    data,
    err: watch.err,
    loading: watch.loading,
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
