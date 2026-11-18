// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.
import { computed, type MaybeRefOrGetter, toValue } from 'vue'

import type { MachineStatusSnapshotSpec } from '@/api/omni/specs/omni.pb'
import { MachineStatusSnapshotSpecPowerStage } from '@/api/omni/specs/omni.pb'
import { MachineStatusEventMachineStage } from '@/api/talos/machine/machine.pb'
import type { StatusAppearance } from '@/components/Status/StatusPill.vue'

interface StatusDescriptor extends StatusAppearance {
  name: string
}

const stageStatus: Partial<Record<MachineStatusEventMachineStage, StatusDescriptor>> = {
  [MachineStatusEventMachineStage.BOOTING]: {
    name: 'Booting',
    tone: 'info',
    glyph: 'progress',
  },
  [MachineStatusEventMachineStage.INSTALLING]: {
    name: 'Installing',
    tone: 'info',
    glyph: 'progress',
  },
  [MachineStatusEventMachineStage.MAINTENANCE]: {
    name: 'Maintenance',
    tone: 'info',
    glyph: 'neutral',
  },
  [MachineStatusEventMachineStage.RUNNING]: {
    name: 'Running',
    tone: 'success',
    glyph: 'success',
  },
  [MachineStatusEventMachineStage.REBOOTING]: {
    name: 'Rebooting',
    tone: 'info',
    glyph: 'progress',
  },
  [MachineStatusEventMachineStage.SHUTTING_DOWN]: {
    name: 'Shutting Down',
    tone: 'info',
    glyph: 'progress',
  },
  [MachineStatusEventMachineStage.RESETTING]: {
    name: 'Resetting',
    tone: 'info',
    glyph: 'progress',
  },
  [MachineStatusEventMachineStage.UPGRADING]: {
    name: 'Upgrading',
    tone: 'info',
    glyph: 'progress',
  },
}

const powerStageStatus: Partial<Record<MachineStatusSnapshotSpecPowerStage, StatusDescriptor>> = {
  [MachineStatusSnapshotSpecPowerStage.POWER_STAGE_POWERED_OFF]: {
    name: 'Powered Off',
    tone: 'info',
    glyph: 'neutral',
  },
  [MachineStatusSnapshotSpecPowerStage.POWER_STAGE_POWERING_ON]: {
    name: 'Powering On',
    tone: 'info',
    glyph: 'progress',
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
