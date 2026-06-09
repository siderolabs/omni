// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.
import { faker } from '@faker-js/faker'
import { createWatchStreamHandler } from '@msw/helpers'
import type { Meta, StoryObj } from '@storybook/vue3-vite'
import { http, HttpResponse } from 'msw'

import type { Resource } from '@/api/grpc'
import type { GetRequest, GetResponse } from '@/api/omni/resources/resources.pb'
import type { MachineStatusLinkSpec } from '@/api/omni/specs/ephemeral.pb'
import type { InfraProviderStatusSpec } from '@/api/omni/specs/infra.pb'
import type { MachineStatusMetricsSpec, MachineStatusSnapshotSpec } from '@/api/omni/specs/omni.pb'
import { MachineStatusSpecPowerState } from '@/api/omni/specs/omni.pb'
import type { PermissionsSpec } from '@/api/omni/specs/virtual.pb'
import {
  DefaultNamespace,
  EphemeralNamespace,
  InfraProviderNamespace,
  InfraProviderStatusType,
  LabelCluster,
  LabelsCompletionType,
  MachineStatusLabelArch,
  MachineStatusLabelAvailable,
  MachineStatusLabelConnected,
  MachineStatusLabelCores,
  MachineStatusLabelCPU,
  MachineStatusLabelInstalled,
  MachineStatusLabelMem,
  MachineStatusLabelPlatform,
  MachineStatusLabelReadyToUse,
  MachineStatusLabelReportingEvents,
  MachineStatusLabelStorage,
  MachineStatusLabelTalosVersion,
  MachineStatusLinkType,
  MachineStatusMetricsID,
  MachineStatusMetricsType,
  MachineStatusSnapshotType,
  MachineStatusType,
  MetricsNamespace,
  PermissionsID,
  PermissionsType,
  VirtualNamespace,
} from '@/api/resources'
import { MachineStatusEventMachineStage } from '@/api/talos/machine/machine.pb'

import Machines from './Machines.vue'

faker.seed(42)

const machineIds = faker.helpers.multiple(faker.string.uuid, { count: 100 })
const clusterIds = faker.helpers.multiple(() => `cluster-${faker.hacker.noun()}`, { count: 5 })

const machines = machineIds.map<Resource<MachineStatusLinkSpec>>((id) => {
  const cluster = faker.helpers.maybe(() => faker.helpers.arrayElement(clusterIds))
  const connected = faker.datatype.boolean({ probability: 0.9 })
  const talosVersion = `v${faker.system.semver()}`

  const labels: Record<string, string> = {
    [MachineStatusLabelArch]: faker.datatype.boolean() ? 'arm64' : 'amd64',
    [MachineStatusLabelCores]: faker.number.int({ min: 2, max: 16 }).toString(),
    [MachineStatusLabelCPU]: 'qemu',
    [MachineStatusLabelMem]: `${faker.number.int({ min: 2, max: 32, multipleOf: 2 })}GiB`,
    [MachineStatusLabelPlatform]: 'metal',
    [MachineStatusLabelStorage]: `${faker.number.int({ min: 10, max: 500, multipleOf: 10 })}GB`,
    [MachineStatusLabelTalosVersion]: talosVersion,
  }
  if (cluster) {
    labels[LabelCluster] = cluster
  } else {
    labels[MachineStatusLabelAvailable] = ''
  }

  if (connected) {
    labels[MachineStatusLabelConnected] = ''
    labels[MachineStatusLabelReportingEvents] = ''
    labels[MachineStatusLabelReadyToUse] = ''
  }

  if (faker.datatype.boolean({ probability: 0.75 })) labels[MachineStatusLabelInstalled] = ''

  return {
    spec: {
      tearing_down: faker.datatype.boolean({ probability: 0.1 }),
      message_status: {
        connected,
        maintenance: faker.datatype.boolean({ probability: 0.25 }),
        cluster,
        talos_version: talosVersion,
        network: {
          hostname: faker.internet.domainWord(),
        },
        power_state: faker.helpers.enumValue(MachineStatusSpecPowerState),
      },
    },
    metadata: {
      namespace: MetricsNamespace,
      type: MachineStatusLinkType,
      id,
      labels,
    },
  }
})

const metrics: Resource<MachineStatusMetricsSpec> = {
  metadata: {
    namespace: EphemeralNamespace,
    type: MachineStatusMetricsType,
    id: MachineStatusMetricsID,
  },
  spec: {
    registered_machines_count: machineIds.length,
    connected_machines_count: 3,
    allocated_machines_count: 1,
  },
}

const permissionsHandler = http.post<never, GetRequest, GetResponse>(
  '/omni.resources.ResourceService/Get',
  async ({ request }) => {
    const { type, namespace } = await request.clone().json()

    if (type !== PermissionsType || namespace !== VirtualNamespace) return

    return HttpResponse.json({
      body: JSON.stringify({
        spec: {
          can_access_maintenance_nodes: true,
          can_create_clusters: true,
          can_manage_backup_store: true,
          can_manage_machine_config_patches: true,
          can_manage_users: true,
          can_read_audit_log: true,
          can_read_clusters: true,
          can_read_machine_config_patches: true,
          can_read_machine_logs: true,
          can_read_machines: true,
          can_remove_machines: true,
        },
        metadata: {
          namespace: VirtualNamespace,
          type: PermissionsType,
          id: PermissionsID,
        },
      } as Resource<PermissionsSpec>),
    })
  },
)

const meta: Meta<typeof Machines> = {
  component: Machines,
  parameters: {
    layout: 'fullscreen',
  },
  decorators: [
    () => ({
      template: '<div class="h-screen"><story /></div>',
    }),
  ],
}

export default meta
type Story = StoryObj<typeof meta>

export const Default: Story = {
  parameters: {
    msw: {
      handlers: [
        createWatchStreamHandler<MachineStatusLinkSpec>({
          expectedOptions: {
            type: MachineStatusLinkType,
            namespace: MetricsNamespace,
          },
          totalResults: machineIds.length,
          initialResources: ({ limit = 5, offset = 0 }) => machines.slice(0, offset + limit),
        }).handler,

        createWatchStreamHandler<MachineStatusSnapshotSpec>({
          expectedOptions: {
            type: MachineStatusSnapshotType,
            namespace: DefaultNamespace,
          },
          initialResources: machineIds.map<Resource<MachineStatusSnapshotSpec>>((id) => ({
            spec: {
              machine_status: {
                stage: faker.helpers.enumValue(MachineStatusEventMachineStage),
              },
            },
            metadata: {
              namespace: DefaultNamespace,
              type: MachineStatusSnapshotType,
              id,
            },
          })),
        }).handler,

        createWatchStreamHandler<MachineStatusMetricsSpec>({
          expectedOptions: {
            type: MachineStatusMetricsType,
            namespace: EphemeralNamespace,
            id: MachineStatusMetricsID,
          },
          initialResources: [metrics],
        }).handler,

        createWatchStreamHandler<InfraProviderStatusSpec>({
          expectedOptions: {
            type: InfraProviderStatusType,
            namespace: InfraProviderNamespace,
          },
          initialResources: [],
        }).handler,

        createWatchStreamHandler({
          expectedOptions: {
            id: MachineStatusType,
            type: LabelsCompletionType,
            namespace: VirtualNamespace,
          },
          initialResources: [],
        }).handler,

        permissionsHandler,
      ],
    },
  },
}

export const NoData: Story = {
  parameters: {
    msw: {
      handlers: [createWatchStreamHandler().handler, permissionsHandler],
    },
  },
}
