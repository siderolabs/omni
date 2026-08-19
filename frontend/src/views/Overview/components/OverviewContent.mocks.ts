// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.
import { createWatchStreamHandler } from '@msw/helpers'
import { http, HttpResponse } from 'msw'

import type { Resource } from '@/api/grpc'
import type { GetRequest, GetResponse } from '@/api/omni/resources/resources.pb'
import type {
  ClusterDiagnosticsSpec,
  ClusterMachineStatusSpec,
  ClusterSecretsRotationStatusSpec,
  ClusterStatusSpec,
  ControlPlaneStatusSpec,
  EtcdBackupOverallStatusSpec,
  EtcdBackupStatusSpec,
  FeaturesConfigSpec,
  KubernetesHealthCheckStatusSpec,
  KubernetesStatusSpec,
  KubernetesUpgradeStatusSpec,
  KubernetesUsageSpec,
  MachineSetSpec,
  MachineSetStatusSpec,
  TalosUpgradeStatusSpec,
} from '@/api/omni/specs/omni.pb'
import {
  ClusterMachineStatusSpecStage,
  ClusterStatusSpecPhase,
  MachineSetPhase,
} from '@/api/omni/specs/omni.pb'
import type { ClusterPermissionsSpec, VersionContractSpec } from '@/api/omni/specs/virtual.pb'
import {
  ClusterDiagnosticsType,
  ClusterMachineStatusLabelNodeName,
  ClusterMachineStatusType,
  ClusterPermissionsType,
  ClusterSecretsRotationStatusType,
  ClusterStatusType,
  ControlPlaneStatusType,
  DefaultNamespace,
  EphemeralNamespace,
  EtcdBackupOverallStatusID,
  EtcdBackupOverallStatusType,
  EtcdBackupStatusType,
  FeaturesConfigID,
  FeaturesConfigType,
  KubernetesHealthCheckStatusType,
  KubernetesStatusType,
  KubernetesUpgradeStatusType,
  KubernetesUsageType,
  LabelCluster,
  LabelControlPlaneRole,
  LabelHostname,
  LabelMachineSet,
  LabelWorkerRole,
  MachineSetStatusType,
  MachineSetType,
  MachineStatusLabelConnected,
  MetricsNamespace,
  TalosUpgradeStatusType,
  VersionContractType,
  VirtualNamespace,
} from '@/api/resources'
import { controlPlaneMachineSetId, defaultWorkersMachineSetId } from '@/methods/machineset'

export const CLUSTER_ID = 'talos-default'
export const TALOS_VERSION = 'v1.9.1'

const CONTROL_PLANE_MACHINE_SET_ID = controlPlaneMachineSetId(CLUSTER_ID)
const WORKERS_MACHINE_SET_ID = defaultWorkersMachineSetId(CLUSTER_ID)

function createMachine(
  id: string,
  machineSetId: string,
  role: typeof LabelControlPlaneRole | typeof LabelWorkerRole,
  overrides: Partial<ClusterMachineStatusSpec> = {},
): Resource<ClusterMachineStatusSpec> {
  return {
    spec: {
      stage: ClusterMachineStatusSpecStage.RUNNING,
      ready: true,
      config_up_to_date: true,
      ...overrides,
    },
    metadata: {
      id,
      labels: {
        [LabelCluster]: CLUSTER_ID,
        [LabelMachineSet]: machineSetId,
        [role]: '',
        [LabelHostname]: id,
        [ClusterMachineStatusLabelNodeName]: id,
        [MachineStatusLabelConnected]: '',
      },
    },
  }
}

export const defaultMachines: Resource<ClusterMachineStatusSpec>[] = [
  createMachine(`${CLUSTER_ID}-cp-1`, CONTROL_PLANE_MACHINE_SET_ID, LabelControlPlaneRole),
  createMachine(`${CLUSTER_ID}-worker-1`, WORKERS_MACHINE_SET_ID, LabelWorkerRole),
  createMachine(`${CLUSTER_ID}-worker-2`, WORKERS_MACHINE_SET_ID, LabelWorkerRole),
]

export function createHandlers({
  clusterStatus,
  kubernetesUpgradeStatus,
  talosUpgradeStatus,
  secretRotationStatus,
  machines = defaultMachines,
  healthChecks = [],
}: {
  clusterStatus?: Partial<ClusterStatusSpec>
  kubernetesUpgradeStatus?: KubernetesUpgradeStatusSpec
  talosUpgradeStatus?: TalosUpgradeStatusSpec
  secretRotationStatus?: ClusterSecretsRotationStatusSpec
  machines?: Resource<ClusterMachineStatusSpec>[]
  healthChecks?: Resource<KubernetesHealthCheckStatusSpec>[]
} = {}) {
  return [
    http.post<never, GetRequest, GetResponse>(
      '/omni.resources.ResourceService/Get',
      async ({ request }) => {
        const { type, namespace } = await request.clone().json()

        if (type === ClusterPermissionsType && namespace === VirtualNamespace) {
          return HttpResponse.json({
            body: JSON.stringify({
              metadata: {},
              spec: {
                can_add_machines: true,
                can_remove_machines: true,
                can_reboot_machines: true,
                can_update_kubernetes: true,
                can_download_kubeconfig: true,
                can_update_talos: true,
                can_download_talosconfig: true,
                can_download_support_bundle: true,
                can_manage_cluster_features: true,
              } satisfies ClusterPermissionsSpec,
            } as Resource<ClusterPermissionsSpec>),
          })
        }

        if (type === VersionContractType && namespace === VirtualNamespace) {
          return HttpResponse.json({
            body: JSON.stringify({
              metadata: {},
              spec: {
                discovery_service_multidoc_config: true,
              } satisfies VersionContractSpec,
            } as Resource<VersionContractSpec>),
          })
        }
      },
    ),
    createWatchStreamHandler<FeaturesConfigSpec>({
      expectedOptions: {
        namespace: DefaultNamespace,
        type: FeaturesConfigType,
        id: FeaturesConfigID,
      },
      initialResources: [
        {
          spec: { enable_workload_proxying: true, embedded_discovery_service: true },
          metadata: { namespace: DefaultNamespace, type: FeaturesConfigType, id: FeaturesConfigID },
        },
      ],
    }).handler,
    createWatchStreamHandler<EtcdBackupOverallStatusSpec>({
      expectedOptions: {
        namespace: MetricsNamespace,
        type: EtcdBackupOverallStatusType,
        id: EtcdBackupOverallStatusID,
      },
      initialResources: [
        {
          spec: { configuration_name: 's3' },
          metadata: {
            namespace: MetricsNamespace,
            type: EtcdBackupOverallStatusType,
            id: EtcdBackupOverallStatusID,
          },
        },
      ],
    }).handler,
    createWatchStreamHandler<ClusterStatusSpec>({
      expectedOptions: { namespace: DefaultNamespace, type: ClusterStatusType, id: CLUSTER_ID },
      initialResources: [
        {
          spec: {
            phase: ClusterStatusSpecPhase.RUNNING,
            ready: true,
            available: true,
            controlplaneReady: true,
            kubernetesAPIReady: true,
            talos_version: TALOS_VERSION,
            initial_talos_version: TALOS_VERSION,
            machines: { total: 3, healthy: 3, connected: 3 },
            ...clusterStatus,
          },
          metadata: {
            namespace: DefaultNamespace,
            type: ClusterStatusType,
            id: CLUSTER_ID,
            created: new Date('2026-06-01T12:00:00Z').toISOString(),
          },
        },
      ],
    }).handler,
    createWatchStreamHandler<KubernetesUsageSpec>({
      expectedOptions: { namespace: VirtualNamespace, type: KubernetesUsageType, id: CLUSTER_ID },
      initialResources: [
        {
          spec: {
            cpu: { requests: 3.2, limits: 6.1, capacity: 8 },
            mem: { requests: 4_000_000_000, limits: 8_000_000_000, capacity: 16_000_000_000 },
            pods: { count: 42, capacity: 110 },
            storage: {
              requests: 20_000_000_000,
              limits: 50_000_000_000,
              capacity: 100_000_000_000,
            },
          },
          metadata: { namespace: VirtualNamespace, type: KubernetesUsageType, id: CLUSTER_ID },
        },
      ],
    }).handler,
    createWatchStreamHandler<KubernetesUpgradeStatusSpec>({
      expectedOptions: {
        namespace: DefaultNamespace,
        type: KubernetesUpgradeStatusType,
        id: CLUSTER_ID,
      },
      initialResources: kubernetesUpgradeStatus
        ? [
            {
              spec: kubernetesUpgradeStatus,
              metadata: {
                namespace: DefaultNamespace,
                type: KubernetesUpgradeStatusType,
                id: CLUSTER_ID,
              },
            },
          ]
        : [],
    }).handler,
    createWatchStreamHandler<TalosUpgradeStatusSpec>({
      expectedOptions: {
        namespace: DefaultNamespace,
        type: TalosUpgradeStatusType,
        id: CLUSTER_ID,
      },
      initialResources: talosUpgradeStatus
        ? [
            {
              spec: talosUpgradeStatus,
              metadata: {
                namespace: DefaultNamespace,
                type: TalosUpgradeStatusType,
                id: CLUSTER_ID,
              },
            },
          ]
        : [],
    }).handler,
    createWatchStreamHandler<ClusterSecretsRotationStatusSpec>({
      expectedOptions: {
        namespace: DefaultNamespace,
        type: ClusterSecretsRotationStatusType,
        id: CLUSTER_ID,
      },
      initialResources: secretRotationStatus
        ? [
            {
              spec: secretRotationStatus,
              metadata: {
                namespace: DefaultNamespace,
                type: ClusterSecretsRotationStatusType,
                id: CLUSTER_ID,
              },
            },
          ]
        : [],
    }).handler,
    createWatchStreamHandler<ClusterMachineStatusSpec>({
      expectedOptions: { namespace: DefaultNamespace, type: ClusterMachineStatusType },
      initialResources: ({ selectors = {} }) =>
        machines.filter((machine) =>
          Object.entries(selectors).every(
            ([key, value]) => machine.metadata.labels?.[key] === value,
          ),
        ),
    }).handler,
    createWatchStreamHandler<KubernetesHealthCheckStatusSpec>({
      expectedOptions: {
        namespace: EphemeralNamespace,
        type: KubernetesHealthCheckStatusType,
        selectors: { [LabelCluster]: CLUSTER_ID },
      },
      initialResources: healthChecks,
    }).handler,
    createWatchStreamHandler<MachineSetSpec>({
      expectedOptions: {
        namespace: DefaultNamespace,
        type: MachineSetType,
        selectors: { [LabelCluster]: CLUSTER_ID },
      },
      initialResources: [
        {
          spec: {},
          metadata: {
            id: CONTROL_PLANE_MACHINE_SET_ID,
            labels: { [LabelCluster]: CLUSTER_ID, [LabelControlPlaneRole]: '' },
          },
        },
        {
          spec: {},
          metadata: {
            id: WORKERS_MACHINE_SET_ID,
            labels: { [LabelCluster]: CLUSTER_ID, [LabelWorkerRole]: '' },
          },
        },
      ],
    }).handler,
    createWatchStreamHandler<MachineSetStatusSpec>({
      expectedOptions: { namespace: DefaultNamespace, type: MachineSetStatusType },
      initialResources: ({ id }) => {
        if (id === CONTROL_PLANE_MACHINE_SET_ID) {
          const total = machines.filter(
            (m) => m.metadata.labels?.[LabelMachineSet] === CONTROL_PLANE_MACHINE_SET_ID,
          ).length

          return [
            {
              spec: {
                phase: MachineSetPhase.Running,
                ready: true,
                machines: { total, healthy: total },
              },
              metadata: {
                id: CONTROL_PLANE_MACHINE_SET_ID,
                labels: { [LabelCluster]: CLUSTER_ID, [LabelControlPlaneRole]: '' },
              },
            },
          ]
        }

        if (id === WORKERS_MACHINE_SET_ID) {
          const total = machines.filter(
            (m) => m.metadata.labels?.[LabelMachineSet] === WORKERS_MACHINE_SET_ID,
          ).length

          return [
            {
              spec: {
                phase: MachineSetPhase.Running,
                ready: true,
                machines: { total, healthy: total },
              },
              metadata: {
                id: WORKERS_MACHINE_SET_ID,
                labels: { [LabelCluster]: CLUSTER_ID, [LabelWorkerRole]: '' },
              },
            },
          ]
        }

        return []
      },
    }).handler,
    createWatchStreamHandler<ClusterDiagnosticsSpec>({
      expectedOptions: {
        namespace: DefaultNamespace,
        type: ClusterDiagnosticsType,
        id: CLUSTER_ID,
      },
      initialResources: [],
    }).handler,
    createWatchStreamHandler<EtcdBackupStatusSpec>({
      expectedOptions: { namespace: MetricsNamespace, type: EtcdBackupStatusType, id: CLUSTER_ID },
      initialResources: [
        {
          spec: { last_backup_time: new Date('2026-08-18T02:00:00Z').toISOString() },
          metadata: { namespace: MetricsNamespace, type: EtcdBackupStatusType, id: CLUSTER_ID },
        },
      ],
    }).handler,
    createWatchStreamHandler<ControlPlaneStatusSpec>({
      expectedOptions: {
        namespace: DefaultNamespace,
        type: ControlPlaneStatusType,
        id: controlPlaneMachineSetId(CLUSTER_ID),
      },
      initialResources: [
        {
          spec: { conditions: [] },
          metadata: {
            namespace: DefaultNamespace,
            type: ControlPlaneStatusType,
            id: controlPlaneMachineSetId(CLUSTER_ID),
          },
        },
      ],
    }).handler,
    createWatchStreamHandler<KubernetesStatusSpec>({
      expectedOptions: { namespace: DefaultNamespace, type: KubernetesStatusType, id: CLUSTER_ID },
      initialResources: [
        {
          spec: {
            nodes: [
              { nodename: 'talos-default-cp-1', kubelet_version: '1.31.2', ready: true },
              { nodename: 'talos-default-worker-1', kubelet_version: '1.31.2', ready: true },
              { nodename: 'talos-default-worker-2', kubelet_version: '1.31.2', ready: true },
            ],
          },
          metadata: { namespace: DefaultNamespace, type: KubernetesStatusType, id: CLUSTER_ID },
        },
      ],
    }).handler,
  ]
}
