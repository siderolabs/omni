// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.
import { computed, type MaybeRefOrGetter, toValue } from 'vue'
import type { RouteLocationRaw } from 'vue-router'

import { Runtime } from '@/api/common/omni.pb'

export type ResourceRuntime = 'omni' | 'talos'

/** Where resources are read from. Talos resources live on each node, so a machine is required. */
export type ResourceTarget = { runtime: 'omni' } | { runtime: 'talos'; machine: string }

export function useResourceRuntime(target: MaybeRefOrGetter<ResourceTarget>) {
  // Targets are usually passed as inline objects, which are recreated on every render.
  // Everything below derives from this primitive so it only changes when the target does.
  const machine = computed(() => {
    const t = toValue(target)

    return t.runtime === 'talos' ? t.machine : undefined
  })

  const watchOptions = computed(() =>
    machine.value
      ? ({ runtime: Runtime.Talos, context: { node: machine.value } } as const)
      : ({ runtime: Runtime.Omni } as const),
  )

  /** The resource type list for this target. */
  const listRoute = computed<RouteLocationRaw>(() =>
    machine.value
      ? { name: 'InternalsTalosResources', params: { machine: machine.value } }
      : { name: 'InternalsOmniResources' },
  )

  /** The resources of a single type for this target. */
  function resourceRoute(type: string): RouteLocationRaw {
    const t = toValue(target)

    return t.runtime === 'talos'
      ? { name: 'InternalsTalosResource', params: { machine: t.machine, type } }
      : { name: 'InternalsOmniResource', params: { type } }
  }

  return { machine, watchOptions, listRoute, resourceRoute }
}
