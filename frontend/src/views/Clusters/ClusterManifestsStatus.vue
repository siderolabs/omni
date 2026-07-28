<!--
Copyright (c) 2026 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import { useRouteHash } from '@vueuse/router'
import { computed, type Ref } from 'vue'

import { Runtime } from '@/api/common/omni.pb'
import type { ClusterKubernetesManifestsStatusSpec } from '@/api/omni/specs/omni.pb'
import { ClusterKubernetesManifestsStatusType, DefaultNamespace } from '@/api/resources'
import TIcon from '@/components/Icon/TIcon.vue'
import PageContainer from '@/components/PageContainer/PageContainer.vue'
import PageHeader from '@/components/PageHeader.vue'
import TSpinner from '@/components/Spinner/TSpinner.vue'
import StatsItem from '@/components/Stats/StatsItem.vue'
import TabButton from '@/components/Tabs/TabButton.vue'
import TabContent from '@/components/Tabs/TabContent.vue'
import Tabs from '@/components/Tabs/Tabs.vue'
import TAlert from '@/components/TAlert.vue'
import { getDocsLink } from '@/methods'
import { useResourceWatch } from '@/methods/useResourceWatch'
import ClusterManifestsStatusGraph from '@/views/Clusters/ClusterManifestsStatusGraph.vue'
import ClusterManifestsStatusList from '@/views/Clusters/ClusterManifestsStatusList.vue'

enum TabType {
  GRAPH = '#graph',
  LIST = '#list',
}

const { cluster } = defineProps<{
  cluster: string
}>()

const routeHash = useRouteHash(TabType.GRAPH) as Ref<TabType>

const {
  data: manifestsStatus,
  loading: manifestsStatusLoading,
  err: manifestsStatusErr,
} = useResourceWatch<ClusterKubernetesManifestsStatusSpec>(() => ({
  runtime: Runtime.Omni,
  resource: {
    namespace: DefaultNamespace,
    type: ClusterKubernetesManifestsStatusType,
    id: cluster,
  },
}))

const inSyncCount = computed(
  () => (manifestsStatus.value?.spec.total ?? 0) - (manifestsStatus.value?.spec.out_of_sync ?? 0),
)

const hasManifests = computed(() => Object.keys(manifestsStatus.value?.spec.groups ?? {}).length)
</script>

<template>
  <PageContainer class="@container flex h-full flex-col">
    <PageHeader :title="`Manifests Status — ${cluster}`">
      <template v-if="manifestsStatus && hasManifests">
        <StatsItem title="Total" :value="manifestsStatus.spec.total ?? 0" icon="document-text" />
        <StatsItem title="In Sync" :value="inSyncCount" icon="check-in-circle" />
        <StatsItem
          v-if="manifestsStatus.spec.out_of_sync"
          title="Out of Sync"
          :value="manifestsStatus.spec.out_of_sync"
          icon="warning"
        />
      </template>
    </PageHeader>

    <div v-if="manifestsStatusLoading" class="flex h-40 items-center justify-center">
      <TSpinner class="h-6 w-6" />
    </div>

    <TAlert v-else-if="manifestsStatusErr" title="Error" type="error">
      {{ manifestsStatusErr }}
    </TAlert>

    <TAlert v-else-if="manifestsStatus?.spec.last_error" title="Manifest Error" type="error">
      {{ manifestsStatus.spec.last_error }}
    </TAlert>

    <div
      v-else-if="!hasManifests"
      class="@container flex grow flex-col items-center justify-center overflow-y-auto"
    >
      <div class="flex flex-col items-center gap-3 text-center">
        <div class="flex size-14 items-center justify-center rounded-full bg-naturals-n3">
          <TIcon icon="document-text" class="size-7 text-primary-p3" />
        </div>

        <h1 class="text-xl font-medium text-naturals-n14">No Manifest Groups</h1>

        <p class="max-w-xl text-sm text-naturals-n11">
          Omni tracks the Kubernetes bootstrap manifests it manages for this cluster — things like
          CNI, CSI, and other add-ons — and reports whether the cluster's live state matches what
          was applied. No manifest groups have been reported for this cluster yet.
        </p>

        <a
          :href="getDocsLink('omni', '/cluster-management/sync-kubernetes-manifests')"
          target="_blank"
          rel="noopener noreferrer"
          class="link-primary inline-flex items-center gap-1 text-sm"
        >
          Learn more about syncing Kubernetes manifests
          <TIcon icon="external-link" class="size-3.5" />
        </a>
      </div>

      <div class="grid max-w-3xl gap-3 py-6 @2xl:grid-cols-3">
        <div class="flex flex-col items-center gap-2 rounded-lg bg-naturals-n2 p-4 text-center">
          <TIcon icon="pods" class="size-5 text-naturals-n13" />
          <h3 class="text-sm font-medium text-naturals-n14">Grouped manifests</h3>
          <p class="text-xs text-naturals-n11">
            Related Kubernetes objects are grouped so you can see the state of each bootstrap
            component at a glance.
          </p>
        </div>

        <div class="flex flex-col items-center gap-2 rounded-lg bg-naturals-n2 p-4 text-center">
          <TIcon icon="check-in-circle" class="size-5 text-naturals-n13" />
          <h3 class="text-sm font-medium text-naturals-n14">Sync tracking</h3>
          <p class="text-xs text-naturals-n11">
            Each object is compared against the cluster's live state to detect drift from what Omni
            applied.
          </p>
        </div>

        <div class="flex flex-col items-center gap-2 rounded-lg bg-naturals-n2 p-4 text-center">
          <TIcon icon="list-bullet" class="size-5 text-naturals-n13" />
          <h3 class="text-sm font-medium text-naturals-n14">Graph &amp; list views</h3>
          <p class="text-xs text-naturals-n11">
            Once available, manifests can be explored as a dependency graph or as a flat, filterable
            list.
          </p>
        </div>
      </div>
    </div>

    <Tabs
      v-else-if="manifestsStatus"
      v-model="routeHash"
      tabs-list-class="mb-2"
      class="grow overflow-y-hidden"
    >
      <template #triggers>
        <TabButton class="flex items-center gap-1" :value="TabType.GRAPH">
          <TIcon icon="pods" aria-hidden="true" class="size-4" />
          Graph
        </TabButton>

        <TabButton class="flex items-center gap-1" :value="TabType.LIST">
          <TIcon icon="list-bullet" aria-hidden="true" class="size-4" />
          List
        </TabButton>
      </template>

      <template #contents>
        <TabContent class="grow overflow-y-auto" :value="TabType.GRAPH">
          <ClusterManifestsStatusGraph class="h-full" :manifests-status />
        </TabContent>

        <TabContent class="grow overflow-y-auto" :value="TabType.LIST">
          <ClusterManifestsStatusList class="h-full" :manifests-status />
        </TabContent>
      </template>
    </Tabs>
  </PageContainer>
</template>
