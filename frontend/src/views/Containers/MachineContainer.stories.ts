// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.
import { createWatchStreamHandler } from '@mocks/helpers'
import type { Meta, StoryObj } from '@storybook/vue3-vite'

import {
  TalosContainerHealthStatusType,
  TalosContainerImageStatusType,
  TalosContainerInstanceStatusType,
  TalosContainerMountStatusType,
  TalosContainerRunStatusType,
  TalosContainersNamespace,
  TalosContainerSpecType,
  TalosContainerStatusType,
} from '@/api/resources'
import type {
  ContainerHealthStatusSpec,
  ContainerImageStatusSpec,
  ContainerInstanceStatusSpec,
  ContainerMountStatusSpec,
  ContainerRunStatusSpec,
  ContainerSpecSpec,
  ContainerStatusSpec,
} from '@/api/talos/containers.pb'
import { provideMachineContainers } from '@/views/Containers/lib/machineContainers'

import MachineContainer from './MachineContainer.vue'

const machineId = 'machine'

const ago = (seconds: number) => new Date(Date.now() - seconds * 1000).toISOString()

const metadata = (type: string, id: string, annotations?: Record<string, string>) => ({
  id,
  namespace: TalosContainersNamespace,
  type,
  annotations,
})

/** Serves one resource of a type, to the watch for it by ID (or to the list watch without one). */
const handler = <T>(
  type: string,
  id: string | undefined,
  spec: T,
  annotations?: Record<string, string>,
) =>
  createWatchStreamHandler<T>({
    expectedOptions: id
      ? { namespace: TalosContainersNamespace, type, id }
      : { namespace: TalosContainersNamespace, type },
    initialResources: id ? [{ metadata: metadata(type, id, annotations), spec }] : [],
  }).handler

const listHandler = <T>(type: string, resources: { id: string; spec: T }[]) =>
  createWatchStreamHandler<T>({
    expectedOptions: { namespace: TalosContainersNamespace, type },
    initialResources: resources.map(({ id, spec }) => ({ metadata: metadata(type, id), spec })),
  }).handler

const webStatus: ContainerStatusSpec = {
  state: 'running',
  health: 'degraded',
  restartCount: 2,
  generation: 2,
  pid: 4211,
  error: 'health check: dial tcp 127.0.0.1:80: connection refused',
}

const webSpec: ContainerSpecSpec = {
  image: { ref: 'docker.io/library/nginx:1.27' },
  entrypoint: ['REDACTED'],
  environment: ['REDACTED'],
  runAs: { uid: 101, gid: 101 },
  workingDir: '/usr/share/nginx',
  mounts: [
    { kind: 'tmpfs', destination: '/var/cache/nginx', size: 64 * 2 ** 20 },
    { kind: 'volume', volumeName: 'web-data', destination: '/data', options: ['rw'] },
    { kind: 'hostPath', source: '/var/log/web', destination: '/var/log/nginx', options: ['ro'] },
  ],
  security: {
    capabilitiesAdd: ['NET_BIND_SERVICE', 'CHOWN'],
    capabilitiesDrop: ['ALL'],
    readOnlyRootFS: true,
  },
  network: { hostNetwork: true },
  resources: { cpuLimit: 500, memoryLimit: 256 * 2 ** 20 },
  dependsOn: {
    networks: ['online'],
    paths: ['/var/run/ready'],
    containers: [{ name: 'db', condition: 'running' }],
  },
  restart: { policy: 'always', delay: '1s', maxDelay: '5m0s' },
  stopTimeout: '30s',
  healthCheck: {
    kind: 'http',
    port: 80,
    path: '/healthz',
    interval: '10s',
    timeout: '2s',
    failureThreshold: 3,
  },
}

const webInstances: ContainerInstanceStatusSpec[] = [
  {
    containerID: 'web',
    generation: 0,
    phase: 'terminated',
    exitCode: 137,
    startedAt: ago(5 * 3600),
    finishedAt: ago(4 * 3600),
    error: 'task "web-0" failed: exit code 137',
  },
  {
    containerID: 'web',
    generation: 1,
    phase: 'terminated',
    exitCode: 1,
    startedAt: ago(4 * 3600 - 60),
    finishedAt: ago(3 * 3600 + 6 * 60),
    error: 'task "web-1" failed: exit code 1',
  },
  {
    containerID: 'web',
    generation: 2,
    phase: 'running',
    pid: 4211,
    startedAt: ago(3 * 3600 + 5 * 60),
  },
]

const containerHandlers = (
  id: string,
  {
    status,
    spec,
    image,
    run,
    mounts,
    health,
    instances = [],
  }: {
    status: ContainerStatusSpec
    spec: ContainerSpecSpec
    image?: ContainerImageStatusSpec
    run?: ContainerRunStatusSpec
    mounts?: ContainerMountStatusSpec
    health?: ContainerHealthStatusSpec
    instances?: ContainerInstanceStatusSpec[]
  },
) => [
  listHandler(TalosContainerStatusType, [{ id, spec: status }]),
  handler(TalosContainerSpecType, id, spec, { 'talos.dev/redacted': 'true' }),
  handler(TalosContainerImageStatusType, image && id, image ?? {}),
  handler(TalosContainerRunStatusType, run && id, run ?? {}),
  handler(TalosContainerMountStatusType, mounts && id, mounts ?? {}),
  handler(
    TalosContainerHealthStatusType,
    health && `${id}-${status.generation ?? 0}`,
    health ?? {},
  ),
  listHandler(
    TalosContainerInstanceStatusType,
    instances.map((spec) => ({ id: `${spec.containerID}-${spec.generation}`, spec })),
  ),
]

const meta: Meta<typeof MachineContainer> = {
  component: MachineContainer,
  args: { clusterId: 'cluster', machineId, containerId: 'web' },
  // The machine page provides the container watch; stand in for it
  decorators: [
    (story) => ({
      components: { story },
      setup() {
        provideMachineContainers(machineId, '1.15.0')
      },
      template: '<story />',
    }),
  ],
}

export default meta
type Story = StoryObj<typeof meta>

export const Default: Story = {
  beforeEach({ msw }) {
    msw.use(
      ...containerHandlers('web', {
        status: webStatus,
        spec: webSpec,
        image: {
          phase: 'ready',
          image: 'docker.io/library/nginx:1.27',
          digest: 'sha256:6784fb0834aa7dbbe12e3d7471e69c290df3e6ba810dc38b34ae33d3c1c05f7d',
        },
        run: { restarts: 2, policy: 'always', backoffStep: 1 },
        mounts: { ready: true },
        health: {
          healthy: false,
          message: 'dial tcp 127.0.0.1:80: connection refused',
          lastChange: ago(120),
        },
        instances: webInstances,
      }),
    )
  },
}

/** A phone's width, where the run history stacks */
export const Narrow: Story = {
  ...Default,
  globals: { viewport: { value: 'mobile2', isRotated: false } },
}

export const Waiting: Story = {
  args: { containerId: 'backup-agent' },
  beforeEach({ msw }) {
    msw.use(
      ...containerHandlers('backup-agent', {
        status: {
          state: 'pending',
          health: 'pending',
          waitingFor: ['volume backup-cache', 'network online'],
        },
        spec: {
          image: { ref: 'docker.io/restic/rest-server:0.14.0' },
          dependsOn: { networks: ['online'] },
          mounts: [{ kind: 'volume', volumeName: 'backup-cache', destination: '/cache' }],
          restart: { policy: 'on-failure', maxRetries: 5 },
        },
        image: { phase: 'ready', image: 'docker.io/restic/rest-server:0.14.0' },
        mounts: { ready: false },
      }),
    )
  },
}

export const NotFound: Story = {
  args: { containerId: 'missing' },
  beforeEach({ msw }) {
    msw.use(...containerHandlers('web', { status: webStatus, spec: webSpec }))
  },
}
