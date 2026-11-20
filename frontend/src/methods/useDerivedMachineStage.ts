// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.
import { computed, type MaybeRefOrGetter, toValue } from 'vue'

import type { MachineStatusLinkSpec } from '@/api/omni/specs/ephemeral.pb'
import type { MachineOperationStatusSpec } from '@/api/omni/specs/omni.pb'
import { MachineStatusSnapshotSpecPowerStage } from '@/api/omni/specs/omni.pb'
import { MachineStatusEventMachineStage } from '@/api/talos/machine/machine.pb'
import type { StatusAppearance } from '@/components/Status/StatusPill.vue'
import { machineOperationStageName } from '@/methods/machineOperation'

interface StatusDescriptor extends StatusAppearance {
  name: string
  operation?: MachineOperationStatusSpec
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
  spec: MaybeRefOrGetter<MachineStatusLinkSpec | undefined>,
  operation: MaybeRefOrGetter<MachineOperationStatusSpec | undefined>,
) {
  const stage = computed(() => toValue(spec)?.snapshot?.machine_status?.stage)
  const powerStage = computed(() => toValue(spec)?.snapshot?.power_stage)
  const operationName = computed(() => machineOperationStageName(toValue(operation)))
  const operationInProgress = computed(
    () =>
      !!operationName.value ||
      stage.value === MachineStatusEventMachineStage.INSTALLING ||
      stage.value === MachineStatusEventMachineStage.UPGRADING,
  )
  const status = computed((): StatusDescriptor | null => {
    const ps = powerStage.value
    if (ps !== undefined && powerStageStatus[ps]) {
      return powerStageStatus[ps]
    }

    if (operationName.value) {
      return {
        name: operationName.value,
        tone: 'warning',
        glyph: 'progress',
        operation: toValue(operation),
      }
    }

    const s = stage.value
    if (s === undefined) {
      return null
    }

    return stageStatus[s] ?? null
  })

  return { operationInProgress, status }
}
