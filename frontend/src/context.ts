// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

import { useLocalStorage } from '@vueuse/core'
import { type RouteLocationNormalizedLoadedGeneric, useRoute } from 'vue-router'

import type { WatchContext } from '@/api/watch'

export const current = useLocalStorage<string>('context', null)

export function getContext(route?: RouteLocationNormalizedLoadedGeneric): WatchContext {
  route ||= useRoute()

  const cluster = clusterName()

  const res: WatchContext = {
    cluster: cluster || undefined,
  }

  const machine = (route.params.machine ?? route.query.machine) as string
  if (machine) {
    res.nodes = [machine]
  }

  return res
}

export function clusterName() {
  const route = useRoute()

  if ('cluster' in route?.params) {
    return route.params.cluster
  }

  if (route?.query.cluster) {
    return route.query.cluster as string
  }

  return current.value || null
}
