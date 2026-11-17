// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.
import { render, screen } from '@testing-library/vue'
import { afterEach, beforeEach, expect, test, vi } from 'vitest'
import { nextTick } from 'vue'

import { MachineOperationStatusSpecStage } from '@/api/omni/specs/omni.pb'

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
        stage: MachineOperationStatusSpecStage.DRAINING,
        stage_started_at: '2026-10-29T10:00:00Z',
      },
    },
  })

  expect(screen.getByText('Started 5 seconds ago')).toBeInTheDocument()

  await vi.advanceTimersByTimeAsync(3000)
  await nextTick()

  expect(screen.getByText('Started 8 seconds ago')).toBeInTheDocument()
})

test('the last message is shown below the elapsed time', () => {
  render(ClusterMachineOperationDescription, {
    props: {
      operation: {
        stage: MachineOperationStatusSpecStage.PULLING_IMAGE,
        stage_started_at: '2026-10-29T10:00:00Z',
        message: '[omni] pulling installer image factory.talos.dev/metal-installer/abc:v1.14.1',
      },
    },
  })

  expect(
    screen.getByText(
      '[omni] pulling installer image factory.talos.dev/metal-installer/abc:v1.14.1',
    ),
  ).toBeInTheDocument()
})
