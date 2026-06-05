// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.
import { faker } from '@faker-js/faker'
import { createWatchStreamHandler } from '@msw/helpers.ts'
import type { Meta, StoryObj } from '@storybook/vue3-vite'
import { http, HttpResponse } from 'msw'

import type { Resource } from '@/api/grpc'
import type { GetRequest, ListRequest, ListResponse } from '@/api/omni/resources/resources.pb'
import type {
  EtcdBackupOverallStatusSpec,
  FeaturesConfigSpec,
  MachineConfigGenOptionsSpec,
  MachineStatusSpec,
  TalosVersionSpec,
} from '@/api/omni/specs/omni.pb'
import type { PermissionsSpec } from '@/api/omni/specs/virtual.pb'
import {
  ClusterType,
  DefaultNamespace,
  DefaultTalosVersion,
  EtcdBackupOverallStatusID,
  EtcdBackupOverallStatusType,
  FeaturesConfigID,
  FeaturesConfigType,
  MachineConfigGenOptionsType,
  MachineStatusLabelAvailable,
  MachineStatusLabelReadyToUse,
  MachineStatusLabelReportingEvents,
  MachineStatusType,
  MetricsNamespace,
  PermissionsID,
  PermissionsType,
  TalosVersionType,
  VirtualNamespace,
} from '@/api/resources'

import ClusterCreate from './create.vue'

const meta: Meta<typeof ClusterCreate> = {
  component: ClusterCreate,
  parameters: {
    layout: 'fullscreen',
  },
}

export default meta
type Story = StoryObj<typeof meta>

const machineIDs = faker.helpers.multiple(() => faker.string.uuid(), { count: 8 })

export const Data: Story = {
  parameters: {
    msw: {
      handlers: [
        createWatchStreamHandler<TalosVersionSpec>({
          expectedOptions: {
            type: TalosVersionType,
            namespace: DefaultNamespace,
          },
          initialResources: faker.helpers
            .uniqueArray<string>(
              () =>
                `1.${faker.number.int({ min: 6, max: 11 })}.${faker.number.int({ min: 0, max: 10 })}`,
              40,
            )
            .concat(DefaultTalosVersion)
            .map((version) => ({
              spec: {
                version,
                deprecated: faker.datatype.boolean(),
                unsupported: faker.datatype.boolean(),
              },
              metadata: {
                id: version,
                type: TalosVersionType,
                namespace: DefaultNamespace,
              },
            })),
        }).handler,

        createWatchStreamHandler<MachineConfigGenOptionsSpec>({
          expectedOptions: {
            type: MachineConfigGenOptionsType,
            namespace: DefaultNamespace,
          },
          initialResources: machineIDs.map((id) => ({
            spec: {
              install_disk: '/dev/sda',
            },
            metadata: {
              id,
              type: MachineConfigGenOptionsType,
              namespace: DefaultNamespace,
            },
          })),
        }).handler,

        createWatchStreamHandler<MachineStatusSpec>({
          expectedOptions: {
            type: MachineStatusType,
            namespace: DefaultNamespace,
          },
          initialResources: machineIDs.map((id) => ({
            spec: {
              talos_version: DefaultTalosVersion,
              connected: true,
              network: {
                hostname: faker.internet.domainWord(),
                addresses: [faker.internet.ipv4()],
              },
              hardware: {
                processors: [{ manufacturer: 'Intel', description: faker.hacker.noun() }],
                memory_modules: [{ size_mb: faker.helpers.arrayElement([8192, 16384, 32768]) }],
                blockdevices: [
                  {
                    linux_name: 'sda',
                    size: String(faker.number.int({ min: 100, max: 2000 }) * 1_000_000_000),
                    type: 'ssd',
                  },
                ],
              },
            },
            metadata: {
              id,
              type: MachineStatusType,
              namespace: DefaultNamespace,
              labels: {
                [MachineStatusLabelAvailable]: '',
                [MachineStatusLabelReadyToUse]: '',
                [MachineStatusLabelReportingEvents]: '',
                foo: 'bar',
              },
            },
          })),
        }).handler,

        createWatchStreamHandler<FeaturesConfigSpec>({
          expectedOptions: {
            type: FeaturesConfigType,
            namespace: DefaultNamespace,
            id: FeaturesConfigID,
          },
          initialResources: [
            {
              spec: {
                enable_workload_proxying: true,
                embedded_discovery_service: true,
              },
              metadata: {
                id: FeaturesConfigID,
                type: FeaturesConfigType,
                namespace: DefaultNamespace,
              },
            },
          ],
        }).handler,

        createWatchStreamHandler<EtcdBackupOverallStatusSpec>({
          expectedOptions: {
            type: EtcdBackupOverallStatusType,
            namespace: MetricsNamespace,
            id: EtcdBackupOverallStatusID,
          },
          initialResources: [
            {
              spec: { configuration_name: 's3' },
              metadata: {
                id: EtcdBackupOverallStatusID,
                type: EtcdBackupOverallStatusType,
                namespace: MetricsNamespace,
              },
            },
          ],
        }).handler,

        http.post<never, GetRequest>('/omni.resources.ResourceService/Get', async ({ request }) => {
          const { type, namespace, id } = await request.clone().json()

          if (type !== PermissionsType || namespace !== VirtualNamespace || id !== PermissionsID) {
            return
          }

          return HttpResponse.json({
            body: JSON.stringify({
              spec: {
                can_read_clusters: true,
                can_create_clusters: true,
                can_manage_users: true,
                can_read_machines: true,
                can_remove_machines: true,
                can_read_machine_logs: true,
                can_read_machine_config_patches: true,
                can_manage_machine_config_patches: true,
              } satisfies PermissionsSpec,
              metadata: {
                namespace: VirtualNamespace,
                type: PermissionsType,
                id: PermissionsID,
              },
            } as Resource<PermissionsSpec>),
          })
        }),

        http.post<never, ListRequest, ListResponse>(
          '/omni.resources.ResourceService/List',
          async ({ request }) => {
            const { type, namespace } = await request.clone().json()

            if (type !== ClusterType || namespace !== DefaultNamespace) return

            return HttpResponse.json({ items: [], total: 0 })
          },
        ),
      ],
    },
  },
}

export const NoData: Story = {
  parameters: {
    msw: {
      handlers: [
        createWatchStreamHandler().handler,

        http.post<never, GetRequest>('/omni.resources.ResourceService/Get', async ({ request }) => {
          const { type, namespace, id } = await request.clone().json()

          if (type !== PermissionsType || namespace !== VirtualNamespace || id !== PermissionsID) {
            return
          }

          return HttpResponse.json({
            body: JSON.stringify({
              spec: { can_create_clusters: true } satisfies PermissionsSpec,
              metadata: { namespace: VirtualNamespace, type: PermissionsType, id: PermissionsID },
            } as Resource<PermissionsSpec>),
          })
        }),

        http.post<never, ListRequest, ListResponse>(
          '/omni.resources.ResourceService/List',
          async ({ request }) => {
            const { type, namespace } = await request.clone().json()

            if (type !== ClusterType || namespace !== DefaultNamespace) return

            return HttpResponse.json({ items: [], total: 0 })
          },
        ),
      ],
    },
  },
}
