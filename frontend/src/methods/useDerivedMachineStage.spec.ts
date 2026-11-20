// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.
import { expect, test } from 'vitest'

import type { MachineStatusLinkSpec } from '@/api/omni/specs/ephemeral.pb'
import type { MachineOperationStatusSpec } from '@/api/omni/specs/omni.pb'
import {
  MachineOperationStatusSpecStage,
  MachineStatusSnapshotSpecPowerStage,
} from '@/api/omni/specs/omni.pb'
import { MachineStatusEventMachineStage } from '@/api/talos/machine/machine.pb'
import { useDerivedMachineStage } from '@/methods/useDerivedMachineStage'

const running = { machine_status: { stage: MachineStatusEventMachineStage.RUNNING } }
const draining = { stage: MachineOperationStatusSpecStage.DRAINING }

test.each<[string, MachineStatusLinkSpec, MachineOperationStatusSpec | undefined, string, boolean]>(
  [
    ['the Talos stage', { snapshot: running }, undefined, 'Running', false],
    ['the operation over the Talos stage', { snapshot: running }, draining, 'Draining', true],
    [
      'the power stage over the operation',
      {
        snapshot: {
          ...running,
          power_stage: MachineStatusSnapshotSpecPowerStage.POWER_STAGE_POWERED_OFF,
        },
      },
      draining,
      'Powered Off',
      true,
    ],
    [
      'a config try without blocking the maintenance actions',
      { snapshot: { machine_status: { stage: MachineStatusEventMachineStage.MAINTENANCE } } },
      { stage: MachineOperationStatusSpecStage.TRYING_CONFIG },
      'Trying Config',
      false,
    ],
    [
      'an install which Talos reports',
      { snapshot: { machine_status: { stage: MachineStatusEventMachineStage.INSTALLING } } },
      undefined,
      'Installing',
      true,
    ],
  ],
)('shows %s', (_, spec, operation, name, inProgress) => {
  const { status, operationInProgress } = useDerivedMachineStage(spec, operation)

  expect(status.value?.name).toBe(name)
  expect(operationInProgress.value).toBe(inProgress)
})
