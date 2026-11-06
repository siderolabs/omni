// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.
import { describe, expect, test } from 'vitest'

import { MachineOperationPhase } from '@/api/omni/specs/omni.pb'
import {
  machineOperationError,
  machineOperationErrorDescription,
  machineOperationErrorTitle,
  machineOperationPhaseName,
} from '@/methods/machineOperation'

describe('machineOperationPhaseName', () => {
  test.each([
    [MachineOperationPhase.NONE, ''],
    [undefined, ''],
    [MachineOperationPhase.PULLING_IMAGE, 'Pulling Image'],
    [MachineOperationPhase.INSTALLING, 'Installing'],
    [MachineOperationPhase.UPGRADING, 'Upgrading'],
    [MachineOperationPhase.PREPARING_REBOOT, 'Preparing Reboot'],
    [MachineOperationPhase.CORDONING, 'Cordoning'],
    [MachineOperationPhase.DRAINING, 'Draining'],
    [MachineOperationPhase.REBOOTING, 'Rebooting'],
    [MachineOperationPhase.FINALIZING, 'Finalizing'],
    [MachineOperationPhase.TRYING_CONFIG, 'Trying Config'],
    [MachineOperationPhase.UNCORDONING, 'Uncordoning'],
    [MachineOperationPhase.PREPARING_IMAGE, 'Preparing Image'],
  ])('%s is shown as %j', (phase, expected) => {
    expect(machineOperationPhaseName(phase)).toBe(expected)
  })
})

describe('machine operation error', () => {
  test('with a config error from a failed phase', () => {
    const configError = 'upgrade failed: failed to pull installer image: timeout'
    const operation = { failed_phase: MachineOperationPhase.PULLING_IMAGE }

    expect(machineOperationErrorTitle(operation)).toBe('Pulling Image Failed')
    expect(machineOperationError(configError, operation)).toBe(configError)
    expect(machineOperationErrorDescription(configError, operation)).toBe(
      'Pulling image failed: upgrade failed: failed to pull installer image: timeout',
    )
  })

  test('with an uncordon failure and no config error', () => {
    const operation = {
      last_error: 'failed to uncordon node: node unreachable',
      failed_phase: MachineOperationPhase.UNCORDONING,
    }

    expect(machineOperationError('', operation)).toBe('failed to uncordon node: node unreachable')
    expect(machineOperationErrorDescription('', operation)).toBe(
      'Uncordoning failed: failed to uncordon node: node unreachable',
    )
  })

  test('with an uncordon failure and a config error', () => {
    const operation = {
      last_error: 'failed to uncordon node: node unreachable',
      failed_phase: MachineOperationPhase.UNCORDONING,
    }

    expect(machineOperationError('failed to generate config: invalid patch', operation)).toBe(
      'failed to uncordon node: node unreachable\nConfiguration error: failed to generate config: invalid patch',
    )
    expect(
      machineOperationErrorDescription('failed to generate config: invalid patch', operation),
    ).toBe(
      'Uncordoning failed: failed to uncordon node: node unreachable\nConfiguration error: failed to generate config: invalid patch',
    )
  })

  test('without any error', () => {
    expect(machineOperationError(undefined, undefined)).toBe('')
  })

  test('without a failed phase', () => {
    expect(machineOperationErrorTitle(undefined)).toBe('Configuration Error')
    expect(machineOperationErrorDescription('config apply failed \n', undefined)).toBe(
      'Configuration error: config apply failed',
    )
  })
})
