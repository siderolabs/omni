// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.
import { render, screen } from '@testing-library/vue'
import { beforeEach, expect, test, vi } from 'vitest'
import { ref } from 'vue'

import type { Resource } from '@/api/grpc'
import type { ContainerStatusSpec } from '@/api/talos/containers.pb'

const streamState = vi.hoisted(() => ({
  logs: [] as { msg: string }[],
  opened: 0,
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
      streamState.opened++
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
  streamState.opened = 0
})

function renderMachineLogs(
  service = 'controller-runtime',
  containers: Resource<ContainerStatusSpec>[] = [],
) {
  return render(MachineLogsContainer, {
    props: { machineId: 'offline-machine', service, containers },
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

const container = (spec: ContainerStatusSpec): Resource<ContainerStatusSpec> => ({
  metadata: { id: 'web' },
  spec,
})

test('waits for a Talos container that has not started yet, without streaming', async () => {
  renderMachineLogs('taloscontainers-web', [container({ state: 'pending' })])

  expect(await screen.findByText('Waiting for web to start')).toBeInTheDocument()
  expect(screen.queryByText('Failed to Fetch Logs')).not.toBeInTheDocument()
  expect(streamState.opened).toBe(0)
})

test('streams a Talos container that is running', async () => {
  renderMachineLogs('taloscontainers-web', [container({ state: 'running' })])

  expect(await screen.findByText('Failed to Fetch Logs')).toBeInTheDocument()
  expect(screen.queryByText('Waiting for web to start')).not.toBeInTheDocument()
  expect(streamState.opened).toBe(1)
})

// Talos keeps a container's log across restarts, so a restart has one to show
test('streams a Talos container that is starting again after a restart', async () => {
  renderMachineLogs('taloscontainers-web', [container({ state: 'starting', restartCount: 3 })])

  expect(await screen.findByText('Failed to Fetch Logs')).toBeInTheDocument()
  expect(streamState.opened).toBe(1)
})
