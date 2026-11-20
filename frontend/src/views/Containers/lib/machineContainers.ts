// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.
import { createInjectionState } from '@vueuse/core'
import { computed, type MaybeRefOrGetter, toValue } from 'vue'

import { Runtime } from '@/api/common/omni.pb'
import { TalosContainersNamespace, TalosContainerStatusType } from '@/api/resources'
import type { ContainerStatusSpec } from '@/api/talos/containers.pb'
import { supportsContainers } from '@/methods/containers'
import { useResourceWatch } from '@/methods/useResourceWatch'

/**
 * Watches a machine's container statuses for its pages: the page layout decides from them whether
 * the Containers tab shows, and the tab reuses the same watch. Only Talos 1.15+ has containers.
 */
const [provideMachineContainers, useMachineContainers] = createInjectionState(
  (machineId: MaybeRefOrGetter<string>, talosVersion: MaybeRefOrGetter<string | undefined>) => {
    const supported = computed(() => supportsContainers(toValue(talosVersion)))

    const watch = useResourceWatch<ContainerStatusSpec>(() => ({
      runtime: Runtime.Talos,
      resource: {
        namespace: TalosContainersNamespace,
        type: TalosContainerStatusType,
      },
      context: { node: toValue(machineId) },
      skip: !supported.value,
    }))

    return { ...watch, supported }
  },
)

export { provideMachineContainers }

export function injectMachineContainers() {
  const containers = useMachineContainers()

  if (!containers) throw new Error('injectMachineContainers called outside a machine page')

  return containers
}
