// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.
import { computed, type MaybeRefOrGetter, toValue } from 'vue'

import { Runtime } from '@/api/common/omni.pb'
import type { MachineStatusSpec } from '@/api/omni/specs/omni.pb'
import { DefaultNamespace, MachineStatusType } from '@/api/resources'
import { useResourceWatch } from '@/methods/useResourceWatch'

export interface MachineOption {
  id: string
  name: string
  /** The machine's cluster, or "maintenance" for unallocated machines in maintenance mode. */
  location?: string
  connected: boolean
}

/** Machines whose Talos resources can be browsed, sorted by name. */
export function useMachineOptions(skip?: MaybeRefOrGetter<boolean>) {
  const { data, loading } = useResourceWatch<MachineStatusSpec>(() => ({
    skip: toValue(skip),
    runtime: Runtime.Omni,
    resource: {
      namespace: DefaultNamespace,
      type: MachineStatusType,
    },
  }))

  const machines = computed<MachineOption[]>(() =>
    data.value
      .map(({ metadata, spec }) => ({
        id: metadata.id!,
        name: spec.network?.hostname || metadata.id!,
        location: spec.cluster || (spec.maintenance ? 'maintenance' : undefined),
        connected: !!spec.connected,
      }))
      .sort((a, b) => a.name.localeCompare(b.name)),
  )

  return { machines, loading }
}
