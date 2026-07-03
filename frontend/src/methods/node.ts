// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.
import { computed, type MaybeRefOrGetter, toValue } from 'vue'

import { Runtime } from '@/api/common/omni.pb'
import type { Resource } from '@/api/grpc'
import type { MachineStatusLinkSpec } from '@/api/omni/specs/ephemeral.pb'
import type { MachineStatusSpec } from '@/api/omni/specs/omni.pb'
import { DefaultNamespace, MachineStatusType } from '@/api/resources'
import { useResourceWatch } from '@/methods/useResourceWatch'

type MachineResource = Resource<MachineStatusSpec> | Resource<MachineStatusLinkSpec>

export function getMachineName(resource: MachineResource): string
export function getMachineName(resource: MachineResource | undefined): string | undefined
export function getMachineName(resource: MachineResource | undefined): string | undefined {
  const name = resource && (machineStatus(resource)?.network?.hostname || resource.metadata.id)

  return name ?? ''
}

export function useMachineName(
  machine: MaybeRefOrGetter<string | MachineResource | undefined>,
  options?: MaybeRefOrGetter<{ skip?: boolean }>,
) {
  const machineId = computed(() => {
    const value = toValue(machine)

    return typeof value === 'string' ? value : undefined
  })

  const { data } = useResourceWatch<MachineStatusSpec>(() => ({
    skip: toValue(options)?.skip || machineId.value === undefined,
    resource: {
      namespace: DefaultNamespace,
      type: MachineStatusType,
      id: machineId.value ?? '',
    },
    runtime: Runtime.Omni,
  }))

  return computed(() => {
    const value = toValue(machine)

    if (typeof value === 'string') {
      return getMachineName(data.value) || value
    }

    return getMachineName(value) ?? ''
  })
}

function machineStatus(resource: MachineResource): MachineStatusSpec | undefined {
  const spec = resource.spec

  return 'message_status' in spec ? spec.message_status : (spec as MachineStatusSpec)
}
