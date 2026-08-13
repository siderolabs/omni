// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.
import { faker } from '@faker-js/faker'
import { createWatchStreamHandler } from '@msw/helpers'
import type { Meta, StoryObj } from '@storybook/vue3-vite'
import { http, HttpResponse } from 'msw'

import type { Resource } from '@/api/grpc'
import type {
  GetRequest,
  GetResponse,
  ListRequest,
  ListResponse,
} from '@/api/omni/resources/resources.pb'
import type {
  ClusterConfigVersionSpec,
  ClusterSpec,
  MachineClassSpec,
  MachineConfigGenOptionsSpec,
  MachineSetNodeSpec,
  MachineSetSpec,
  MachineStatusSpec,
} from '@/api/omni/specs/omni.pb'
import type { VersionContractSpec } from '@/api/omni/specs/virtual.pb'
import {
  ClusterConfigVersionType,
  ClusterType,
  ConfigPatchType,
  DefaultNamespace,
  DefaultTalosVersion,
  ExtensionsConfigurationType,
  LabelCluster,
  LabelControlPlaneRole,
  LabelMachineSet,
  LabelWorkerRole,
  MachineClassType,
  MachineConfigGenOptionsType,
  MachineSetNodeType,
  MachineSetType,
  MachineStatusLabelAvailable,
  MachineStatusLabelReadyToUse,
  MachineStatusLabelReportingEvents,
  MachineStatusType,
  VersionContractType,
  VirtualNamespace,
} from '@/api/resources'
import { controlPlaneMachineSetId, defaultWorkersMachineSetId } from '@/methods/machineset'

import ClusterScale from './scale.vue'

const clusterId = 'storybook-cluster'

const meta: Meta<typeof ClusterScale> = {
  component: ClusterScale,
  parameters: {
    layout: 'fullscreen',
  },
  args: {
    currentCluster: {
      metadata: { id: clusterId },
      spec: {},
    },
  },
  decorators: [() => ({ template: '<div class="h-screen"><story/></div>' })],
}

export default meta
type Story = StoryObj<typeof meta>

const machineIDs = faker.helpers.multiple(() => faker.string.uuid(), { count: 8 })

const machineSetNodeIDs = [
  { id: faker.string.uuid(), machineSet: controlPlaneMachineSetId(clusterId) },
  { id: faker.string.uuid(), machineSet: controlPlaneMachineSetId(clusterId) },
  { id: faker.string.uuid(), machineSet: controlPlaneMachineSetId(clusterId) },
  { id: faker.string.uuid(), machineSet: defaultWorkersMachineSetId(clusterId) },
  { id: faker.string.uuid(), machineSet: defaultWorkersMachineSetId(clusterId) },
]

export const Data: Story = {
  beforeEach({ msw }) {
    msw.use(
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
      createWatchStreamHandler<MachineClassSpec>({
        expectedOptions: {
          type: MachineClassType,
          namespace: DefaultNamespace,
        },
        initialResources: [],
      }).handler,
      http.post<never, ListRequest, ListResponse>(
        '/omni.resources.ResourceService/List',
        async ({ request }) => {
          const { type, namespace } = await request.clone().json()

          if (type !== MachineSetType || namespace !== DefaultNamespace) return

          const items: Resource<MachineSetSpec>[] = [
            {
              spec: {},
              metadata: {
                id: controlPlaneMachineSetId(clusterId),
                type: MachineSetType,
                namespace: DefaultNamespace,
                labels: {
                  [LabelCluster]: clusterId,
                  [LabelControlPlaneRole]: '',
                },
              },
            },
            {
              spec: {},
              metadata: {
                id: defaultWorkersMachineSetId(clusterId),
                type: MachineSetType,
                namespace: DefaultNamespace,
                labels: {
                  [LabelCluster]: clusterId,
                  [LabelWorkerRole]: '',
                },
              },
            },
          ]

          return HttpResponse.json({
            items: items.map((i) => JSON.stringify(i)),
            total: items.length,
          })
        },
      ),
      http.post<never, ListRequest, ListResponse>(
        '/omni.resources.ResourceService/List',
        async ({ request }) => {
          const { type, namespace } = await request.clone().json()

          if (type !== MachineSetNodeType || namespace !== DefaultNamespace) return

          const items: Resource<MachineSetNodeSpec>[] = machineSetNodeIDs.map(
            ({ id, machineSet }) => ({
              spec: {},
              metadata: {
                id,
                type: MachineSetNodeType,
                namespace: DefaultNamespace,
                labels: {
                  [LabelCluster]: clusterId,
                  [LabelMachineSet]: machineSet,
                },
              },
            }),
          )

          return HttpResponse.json({
            items: items.map((i) => JSON.stringify(i)),
            total: items.length,
          })
        },
      ),
      http.post<never, ListRequest, ListResponse>(
        '/omni.resources.ResourceService/List',
        async ({ request }) => {
          const { type, namespace } = await request.clone().json()

          if (type !== ConfigPatchType || namespace !== DefaultNamespace) return

          return HttpResponse.json({ items: [], total: 0 })
        },
      ),
      http.post<never, ListRequest, ListResponse>(
        '/omni.resources.ResourceService/List',
        async ({ request }) => {
          const { type, namespace } = await request.clone().json()

          if (type !== ExtensionsConfigurationType || namespace !== DefaultNamespace) return

          return HttpResponse.json({ items: [], total: 0 })
        },
      ),
      http.post<never, GetRequest>('/omni.resources.ResourceService/Get', async ({ request }) => {
        const { type, namespace, id } = await request.clone().json()

        if (
          type !== VersionContractType ||
          namespace !== VirtualNamespace ||
          id !== DefaultTalosVersion
        ) {
          return
        }

        return HttpResponse.json({
          body: JSON.stringify({
            spec: {
              discovery_service_multidoc_config: true,
              kube_span_multidoc_config: true,
              multidoc_kubernetes_config_supported: true,
              multidoc_network_config_supported: true,
            } satisfies VersionContractSpec,
            metadata: {
              namespace: VirtualNamespace,
              type: VersionContractType,
              id,
            },
          } as Resource<VersionContractSpec>),
        })
      }),
      http.post<never, GetRequest, GetResponse>(
        '/omni.resources.ResourceService/Get',
        async ({ request }) => {
          const { id, type, namespace } = await request.clone().json()

          if (id !== clusterId || type !== ClusterType || namespace !== DefaultNamespace) return

          return HttpResponse.json({
            body: JSON.stringify({
              spec: {
                talos_version: DefaultTalosVersion,
                kubernetes_version: '1.31.0',
              } satisfies ClusterSpec,
              metadata: {
                namespace: DefaultNamespace,
                type: ClusterType,
                id,
              },
            } as Resource<ClusterSpec>),
          })
        },
      ),
      http.post<never, GetRequest, GetResponse>(
        '/omni.resources.ResourceService/Get',
        async ({ request }) => {
          const { id, type, namespace } = await request.clone().json()

          if (
            id !== clusterId ||
            type !== ClusterConfigVersionType ||
            namespace !== DefaultNamespace
          )
            return

          return HttpResponse.json({
            body: JSON.stringify({
              spec: { version: DefaultTalosVersion } satisfies ClusterConfigVersionSpec,
              metadata: {
                namespace: DefaultNamespace,
                type: ClusterType,
                id,
              },
            } as Resource<ClusterConfigVersionSpec>),
          })
        },
      ),
    )
  },
}

export const NoData: Story = {
  beforeEach({ msw }) {
    msw.use(
      createWatchStreamHandler().handler,
      http.post<never, GetRequest>('/omni.resources.ResourceService/Get', async ({ request }) => {
        const { type, namespace, id } = await request.clone().json()

        return HttpResponse.json({
          body: JSON.stringify({
            spec: {},
            metadata: { type, namespace, id },
          } satisfies Resource),
        })
      }),
      http.post<never, ListRequest, ListResponse>('/omni.resources.ResourceService/List', () =>
        HttpResponse.json({ items: [], total: 0 }),
      ),
    )
  },
}
