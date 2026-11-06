// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.
import { render, screen } from '@testing-library/vue'
import { afterEach, beforeEach, expect, test, vi } from 'vitest'
import { nextTick } from 'vue'

import { MachineOperationPhase } from '@/api/omni/specs/omni.pb'

import ClusterMachineOperationDescription from './ClusterMachineOperationDescription.vue'

beforeEach(() => {
  vi.useFakeTimers()
  vi.setSystemTime(new Date('2026-10-29T10:00:05Z'))
})

afterEach(() => {
  vi.useRealTimers()
})

test('the elapsed time updates while it is shown', async () => {
  render(ClusterMachineOperationDescription, {
    props: {
      operation: {
        phase: MachineOperationPhase.DRAINING,
        message: 'draining node n1',
        phase_started_at: '2026-10-29T10:00:00Z',
      },
    },
  })

  expect(screen.getByText('draining node n1, started 5 seconds ago')).toBeInTheDocument()

  await vi.advanceTimersByTimeAsync(3000)
  await nextTick()

  expect(screen.getByText('draining node n1, started 8 seconds ago')).toBeInTheDocument()
})

test('a config try shows the attempt', () => {
  render(ClusterMachineOperationDescription, {
    props: {
      operation: {
        phase: MachineOperationPhase.TRYING_CONFIG,
        config_try_attempt: 2,
      },
    },
  })

  expect(screen.getByText('Attempt 2')).toBeInTheDocument()
})
