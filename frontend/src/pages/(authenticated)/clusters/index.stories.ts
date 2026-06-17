// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.
import { faker } from '@faker-js/faker'
import { createWatchStreamHandler } from '@msw/helpers'
import type { Meta, StoryObj } from '@storybook/vue3-vite'
import { http, HttpResponse } from 'msw'

import type { Resource } from '@/api/grpc'
import type { GetRequest } from '@/api/omni/resources/resources.pb'
import {
  type ClusterDiagnosticsSpec,
  type ClusterMachineRequestStatusSpec,
  ClusterMachineRequestStatusSpecStage,
  type ClusterMachineStatusSpec,
  ClusterMachineStatusSpecStage,
  type ClusterStatusMetricsSpec,
  type ClusterStatusSpec,
  ClusterStatusSpecPhase,
  ConfigApplyStatus,
  type MachineSetSpec,
  type MachineSetSpecMachineAllocation,
  MachineSetSpecMachineAllocationType,
  type MachineSetStatusSpec,
} from '@/api/omni/specs/omni.pb'
import type { ClusterPermissionsSpec } from '@/api/omni/specs/virtual.pb'
import {
  ClusterDiagnosticsType,
  ClusterLocked,
  ClusterMachineRequestStatusType,
  ClusterMachineStatusType,
  ClusterPermissionsType,
  ClusterStatusMetricsType,
  ClusterStatusType,
  ControlPlanesIDSuffix,
  DefaultNamespace,
  DefaultWorkersIDSuffix,
  EphemeralNamespace,
  LabelCluster,
  LabelControlPlaneRole,
  LabelMachineSet,
  LabelWorkerRole,
  MachineLocked,
  MachineSetStatusType,
  MachineSetType,
  MachineStatusLabelConnected,
  UpdateLocked,
  VirtualNamespace,
} from '@/api/resources'
import MachineSetPhase from '@/views/ClusterMachines/MachineSetPhase.vue'

import Clusters from './index.vue'

const meta: Meta<typeof Clusters> = {
  component: Clusters,
}

export default meta
type Story = StoryObj<typeof meta>

const seedFromId = (id: string) => id.split('').reduce((prev, curr) => prev + curr.charCodeAt(0), 0)

// Generates the machine statuses for a single machine set. faker is a sequential
// PRNG, so reseeding from the machine-set label makes the generated UUIDs
// deterministic — which lets the diagnostics handler reproduce the exact machine
// IDs that belong to a given cluster without coordinating shared state.
const clusterMachineStatuses = (
  clusterLabel: string,
  machineSetLabel: string,
): Resource<ClusterMachineStatusSpec>[] => {
  faker.seed(seedFromId(machineSetLabel))

  const roleLabel = machineSetLabel.includes(ControlPlanesIDSuffix)
    ? LabelControlPlaneRole
    : LabelWorkerRole

  return faker.helpers.multiple<Resource<ClusterMachineStatusSpec>>(
    () => ({
      spec: {
        ready: faker.datatype.boolean(),
        stage: faker.helpers.enumValue(ClusterMachineStatusSpecStage),
        config_apply_status: faker.helpers.enumValue(ConfigApplyStatus),
        last_config_error: faker.helpers.maybe(() => faker.hacker.phrase()),
      },
      metadata: {
        id: faker.string.uuid(),
        annotations: faker.helpers.maybe(() => ({ [MachineLocked]: '' })),
        labels: {
          [LabelCluster]: clusterLabel,
          [LabelMachineSet]: machineSetLabel,
          [roleLabel]: '',
          ...faker.helpers.maybe(() => ({ [MachineStatusLabelConnected]: '' })),
          ...faker.helpers.maybe(() => ({ [UpdateLocked]: '' })),
        },
      },
    }),
    { count: { min: 0, max: 5 } },
  )
}

// Reproduces the machine IDs of every machine set in a cluster, mirroring how
// MachineSet handler derives the control-plane/worker machine-set IDs.
const clusterMachineIds = (clusterLabel: string): string[] =>
  [`${clusterLabel}-${ControlPlanesIDSuffix}`, `${clusterLabel}-${DefaultWorkersIDSuffix}`].flatMap(
    (machineSetLabel) =>
      clusterMachineStatuses(clusterLabel, machineSetLabel).map(
        (machine) => machine.metadata.id ?? '',
      ),
  )

export const Data: Story = {
  parameters: {
    msw: {
      handlers: [
        createWatchStreamHandler<ClusterStatusSpec>({
          expectedOptions: {
            type: ClusterStatusType,
            namespace: DefaultNamespace,
          },
          totalResults: 30,
          initialResources: ({ limit = 5, offset = 0 }) => {
            faker.seed(offset)
            return faker.helpers.multiple<Resource<ClusterStatusSpec>>(
              () => ({
                spec: {
                  machines: { total: 3, healthy: 3 },
                  phase: faker.helpers.enumValue(ClusterStatusSpecPhase),
                  ready: faker.datatype.boolean(),
                  talos_version: faker.system.semver(),
                  kubernetes_version: faker.system.semver(),
                },
                metadata: {
                  annotations: faker.helpers.maybe(() => ({ [ClusterLocked]: '' })),
                  id: faker.helpers
                    .slugify(faker.word.words({ count: { min: 1, max: 3 } }))
                    .toLocaleLowerCase(),
                  namespace: DefaultNamespace,
                  created: faker.date.anytime().toISOString(),
                  labels: Object.fromEntries(
                    faker.helpers.multiple(
                      () => [
                        faker.hacker.noun(),
                        faker.helpers.maybe(() => faker.hacker.adjective()) ?? '',
                      ],
                      { count: { min: 0, max: 5 } },
                    ),
                  ),
                },
              }),
              { count: limit },
            )
          },
        }).handler,

        createWatchStreamHandler<ClusterStatusMetricsSpec>({
          expectedOptions: {
            type: ClusterStatusMetricsType,
            namespace: EphemeralNamespace,
          },
          initialResources: [{ spec: { not_ready_count: 3 }, metadata: {} }],
        }).handler,

        createWatchStreamHandler<MachineSetSpec>({
          expectedOptions: {
            type: MachineSetType,
            namespace: DefaultNamespace,
          },
          initialResources: ({ selectors: { [LabelCluster]: clusterLabel = '' } = {} }) => {
            faker.seed(clusterLabel.split('').reduce((prev, curr) => prev + curr.charCodeAt(0), 0))

            return [
              {
                spec: {},
                metadata: {
                  id: `${clusterLabel}-${ControlPlanesIDSuffix}`,
                  labels: {
                    [LabelCluster]: clusterLabel,
                    [LabelControlPlaneRole]: '',
                  } as Record<string, string>,
                },
              },
              {
                spec: {},
                metadata: {
                  id: `${clusterLabel}-${DefaultWorkersIDSuffix}`,
                  labels: {
                    [LabelCluster]: clusterLabel,
                    [LabelWorkerRole]: '',
                  } as Record<string, string>,
                },
              },
            ]
          },
        }).handler,

        createWatchStreamHandler<ClusterDiagnosticsSpec>({
          expectedOptions: {
            type: ClusterDiagnosticsType,
            namespace: DefaultNamespace,
          },
          initialResources: ({ id: clusterID = '' }) => {
            const machineIds = clusterMachineIds(clusterID)

            faker.seed(seedFromId(clusterID))

            const nodes = machineIds
              .filter(() => faker.datatype.boolean())
              .map((id) => ({ id, num_diagnostics: faker.number.int({ min: 1, max: 10 }) }))

            return [{ spec: { nodes }, metadata: { id: clusterID } }]
          },
        }).handler,

        createWatchStreamHandler<MachineSetStatusSpec>({
          expectedOptions: {
            type: MachineSetStatusType,
            namespace: DefaultNamespace,
          },
          initialResources: ({ id = '' }) => {
            faker.seed(id.split('').reduce((prev, curr) => prev + curr.charCodeAt(0), 0))

            const machineCount = faker.number.int({ max: 5 })
            const healthyCount = faker.number.int({ max: machineCount })
            const label = id.includes(ControlPlanesIDSuffix)
              ? LabelControlPlaneRole
              : LabelWorkerRole

            return [
              {
                spec: {
                  phase: faker.helpers.enumValue(MachineSetPhase),
                  ready: faker.datatype.boolean(),
                  machines: { total: machineCount, healthy: healthyCount },
                  machine_allocation: faker.helpers.maybe<MachineSetSpecMachineAllocation>(() => ({
                    allocation_type: faker.helpers.enumValue(MachineSetSpecMachineAllocationType),
                    machine_count: faker.number.int({ min: 1, max: 5 }),
                    name: faker.helpers
                      .slugify(faker.word.words({ count: { min: 1, max: 3 } }))
                      .toLocaleLowerCase(),
                  })),
                  locked_updates: faker.helpers.maybe(() => faker.number.int({ min: 1, max: 10 })),
                },
                metadata: {
                  id,
                  labels: {
                    [LabelCluster]: id
                      .replace(`-${ControlPlanesIDSuffix}`, '')
                      .replace(`-${DefaultWorkersIDSuffix}`, ''),
                    [label]: '',
                  },
                },
              },
            ]
          },
        }).handler,

        createWatchStreamHandler<ClusterMachineStatusSpec>({
          expectedOptions: {
            type: ClusterMachineStatusType,
            namespace: DefaultNamespace,
          },
          initialResources: ({
            selectors: {
              [LabelCluster]: clusterLabel = '',
              [LabelMachineSet]: machineSetLabel = '',
            } = {},
          }) => clusterMachineStatuses(clusterLabel, machineSetLabel),
        }).handler,

        createWatchStreamHandler<ClusterMachineRequestStatusSpec>({
          expectedOptions: {
            type: ClusterMachineRequestStatusType,
            namespace: DefaultNamespace,
          },
          initialResources: ({ selectors: { [LabelMachineSet]: machineSetLabel = '' } = {} }) => {
            faker.seed(
              machineSetLabel.split('').reduce((prev, curr) => prev + curr.charCodeAt(0), 0),
            )

            return faker.helpers.multiple<Resource<ClusterMachineRequestStatusSpec>>(
              () => ({
                spec: {
                  machine_uuid: faker.string.uuid(),
                  ...faker.helpers.arrayElement([
                    {
                      stage: ClusterMachineRequestStatusSpecStage.PENDING,
                      status: 'Waiting for the infra provider to start provision',
                    },
                    {
                      stage: ClusterMachineRequestStatusSpecStage.DEPROVISIONING,
                      status: 'Waiting for the infra provider to finish teardown',
                    },
                    {
                      stage: ClusterMachineRequestStatusSpecStage.PROVISIONED,
                      status: 'Waiting for the machine to join Omni',
                    },
                    {
                      stage: ClusterMachineRequestStatusSpecStage.FAILED,
                      status: `Provision Failed: ${faker.hacker.phrase()}`,
                    },
                    { stage: ClusterMachineRequestStatusSpecStage.PROVISIONING },
                    { stage: ClusterMachineRequestStatusSpecStage.PENDING },
                    { stage: ClusterMachineRequestStatusSpecStage.UNKNOWN },
                  ]),
                },
                metadata: {
                  id: faker.string.uuid(),
                },
              }),
              {
                count: { min: 1, max: 5 },
              },
            )
          },
        }).handler,

        http.post<never, GetRequest>('/omni.resources.ResourceService/Get', async ({ request }) => {
          const { type, namespace } = await request.clone().json()

          if (type !== ClusterPermissionsType || namespace !== VirtualNamespace) return

          return HttpResponse.json({
            body: JSON.stringify({
              spec: {
                can_add_machines: true,
                can_remove_machines: true,
                can_reboot_machines: true,
                can_update_kubernetes: true,
                can_download_kubeconfig: true,
                can_sync_kubernetes_manifests: true,
                can_update_talos: true,
                can_download_talosconfig: true,
                can_read_config_patches: true,
                can_manage_config_patches: true,
                can_manage_cluster_features: true,
                can_download_support_bundle: true,
              },
              metadata: {
                namespace: VirtualNamespace,
                type: ClusterPermissionsType,
                id: 'talos-test-cluster',
              },
            } as Resource<ClusterPermissionsSpec>),
          })
        }),
      ],
    },
  },
}

export const NoData: Story = {
  parameters: {
    msw: {
      handlers: [createWatchStreamHandler().handler],
    },
  },
}
