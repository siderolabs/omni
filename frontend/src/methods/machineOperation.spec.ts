// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.
import { describe, expect, test } from 'vitest'

import {
  ClusterMachineStatusSpecStage,
  MachineOperationStatusSpecStage,
} from '@/api/omni/specs/omni.pb'
import {
  clusterMachineShowsOperation,
  machineOperationDescription,
  machineOperationProgress,
  machineOperationStageName,
} from '@/methods/machineOperation'

describe('machineOperationStageName', () => {
  test.each([
    [undefined, ''],
    [{}, ''],
    [{ stage: MachineOperationStatusSpecStage.UNKNOWN }, ''],
    [{ stage: MachineOperationStatusSpecStage.PREPARING_IMAGE }, 'Preparing Image'],
    [{ stage: MachineOperationStatusSpecStage.PULLING_IMAGE }, 'Pulling Image'],
    [{ stage: MachineOperationStatusSpecStage.DRAINING }, 'Draining'],
    [{ stage: MachineOperationStatusSpecStage.UNCORDONING }, 'Uncordoning'],
  ])('%j is shown as %j', (operation, expected) => {
    expect(machineOperationStageName(operation)).toBe(expected)
  })
})

describe('machineOperationDescription', () => {
  const now = new Date('2026-10-29T10:00:05Z')

  test.each([
    [undefined, ''],
    [{ stage: MachineOperationStatusSpecStage.DRAINING }, ''],
    [{ stage_started_at: '2026-10-29T10:00:00Z' }, 'Started 5 seconds ago'],
  ])('%j is described as %j', (operation, expected) => {
    expect(machineOperationDescription(operation, now)).toBe(expected)
  })
})

describe('machineOperationProgress', () => {
  test.each([
    [undefined, ''],
    [{ stage: MachineOperationStatusSpecStage.DRAINING }, 'Draining'],
    [
      {
        stage: MachineOperationStatusSpecStage.PULLING_IMAGE,
        message: '[omni] pulling installer image',
      },
      '[omni] pulling installer image',
    ],
  ])('%j shows the progress %j', (operation, expected) => {
    expect(machineOperationProgress(operation)).toBe(expected)
  })
})

describe('clusterMachineShowsOperation', () => {
  test.each([
    [undefined, true],
    [ClusterMachineStatusSpecStage.RUNNING, true],
    [ClusterMachineStatusSpecStage.UPGRADING, true],
    [ClusterMachineStatusSpecStage.SHUTTING_DOWN, false],
    [ClusterMachineStatusSpecStage.BEFORE_DESTROY, false],
    [ClusterMachineStatusSpecStage.DESTROYING, false],
    [ClusterMachineStatusSpecStage.POWERING_ON, false],
    [ClusterMachineStatusSpecStage.POWERED_OFF, false],
  ])('a machine in stage %j shows its operation: %j', (stage, expected) => {
    expect(clusterMachineShowsOperation(stage)).toBe(expected)
  })
})
