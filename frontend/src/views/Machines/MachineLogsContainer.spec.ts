// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.
import { render, screen } from '@testing-library/vue'
import { beforeEach, expect, test, vi } from 'vitest'
import { ref } from 'vue'

const streamState = vi.hoisted(() => ({
  logs: [] as { msg: string }[],
}))

vi.mock('@vueuse/router', () => ({ useRouteQuery: () => ref('') }))

vi.mock('@/methods/useResourceWatch', () => ({
  useResourceWatch: () => ({ data: ref(undefined) }),
}))

vi.mock('@/methods/useMachineServices', () => ({
  supportsMaintenanceEvents: () => false,
  useMachineServices: () => ({ data: ref([]) }),
}))

vi.mock('@/methods/logs', async (importOriginal) => {
  const actual = await importOriginal<typeof import('@/methods/logs')>()

  return {
    ...actual,
    setupLogStream: (logs: { value: { msg: string }[] }) => {
      logs.value = [...streamState.logs]

      return {
        stream: { err: ref(new Error('machine is unreachable')) },
        shutdown: vi.fn(),
      }
    },
  }
})

import MachineLogsContainer from './MachineLogsContainer.vue'

beforeEach(() => {
  streamState.logs = []
})

function renderMachineLogs() {
  return render(MachineLogsContainer, {
    props: { machineId: 'offline-machine', service: 'controller-runtime' },
    global: {
      stubs: ['TSelectList', 'TInput', 'LogViewer'],
    },
  })
}

test('labels an initial log fetch error as a fetch failure', async () => {
  renderMachineLogs()

  expect(await screen.findByText('Failed to Fetch Logs')).toBeInTheDocument()
  expect(screen.queryByText('Disconnected')).not.toBeInTheDocument()
})

test('labels an error after receiving logs as a disconnect', async () => {
  streamState.logs = [{ msg: 'existing log' }]

  renderMachineLogs()

  expect(await screen.findByText('Disconnected')).toBeInTheDocument()
  expect(screen.queryByText('Failed to Fetch Logs')).not.toBeInTheDocument()
})
