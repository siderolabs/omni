// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.
import { createWatchStreamHandler } from '@mocks/helpers'
import type { Meta, StoryObj } from '@storybook/vue3-vite'
import { http, HttpResponse } from 'msw'

import {
  TalosContainerImageStatusType,
  TalosContainerInstanceStatusType,
  TalosContainersNamespace,
  TalosContainerStatusType,
} from '@/api/resources'
import type {
  ContainerImageStatusSpec,
  ContainerInstanceStatusSpec,
  ContainerStatusSpec,
} from '@/api/talos/containers.pb'
import type { StatsResponse } from '@/api/talos/machine/machine.pb'
import { provideMachineContainers } from '@/views/Containers/lib/machineContainers'

import MachineContainers from './MachineContainers.vue'

const machineId = 'machine'

/** A mocked container: what Talos reports about it, its image and its newest instance. */
interface MockContainer {
  id: string
  status: ContainerStatusSpec
  image?: ContainerImageStatusSpec
  instance?: Omit<ContainerInstanceStatusSpec, 'containerID'>
  /** What a running instance uses: CPU in cores, memory in bytes. */
  usage?: { cores: number; memory: number }
}

const ago = (seconds: number) => new Date(Date.now() - seconds * 1000).toISOString()

const watchHandler = <T>(type: string, resources: { id: string; spec: T }[]) =>
  createWatchStreamHandler<T>({
    expectedOptions: { namespace: TalosContainersNamespace, type },
    initialResources: resources.map(({ id, spec }) => ({
      metadata: { id, namespace: TalosContainersNamespace, type },
      spec,
    })),
  }).handler

// Stats report CPU time as a running total, so it grows with each poll at the container's rate
const statsHandler = (containers: MockContainer[]) => {
  const start = Date.now()

  return http.post('/machine.MachineService/Stats', () =>
    HttpResponse.json<StatsResponse>({
      messages: [
        {
          stats: containers.flatMap(({ id, status, usage }) =>
            usage
              ? [
                  {
                    id: `${id}-${status.generation}`,
                    cpu_usage: String(Math.round(usage.cores * (Date.now() - start) * 1e6)),
                    memory_usage: String(usage.memory),
                  },
                ]
              : [],
          ),
        },
      ],
    }),
  )
}

const containerHandlers = (containers: MockContainer[]) => [
  statsHandler(containers),
  watchHandler(
    TalosContainerStatusType,
    containers.map(({ id, status }) => ({ id, spec: status })),
  ),
  watchHandler(
    TalosContainerImageStatusType,
    containers.flatMap(({ id, image }) => (image ? [{ id, spec: image }] : [])),
  ),
  watchHandler(
    TalosContainerInstanceStatusType,
    containers.flatMap(({ id, instance }) =>
      instance
        ? [{ id: `${id}-${instance.generation ?? 1}`, spec: { containerID: id, ...instance } }]
        : [],
    ),
  ),
]

const containers: MockContainer[] = [
  {
    id: 'node-exporter',
    status: { state: 'running', health: 'healthy', restartCount: 0, generation: 1 },
    image: { phase: 'ready', image: 'quay.io/prometheus/node-exporter:v1.10.2' },
    instance: { generation: 1, phase: 'running', startedAt: ago(12 * 86400 + 6 * 3600) },
    usage: { cores: 0.012, memory: 18.4 * 2 ** 20 },
  },
  {
    id: 'fluent-bit',
    status: {
      state: 'backoff',
      health: 'degraded',
      restartCount: 7,
      exitCode: 1,
      error: 'task "fluent-bit" failed: exit code 1',
      errorLastLog: 'starting\n\x1b[31merror\x1b[0m open /var/log/journal: permission denied',
    },
    image: { phase: 'ready', image: 'cr.fluentbit.io/fluent/fluent-bit:4.1.1' },
    instance: {
      generation: 8,
      phase: 'terminated',
      exitCode: 1,
      startedAt: ago(61),
      finishedAt: ago(41),
    },
  },
  {
    id: 'backup-agent',
    status: {
      state: 'pending',
      health: 'pending',
      waitingFor: ['volume backup-cache', 'network online'],
    },
    image: { phase: 'ready', image: 'docker.io/restic/rest-server:0.14.0' },
  },
  {
    id: 'tailscale',
    status: { state: 'pulling', health: 'pulling' },
    image: { phase: 'pulling', image: 'ghcr.io/tailscale/tailscale:v1.90.6' },
  },
  {
    id: 'web',
    status: {
      state: 'running',
      health: 'degraded',
      restartCount: 1,
      generation: 2,
      error: 'health check: dial tcp 127.0.0.1:80: connection refused',
    },
    image: { phase: 'ready', image: 'docker.io/library/nginx:1.27' },
    instance: { generation: 2, phase: 'running', startedAt: ago(3 * 3600 + 5 * 60) },
    usage: { cores: 0.42, memory: 212 * 2 ** 20 },
  },
  {
    id: 'db-migrate',
    status: { state: 'succeeded', health: 'healthy' },
    image: { phase: 'ready', image: 'ghcr.io/example/migrate:2026.11' },
    instance: { generation: 1, phase: 'terminated', startedAt: ago(4000), finishedAt: ago(3600) },
  },
  {
    id: 'never-started',
    status: {
      state: 'failed',
      health: 'degraded',
      error: 'failed to get image "ghcr.io/example/missing:latest": not found',
    },
  },
]

const meta: Meta<typeof MachineContainers> = {
  component: MachineContainers,
  args: { clusterId: 'cluster', machineId },
  // The machine page provides the container watch; stand in for it, with the Talos version
  // from the story's parameters.
  decorators: [
    (story, { parameters }) => ({
      components: { story },
      setup() {
        provideMachineContainers(machineId, parameters.talosVersion ?? '1.15.0')
      },
      template: '<story />',
    }),
  ],
  beforeEach({ msw }) {
    msw.use(...containerHandlers([]))
  },
}

export default meta
type Story = StoryObj<typeof meta>

export const Default: Story = {
  beforeEach({ msw }) {
    msw.use(...containerHandlers(containers))
  },
}

export const NoContainers: Story = {}

export const UnsupportedTalos: Story = {
  parameters: { talosVersion: '1.14.5' },
}
