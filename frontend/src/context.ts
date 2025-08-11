// Copyright (c) 2025 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

import { ref } from 'vue'
import { useRoute } from 'vue-router'

import type { WatchContext } from '@/api/watch'

export const current: any = ref(localStorage.context ? JSON.parse(localStorage.context) : null)

export function changeContext(c: any) {
  localStorage.context = JSON.stringify(c)

  current.value = c
}

export function getContext(route: any = null): WatchContext {
  route = route || useRoute()

  const cluster = clusterName()

  const res: WatchContext = {
    cluster: cluster || '',
  }

  const machine = route.params.machine ?? route.query.machine
  if (machine) {
    res.nodes = [machine]
  }

  return res
}

export function clusterName(): string | null {
  const route = useRoute()

  if (route && route.params.cluster) {
    return route.params.cluster as string
  }

  if (route && route.query.cluster) {
    return route.query.cluster as string
  }

  return current.value ? current.value.name : null
}
