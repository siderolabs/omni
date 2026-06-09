// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.
import { computed, type MaybeRefOrGetter, toValue } from 'vue'

import type { MachineStatusSnapshotSpec } from '@/api/omni/specs/omni.pb'
import { MachineStatusSnapshotSpecPowerStage } from '@/api/omni/specs/omni.pb'
import { MachineStatusEventMachineStage } from '@/api/talos/machine/machine.pb'
import type { IconType } from '@/components/Icon/TIcon.vue'

interface StatusDescriptor {
  name: string
  icon: IconType
  class: string
}

const stageStatus: Partial<Record<MachineStatusEventMachineStage, StatusDescriptor>> = {
  [MachineStatusEventMachineStage.BOOTING]: {
    name: 'Booting',
    icon: 'loading',
    class: 'text-yellow-y1',
  },
  [MachineStatusEventMachineStage.INSTALLING]: {
    name: 'Installing',
    icon: 'loading',
    class: 'text-yellow-y1',
  },
  [MachineStatusEventMachineStage.MAINTENANCE]: {
    name: 'Maintenance',
    icon: 'settings',
    class: 'text-naturals-n11',
  },
  [MachineStatusEventMachineStage.RUNNING]: {
    name: 'Running',
    icon: 'check-in-circle',
    class: 'text-green-g1',
  },
  [MachineStatusEventMachineStage.REBOOTING]: {
    name: 'Rebooting',
    icon: 'loading',
    class: 'text-yellow-y1',
  },
  [MachineStatusEventMachineStage.SHUTTING_DOWN]: {
    name: 'Shutting Down',
    icon: 'loading',
    class: 'text-red-r1',
  },
  [MachineStatusEventMachineStage.RESETTING]: {
    name: 'Resetting',
    icon: 'loading',
    class: 'text-red-r1',
  },
  [MachineStatusEventMachineStage.UPGRADING]: {
    name: 'Upgrading',
    icon: 'loading',
    class: 'text-yellow-y1',
  },
}

const powerStageStatus: Partial<Record<MachineStatusSnapshotSpecPowerStage, StatusDescriptor>> = {
  [MachineStatusSnapshotSpecPowerStage.POWER_STAGE_POWERED_OFF]: {
    name: 'Powered Off',
    icon: 'power-off',
    class: 'text-red-r1',
  },
  [MachineStatusSnapshotSpecPowerStage.POWER_STAGE_POWERING_ON]: {
    name: 'Powering On',
    icon: 'loading',
    class: 'text-yellow-y1',
  },
}

export function useDerivedMachineStage(
  snapshot: MaybeRefOrGetter<MachineStatusSnapshotSpec | undefined>,
) {
  const stage = computed(() => toValue(snapshot)?.machine_status?.stage)
  const powerStage = computed(() => toValue(snapshot)?.power_stage)
  const installing = computed(() => stage.value === MachineStatusEventMachineStage.INSTALLING)
  const upgrading = computed(() => stage.value === MachineStatusEventMachineStage.UPGRADING)
  const status = computed(() => {
    const ps = powerStage.value
    if (ps !== undefined && powerStageStatus[ps]) {
      return powerStageStatus[ps]
    }

    const s = stage.value
    if (s === undefined) {
      return null
    }

    return stageStatus[s] ?? null
  })

  return { installing, upgrading, status }
}
