// Copyright (c) 2025 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.
import { createWatchStreamHandler } from '@msw/helpers'
import type { Meta, StoryObj } from '@storybook/vue3-vite'
import { http, HttpResponse } from 'msw'
import { vueRouter } from 'storybook-vue3-router'
import { RouterView } from 'vue-router'

import type { GetRequest } from '@/api/omni/resources/resources.pb'
import type { InfraProviderStatusSpec } from '@/api/omni/specs/infra.pb'
import type {
  ClusterSpec,
  EtcdBackupOverallStatusSpec,
  FeaturesConfigSpec,
  KubernetesUpgradeManifestStatusSpec,
  MachineStatusMetricsSpec,
} from '@/api/omni/specs/omni.pb'
import {
  ClusterMachineIdentityType,
  ClusterPermissionsType,
  ClusterType,
  DefaultNamespace,
  EphemeralNamespace,
  EtcdBackupOverallStatusType,
  FeaturesConfigType,
  InfraProviderNamespace,
  InfraProviderStatusType,
  KubernetesUpgradeManifestStatusType,
  MachineStatusMetricsID,
  MachineStatusMetricsType,
  MetricsNamespace,
  TalosRuntimeNamespace,
  TalosServiceType,
  VirtualNamespace,
} from '@/api/resources'
import { routes } from '@/router'

import TSideBar from './TSideBar.vue'

const meta: Meta<typeof TSideBar> = {
  component: TSideBar,
}

export default meta
type Story = StoryObj<typeof meta>

export const Default: Story = {
  decorators: [
    vueRouter([...routes, { path: '/fake/:cluster/:machine', component: RouterView }], {
      initialRoute: '/fake/my-cluster/my-machine',
    }),
  ],
  parameters: {
    msw: {
      handlers: [
        createWatchStreamHandler<EtcdBackupOverallStatusSpec>({
          expectedOptions: {
            namespace: MetricsNamespace,
            type: EtcdBackupOverallStatusType,
          },
          initialResources: [
            {
              spec: { configuration_name: 's3', configuration_error: 'not initialized' },
              metadata: {
                created: '2025-10-17T16:06:40Z',
                updated: '2025-10-17T16:06:40Z',
                namespace: 'metrics',
                type: 'EtcdBackupOverallStatuses.omni.sidero.dev',
                id: 'etcdbackup-overall-status',
                owner: 'EtcdBackupOverallStatusController',
                phase: 'running',
                version: '1',
              },
            },
          ],
        }).handler,

        createWatchStreamHandler<MachineStatusMetricsSpec>({
          expectedOptions: {
            namespace: EphemeralNamespace,
            type: MachineStatusMetricsType,
            id: MachineStatusMetricsID,
          },
          initialResources: [
            {
              spec: {
                registered_machines_count: 3,
                connected_machines_count: 3,
                allocated_machines_count: 3,
                versions_map: { 'v1.11.3': 3 },
                platforms: {
                  akamai: 0,
                  aws: 0,
                  azure: 0,
                  cloudstack: 0,
                  'digital-ocean': 0,
                  equinixMetal: 0,
                  exoscale: 0,
                  gcp: 0,
                  hcloud: 0,
                  metal: 3,
                  nocloud: 0,
                  opennebula: 0,
                  openstack: 0,
                  oracle: 0,
                  scaleway: 0,
                  upcloud: 0,
                  vmware: 0,
                  vultr: 0,
                },
                secure_boot_status: { false: 3, true: 0, unknown: 0 },
                uki_status: { false: 0, true: 3, unknown: 0 },
              },
              metadata: {
                created: '2025-10-23T08:26:35Z',
                updated: '2025-10-23T08:29:07Z',
                namespace: 'ephemeral',
                type: 'MachineStatusMetrics.omni.sidero.dev',
                id: 'metrics',
                owner: 'MachineStatusMetricsController',
                phase: 'running',
                version: '8',
              },
            },
          ],
        }).handler,

        createWatchStreamHandler<InfraProviderStatusSpec>({
          expectedOptions: {
            namespace: InfraProviderNamespace,
            type: InfraProviderStatusType,
          },
          initialResources: [],
        }).handler,

        createWatchStreamHandler<KubernetesUpgradeManifestStatusSpec>({
          expectedOptions: {
            namespace: DefaultNamespace,
            type: KubernetesUpgradeManifestStatusType,
          },
          initialResources: [
            {
              spec: {},
              metadata: {
                created: '2025-10-23T08:27:27Z',
                updated: '2025-10-23T08:31:19Z',
                namespace: 'default',
                type: 'KubernetesUpgradeManifestStatuses.omni.sidero.dev',
                id: 'talos-default',
                owner: 'KubernetesUpgradeManifestStatusController',
                phase: 'running',
                annotations: {
                  inputResourceVersion:
                    'cbf12956c05033603551a2b97ab47f83f7548145dc8bd64ea0a76e9ffac087b7',
                },
                version: '2',
              },
            },
          ],
        }).handler,

        createWatchStreamHandler<ClusterSpec>({
          expectedOptions: {
            namespace: DefaultNamespace,
            type: ClusterType,
          },
          initialResources: [
            {
              spec: { kubernetes_version: '1.34.1', talos_version: '1.11.3', features: {} },
              metadata: {
                created: '2025-10-23T08:27:26Z',
                updated: '2025-10-23T08:27:26Z',
                namespace: 'default',
                type: 'Clusters.omni.sidero.dev',
                id: 'talos-default',
                owner: '',
                phase: 'running',
                finalizers: [
                  'TalosUpgradeStatusController',
                  'ClusterController',
                  'BackupDataController',
                  'SecretsController',
                  'KubernetesUpgradeStatusController',
                  'ClusterUUIDController',
                  'ClusterConfigVersionController',
                  'ClusterWorkloadProxyStatusController',
                  'ClusterEndpointController',
                  'ClusterDestroyStatusController',
                  'ClusterStatusController',
                ],
                version: '12',
              },
            },
          ],
        }).handler,

        createWatchStreamHandler({
          expectedOptions: {
            type: TalosServiceType,
            namespace: TalosRuntimeNamespace,
          },
          initialResources: [
            {
              spec: { healthy: true, running: true, unknown: false },
              metadata: {
                created: '2025-10-23T08:28:35Z',
                updated: '2025-10-23T08:28:36Z',
                namespace: 'runtime',
                type: 'Services.v1alpha1.talos.dev',
                id: 'apid',
                owner: 'v1alpha1.ServiceController',
                phase: 'running',
                version: '2',
              },
            },
            {
              spec: { healthy: true, running: true, unknown: false },
              metadata: {
                created: '2025-10-23T08:28:32Z',
                updated: '2025-10-23T08:28:33Z',
                namespace: 'runtime',
                type: 'Services.v1alpha1.talos.dev',
                id: 'auditd',
                owner: 'v1alpha1.ServiceController',
                phase: 'running',
                version: '2',
              },
            },
            {
              spec: { healthy: true, running: true, unknown: false },
              metadata: {
                created: '2025-10-23T08:28:32Z',
                updated: '2025-10-23T08:28:33Z',
                namespace: 'runtime',
                type: 'Services.v1alpha1.talos.dev',
                id: 'containerd',
                owner: 'v1alpha1.ServiceController',
                phase: 'running',
                version: '2',
              },
            },
            {
              spec: { healthy: true, running: true, unknown: false },
              metadata: {
                created: '2025-10-23T08:28:35Z',
                updated: '2025-10-23T08:28:36Z',
                namespace: 'runtime',
                type: 'Services.v1alpha1.talos.dev',
                id: 'cri',
                owner: 'v1alpha1.ServiceController',
                phase: 'running',
                version: '2',
              },
            },
            {
              spec: { healthy: false, running: true, unknown: true },
              metadata: {
                created: '2025-10-23T08:28:35Z',
                updated: '2025-10-23T08:28:35Z',
                namespace: 'runtime',
                type: 'Services.v1alpha1.talos.dev',
                id: 'dashboard',
                owner: 'v1alpha1.ServiceController',
                phase: 'running',
                version: '1',
              },
            },
            {
              spec: { healthy: true, running: true, unknown: false },
              metadata: {
                created: '2025-10-23T08:28:59Z',
                updated: '2025-10-23T08:29:04Z',
                namespace: 'runtime',
                type: 'Services.v1alpha1.talos.dev',
                id: 'etcd',
                owner: 'v1alpha1.ServiceController',
                phase: 'running',
                version: '2',
              },
            },
            {
              spec: { healthy: false, running: true, unknown: true },
              metadata: {
                created: '2025-10-23T08:28:33Z',
                updated: '2025-10-23T08:28:33Z',
                namespace: 'runtime',
                type: 'Services.v1alpha1.talos.dev',
                id: 'ext-hello-world',
                owner: 'v1alpha1.ServiceController',
                phase: 'running',
                version: '1',
              },
            },
            {
              spec: { healthy: true, running: true, unknown: false },
              metadata: {
                created: '2025-10-23T08:28:43Z',
                updated: '2025-10-23T08:28:45Z',
                namespace: 'runtime',
                type: 'Services.v1alpha1.talos.dev',
                id: 'kubelet',
                owner: 'v1alpha1.ServiceController',
                phase: 'running',
                version: '2',
              },
            },
            {
              spec: { healthy: true, running: true, unknown: false },
              metadata: {
                created: '2025-10-23T08:28:32Z',
                updated: '2025-10-23T08:28:33Z',
                namespace: 'runtime',
                type: 'Services.v1alpha1.talos.dev',
                id: 'machined',
                owner: 'v1alpha1.ServiceController',
                phase: 'running',
                version: '2',
              },
            },
            {
              spec: { healthy: true, running: true, unknown: false },
              metadata: {
                created: '2025-10-23T08:28:33Z',
                updated: '2025-10-23T08:28:34Z',
                namespace: 'runtime',
                type: 'Services.v1alpha1.talos.dev',
                id: 'syslogd',
                owner: 'v1alpha1.ServiceController',
                phase: 'running',
                version: '2',
              },
            },
            {
              spec: { healthy: true, running: true, unknown: false },
              metadata: {
                created: '2025-10-23T08:28:35Z',
                updated: '2025-10-23T08:28:36Z',
                namespace: 'runtime',
                type: 'Services.v1alpha1.talos.dev',
                id: 'trustd',
                owner: 'v1alpha1.ServiceController',
                phase: 'running',
                version: '2',
              },
            },
            {
              spec: { healthy: true, running: true, unknown: false },
              metadata: {
                created: '2025-10-23T08:28:32Z',
                updated: '2025-10-23T08:28:33Z',
                namespace: 'runtime',
                type: 'Services.v1alpha1.talos.dev',
                id: 'udevd',
                owner: 'v1alpha1.ServiceController',
                phase: 'running',
                version: '2',
              },
            },
          ],
        }).handler,

        createWatchStreamHandler<FeaturesConfigSpec>({
          expectedOptions: {
            type: FeaturesConfigType,
            namespace: DefaultNamespace,
          },
          initialResources: [
            {
              spec: {
                enable_workload_proxying: true,
                etcd_backup_settings: {
                  tick_interval: '60s',
                  min_interval: '3600s',
                  max_interval: '86400s',
                },
                embedded_discovery_service: true,
                audit_log_enabled: true,
                image_factory_base_url: 'https://factory.talos.dev',
                user_pilot_settings: {},
                stripe_settings: {},
              },
              metadata: {
                created: '2025-10-17T16:06:40Z',
                updated: '2025-10-17T16:06:40Z',
                namespace: 'default',
                type: 'FeaturesConfigs.omni.sidero.dev',
                id: 'features-config',
                owner: '',
                phase: 'running',
                version: '1',
              },
            },
          ],
        }).handler,

        http.post<never, GetRequest>('/omni.resources.ResourceService/Get', async ({ request }) => {
          const { type, namespace } = await request.clone().json()

          if (type !== ClusterPermissionsType || namespace !== VirtualNamespace) return

          return HttpResponse.json(
            {
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
                  created: '2025-10-23T09:34:12Z',
                  updated: '2025-10-23T09:34:12Z',
                  namespace: 'virtual',
                  type: 'ClusterPermissions.omni.sidero.dev',
                  id: 'talos-default',
                  owner: '',
                  phase: 'running',
                  version: '1',
                },
              }),
            },
            {
              headers: {
                'content-type': 'application/json',
                'Grpc-metadata-content-type': 'application/grpc',
              },
            },
          )
        }),

        http.post<never, GetRequest>('/omni.resources.ResourceService/Get', async ({ request }) => {
          const { type, namespace } = await request.clone().json()

          if (type !== ClusterMachineIdentityType || namespace !== DefaultNamespace) return

          return HttpResponse.json(
            {
              body: JSON.stringify({
                spec: {
                  node_identity: 'xdwlmW302gaDjm7JaSQ4ZF6c67gAciYOZBsTZzkVoVE',
                  etcd_member_id: '11861357848454170587',
                  nodename: 'machine-cdecae49-a354-4931-bd7c-e4ca18410f10',
                  node_ips: ['172.20.0.2'],
                  discovery_service_endpoint: 'https://discovery.talos.dev:443',
                },
                metadata: {
                  created: '2025-10-23T08:28:43Z',
                  updated: '2025-10-23T08:29:04Z',
                  namespace: 'default',
                  type: 'ClusterMachineIdentities.omni.sidero.dev',
                  id: 'cdecae49-a354-4931-bd7c-e4ca18410f10',
                  owner: 'ClusterMachineIdentityController',
                  phase: 'running',
                  labels: {
                    'omni.sidero.dev/cluster': 'talos-default',
                    'omni.sidero.dev/machine-set': 'talos-default-control-planes',
                    'omni.sidero.dev/role-controlplane': '',
                  },
                  version: 5,
                },
              }),
            },
            {
              headers: {
                'content-type': 'application/json',
                'Grpc-metadata-content-type': 'application/grpc',
              },
            },
          )
        }),
      ],
    },
  },
}
