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
import { useResourceWatch } from '@/methods/useResourceWatch'
import ClusterManifestsStatusGraph from '@/views/Clusters/ClusterManifestsStatusGraph.vue'
import ClusterManifestsStatusList from '@/views/Clusters/ClusterManifestsStatusList.vue'
import ClusterManifestsQuickStart from '@/views/Clusters/components/ClusterManifestsQuickStart.vue'

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
        <StatsItem title="In Sync" :value="inSyncCount" icon="check-circle" />
        <StatsItem
          v-if="manifestsStatus.spec.out_of_sync"
          title="Out of Sync"
          :value="manifestsStatus.spec.out_of_sync"
          icon="exclamation-triangle"
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

    <ClusterManifestsQuickStart v-else-if="!hasManifests" />

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
