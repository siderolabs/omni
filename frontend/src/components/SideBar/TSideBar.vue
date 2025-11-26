<!--
Copyright (c) 2025 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useRoute } from 'vue-router'

import { Runtime } from '@/api/common/omni.pb'
import { type Resource, ResourceService } from '@/api/grpc'
import type { InfraProviderStatusSpec } from '@/api/omni/specs/infra.pb'
import type {
  ClusterMachineIdentitySpec,
  ClusterSpec,
  KubernetesUpgradeManifestStatusSpec,
  MachineStatusMetricsSpec,
} from '@/api/omni/specs/omni.pb'
import { withRuntime } from '@/api/options'
import {
  ClusterMachineIdentityType,
  ClusterType,
  DefaultNamespace,
  EphemeralNamespace,
  InfraProviderNamespace,
  InfraProviderStatusType,
  KubernetesUpgradeManifestStatusType,
  MachineStatusMetricsID,
  MachineStatusMetricsType,
  TalosRuntimeNamespace,
  TalosServiceType,
} from '@/api/resources'
import UserInfo from '@/components/common/UserInfo/UserInfo.vue'
import type { SideBarItem } from '@/components/SideBar/TSideBarList.vue'
import TSidebarList from '@/components/SideBar/TSideBarList.vue'
import { getContext } from '@/context'
import { setupBackupStatus } from '@/methods'
import {
  canManageBackupStore,
  canManageUsers,
  canReadClusters,
  canReadMachines,
  setupClusterPermissions,
} from '@/methods/auth'
import { useFeatures, useInstallationMediaEnabled } from '@/methods/features'
import { useIdentity } from '@/methods/identity'
import { useResourceWatch } from '@/methods/useResourceWatch'
import ExposedServiceSideBar from '@/views/cluster/ExposedService/ExposedServiceSideBar.vue'

const route = useRoute()
const context = getContext()
const { avatar, fullname, identity } = useIdentity()

const { data: featuresConfig } = useFeatures()
const { value: installationMediaEnabled } = useInstallationMediaEnabled()

const { status: backupStatus } = setupBackupStatus()
const { canSyncKubernetesManifests, canManageClusterFeatures } = setupClusterPermissions(
  computed(() => route.params.cluster as string),
)

const { data: machineMetrics } = useResourceWatch<MachineStatusMetricsSpec>({
  resource: {
    namespace: EphemeralNamespace,
    type: MachineStatusMetricsType,
    id: MachineStatusMetricsID,
  },
  runtime: Runtime.Omni,
})

const { data: infraProviderStatuses } = useResourceWatch<InfraProviderStatusSpec>(() => ({
  skip: !canReadMachines.value,
  resource: {
    namespace: InfraProviderNamespace,
    type: InfraProviderStatusType,
  },
  runtime: Runtime.Omni,
}))

const { data: kubernetesUpgradeManifestStatus } =
  useResourceWatch<KubernetesUpgradeManifestStatusSpec>(() => ({
    skip: !route.params.cluster,
    runtime: Runtime.Omni,
    resource: {
      namespace: DefaultNamespace,
      type: KubernetesUpgradeManifestStatusType,
      id: route.params.cluster as string,
    },
  }))

const { data: cluster } = useResourceWatch<ClusterSpec>(() => ({
  skip: !route.params.cluster,
  runtime: Runtime.Omni,
  resource: {
    namespace: DefaultNamespace,
    type: ClusterType,
    id: route.params.cluster as string,
  },
}))

const { data: services } = useResourceWatch(() => ({
  skip: !route.params.machine,
  resource: {
    type: TalosServiceType,
    namespace: TalosRuntimeNamespace,
  },
  runtime: Runtime.Talos,
  context,
}))

const node = ref<string>()

watch(
  () => route.params.machine as string,
  async (machineId) => {
    if (!machineId) {
      node.value = undefined
      return
    }

    const nodename = await ResourceService.Get<Resource<ClusterMachineIdentitySpec>>(
      {
        type: ClusterMachineIdentityType,
        id: machineId,
        namespace: DefaultNamespace,
      },
      withRuntime(Runtime.Omni),
    )

    node.value = nodename.spec.nodename
  },
  { immediate: true },
)

const groupProviders = (statuses: Resource<InfraProviderStatusSpec>[]) =>
  statuses.reduce<Record<string, typeof statuses>>((res, status) => {
    res[status.spec.name!] ||= []
    res[status.spec.name!].push(status)

    return res
  }, {})

const pendingManifests = computed(() => {
  if (kubernetesUpgradeManifestStatus.value?.spec.last_fatal_error) return '!'
  if (!kubernetesUpgradeManifestStatus.value?.spec.out_of_sync) return undefined

  return kubernetesUpgradeManifestStatus.value.spec.out_of_sync
})

const rootItems = computed(() => {
  const getRoute = (name: string, path: string) =>
    route.query.cluster
      ? {
          name: name,
          query: {
            cluster: route.query.cluster,
            namespace: route.query.namespace,
            uid: route.query.uid,
          },
        }
      : path

  const result: SideBarItem[] = [
    {
      name: 'Home',
      route: getRoute('Home', '/'),
      icon: 'home',
    },
  ]

  if (canReadClusters.value) {
    result.push({
      name: 'Clusters',
      route: getRoute('Clusters', '/clusters'),
      icon: 'clusters',
    })
  }

  if (canReadMachines.value) {
    const autoprovisionedMenuItem: SideBarItem = {
      name: 'Auto-Provisioned',
      icon: 'machines-autoprovisioned',
      route: getRoute('MachinesManaged', '/machines/managed'),
    }

    if (infraProviderStatuses.value.length > 0) {
      autoprovisionedMenuItem.subItems = []

      const items = groupProviders(infraProviderStatuses.value)

      for (const name in items) {
        const values = items[name]

        const item: SideBarItem = {
          name: name,
          iconSvgBase64: values[0].spec.icon,
        }

        if (!item.iconSvgBase64) {
          item.icon = 'cloud-connection'
        }

        if (values.length > 1) {
          item.subItems = []

          for (const provider of values) {
            item.subItems.push({
              name: provider.metadata.id!,
              route: getRoute(
                'MachinesManagedProvider',
                `/machines/managed/${provider.metadata.id!}`,
              ),
            })
          }
        } else {
          item.route = getRoute(
            'MachinesManagedProvider',
            `/machines/managed/${values[0].metadata.id}`,
          )
        }

        autoprovisionedMenuItem.subItems.push(item)
      }
    }

    const item = {
      name: 'Machines',
      route: getRoute('Machines', '/machines'),
      icon: 'nodes',
      subItems: [
        {
          name: 'Self-Managed',
          route: getRoute('MachinesManual', '/machines/manual'),
          icon: 'machines-manual',
        },
        autoprovisionedMenuItem,
      ],
    } satisfies SideBarItem

    if (machineMetrics.value?.spec.pending_machines_count) {
      item.subItems.push({
        name: 'Pending',
        route: getRoute('MachinesPending', '/machines/pending'),
        icon: 'question',
        label: machineMetrics.value.spec.pending_machines_count,
      })
    }

    result.push(item)
  }

  if (canReadMachines.value) {
    const item = {
      name: 'Machine Management',
      icon: 'nodes',
      subItems: [
        {
          name: 'Classes',
          route: getRoute('MachineClasses', '/machine-classes'),
          icon: 'code-bracket',
        },
        {
          name: 'Join Tokens',
          route: getRoute('JoinTokens', '/machines/jointokens'),
          icon: 'key',
        },
      ] as SideBarItem[],
    } satisfies SideBarItem

    if (installationMediaEnabled) {
      item.subItems.push({
        name: 'Installation Media',
        route: getRoute('InstallationMedia', '/machines/installation-media'),
        icon: 'kube-config',
      })
    }

    result.push(item)
  }

  if (canManageUsers.value || (backupStatus.value.configurable && canManageBackupStore.value)) {
    const subItems: SideBarItem[] = [
      {
        name: 'Users',
        route: getRoute('Users', '/settings/users'),
        icon: 'users',
      },
      {
        name: 'Service Accounts',
        route: getRoute('ServiceAccounts', '/settings/serviceaccounts'),
        icon: 'users',
      },
      {
        name: 'Infra Providers',
        route: getRoute('InfraProviders', '/settings/infraproviders'),
        icon: 'machines-autoprovisioned',
      },
      {
        name: 'Backups',
        route: getRoute('Backups', '/settings/backups'),
        icon: 'rollback',
      },
    ]

    if (featuresConfig.value?.spec.stripe_settings?.enabled) {
      subItems.push({
        name: 'Stripe',
        route: 'https://billing.stripe.com/p/login/8wMcOC8z51GgdPi144',
        regularLink: true,
        icon: 'dashboard',
      })
    }

    result.push({
      name: 'Settings',
      icon: 'settings',
      subItems,
    })
  }

  return result
})

const clusterItems = computed(() => {
  const getRoute = (path: string) => `/clusters/${route.params.cluster}${path}`

  const result: SideBarItem[] = [
    {
      name: 'Overview',
      route: getRoute(''),
      icon: 'overview',
    },
    {
      name: 'Nodes',
      route: getRoute('/nodes'),
      icon: 'nodes',
    },
    {
      name: 'Pods',
      route: getRoute('/pods'),
      icon: 'pods',
    },
    {
      name: 'Config Patches',
      route: getRoute('/patches'),
      icon: 'settings',
    },
  ]

  if (canSyncKubernetesManifests.value) {
    result.push({
      name: 'Bootstrap Manifests',
      route: getRoute('/manifests'),
      icon: 'bootstrap-manifests',
      label: pendingManifests.value,
      labelColor: pendingManifests.value === '!' ? 'red-r1' : undefined,
    })
  }

  if (canManageClusterFeatures.value) {
    result.push({
      name: 'Backups',
      route: getRoute('/backups'),
      icon: 'rollback',
    })
  }

  return result
})

const nodeItems = computed(() =>
  ['controller-runtime', ...services.value.map((item) => item.metadata.id!)].map<SideBarItem>(
    (service) => ({
      name: service,
      route: {
        name: 'NodeLogs',
        params: {
          machine: route.params.machine as string,
          service: service,
        },
      },
    }),
  ),
)
</script>

<template>
  <aside class="flex flex-col border-r border-naturals-n4 bg-naturals-n1">
    <div class="grow overflow-auto">
      <TSidebarList :items="rootItems" />

      <div v-if="$route.params.cluster" class="border-t border-naturals-n4">
        <p class="mt-5 mb-2 px-6 text-xs text-naturals-n8">Cluster</p>
        <p class="truncate px-6 text-xs text-naturals-n13">
          {{ $route.params.cluster }}
        </p>

        <TSidebarList :items="clusterItems" />

        <ExposedServiceSideBar
          v-if="
            featuresConfig?.spec.enable_workload_proxying &&
            cluster?.spec.features?.enable_workload_proxy
          "
        />

        <div v-if="$route.params.machine" class="border-t border-naturals-n4">
          <p class="mt-5 mb-2 px-6 text-xs text-naturals-n8">Node</p>
          <p class="truncate px-6 text-xs text-naturals-n13">{{ node }}</p>
          <TSidebarList :items="nodeItems" />
        </div>
      </div>
    </div>

    <UserInfo
      class="h-16 shrink-0 border-t border-inherit px-2"
      with-logout-controls
      size="small"
      :avatar="avatar"
      :fullname="fullname"
      :email="identity"
    />
  </aside>
</template>
