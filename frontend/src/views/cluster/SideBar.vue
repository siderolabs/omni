<!--
Copyright (c) 2025 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import type { Ref } from 'vue'
import { computed, ref } from 'vue'
import { useRoute } from 'vue-router'

import { Runtime } from '@/api/common/omni.pb'
import type { Resource } from '@/api/grpc'
import type { ClusterSpec, KubernetesUpgradeManifestStatusSpec } from '@/api/omni/specs/omni.pb'
import { ClusterType, DefaultNamespace, KubernetesUpgradeManifestStatusType } from '@/api/resources'
import Watch from '@/api/watch'
import type { SideBarItem } from '@/components/SideBar/TSideBarList.vue'
import { default as TSidebarList } from '@/components/SideBar/TSideBarList.vue'
import { setupClusterPermissions } from '@/methods/auth'
import { setupWorkloadProxyingEnabledFeatureWatch } from '@/methods/features'
import ExposedServiceSideBar from '@/views/cluster/ExposedService/ExposedServiceSideBar.vue'
import OmniSideBar from '@/views/omni/SideBar.vue'

const route = useRoute()

const getRoute = (path: string) => `/cluster/${route.params.cluster}${path}`

const kubernetesUpgradeManifestStatus: Ref<
  Resource<KubernetesUpgradeManifestStatusSpec> | undefined
> = ref()
const kubernetesUpgradeManifestStatusWatch = new Watch(kubernetesUpgradeManifestStatus)

kubernetesUpgradeManifestStatusWatch.setup({
  runtime: Runtime.Omni,
  resource: {
    namespace: DefaultNamespace,
    type: KubernetesUpgradeManifestStatusType,
    id: route.params.cluster as string,
  },
})

const pendingManifests = computed(() => {
  const pending = kubernetesUpgradeManifestStatus?.value?.spec.last_fatal_error
    ? '!'
    : kubernetesUpgradeManifestStatus?.value?.spec.out_of_sync

  return pending === undefined || pending === 0 ? undefined : pending
})

const { canSyncKubernetesManifests, canManageClusterFeatures } = setupClusterPermissions(
  computed(() => route.params.cluster as string),
)

const cluster: Ref<Resource<ClusterSpec> | undefined> = ref()
const clusterWatch = new Watch(cluster)
clusterWatch.setup({
  runtime: Runtime.Omni,
  resource: {
    namespace: DefaultNamespace,
    type: ClusterType,
    id: route.params.cluster as string,
  },
})

const items = computed(() => {
  const result: SideBarItem[] = [
    {
      name: 'Overview',
      route: getRoute('/overview'),
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
      labelColor: pendingManifests.value === '!' ? 'red-R1' : undefined,
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

const workloadProxyingEnabled = setupWorkloadProxyingEnabledFeatureWatch()
</script>

<template>
  <div class="flex-1">
    <OmniSideBar class="border-b border-naturals-N4" />
    <p class="mb-2 mt-5 px-6 text-xs text-naturals-N8">Cluster</p>
    <p class="truncate px-6 text-xs text-naturals-N13">
      {{ $route.params.cluster }}
    </p>
    <TSidebarList :items="items" />

    <ExposedServiceSideBar
      v-if="workloadProxyingEnabled && cluster?.spec?.features?.enable_workload_proxy"
    />
  </div>
</template>
