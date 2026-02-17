// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.
import { useRoute } from 'vue-router'

import type { WatchContext } from '@/api/watch'

export function getContext(): WatchContext {
  const route = useRoute()

  const cluster = route.params.cluster?.toString() || route.query.cluster?.toString()
  const machine = route.params.machine?.toString() || route.query.machine?.toString()

  return {
    cluster,
    nodes: machine ? [machine] : undefined,
  }
}
