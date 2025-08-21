// Copyright (c) 2025 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.
import { computed, ref } from 'vue'

import type { Resource } from '@/api/grpc'
import type { WatchJoinOptions, WatchOptions } from '@/api/watch'
import Watch, { WatchJoin } from '@/api/watch'

export function useWatch<TSpec = unknown, TStatus = unknown>(
  opts: WatchJoinOptions[] | WatchOptions,
) {
  const items = ref<Resource<TSpec, TStatus>[]>([])

  let watch: Watch<Resource> | WatchJoin<Resource>

  if (Array.isArray(opts)) {
    watch = new WatchJoin(items)
    watch.setup(
      computed(() => opts[0]),
      computed(() => opts.slice(1, opts.length)),
    )
  } else {
    watch = new Watch(items)
    watch.setup(computed(() => opts))
  }

  return {
    items,
    err: watch.err,
    loading: watch.loading,
  }
}
