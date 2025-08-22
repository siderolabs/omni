// Copyright (c) 2025 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.
import { computed, type Ref, ref } from 'vue'

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
  opts: WatchOptionsSingle,
): WatchSingle<TSpec, TStatus>

export function useWatch<TSpec = unknown, TStatus = unknown>(
  opts: WatchOptionsMulti,
): WatchMulti<TSpec, TStatus>

export function useWatch<TSpec = unknown, TStatus = unknown>(
  opts: WatchJoinOptions[],
): WatchMulti<TSpec, TStatus>

export function useWatch<TSpec = unknown, TStatus = unknown>(
  opts: WatchOptions | WatchJoinOptions[],
): WatchSingle<TSpec, TStatus> | WatchMulti<TSpec, TStatus>

export function useWatch<TSpec, TStatus>(opts: WatchOptions | WatchJoinOptions[]) {
  if (Array.isArray(opts)) return useWatchJoin<TSpec, TStatus>(opts)

  return isWatchOptionsSingle(opts)
    ? useWatchSingle<TSpec, TStatus>(opts)
    : useWatchMulti<TSpec, TStatus>(opts)
}

function useWatchSingle<TSpec = unknown, TStatus = unknown>(opts: WatchOptionsSingle) {
  const data = ref<Resource<TSpec, TStatus>>()

  const watch = new Watch(data)
  watch.setup(computed(() => opts))

  return {
    data,
    err: watch.err,
    loading: watch.loading,
  }
}

function useWatchMulti<TSpec = unknown, TStatus = unknown>(opts: WatchOptionsMulti) {
  const data = ref<Resource<TSpec, TStatus>[]>([])

  const watch = new Watch(data)
  watch.setup(computed(() => opts))

  return {
    data,
    err: watch.err,
    loading: watch.loading,
  }
}

function useWatchJoin<TSpec = unknown, TStatus = unknown>(opts: WatchJoinOptions[]) {
  const data = ref<Resource<TSpec, TStatus>[]>([])

  const watch = new WatchJoin(data)
  watch.setup(
    computed(() => opts[0]),
    computed(() => opts.slice(1, opts.length)),
  )

  return {
    data,
    err: watch.err,
    loading: watch.loading,
  }
}

function isWatchOptionsSingle(opts: WatchOptions): opts is WatchOptionsSingle {
  return 'id' in opts.resource
}
