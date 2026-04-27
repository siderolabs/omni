<!--
Copyright (c) 2026 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import { computed, ref, watchEffect } from 'vue'

import { Runtime } from '@/api/common/omni.pb'
import type { Resource } from '@/api/grpc'
import type { ClusterSpec, ClusterStatusSpec } from '@/api/omni/specs/omni.pb'
import { ClusterStatusType, DefaultNamespace } from '@/api/resources'
import PageContainer from '@/components/PageContainer/PageContainer.vue'
import TSelectList from '@/components/SelectList/TSelectList.vue'
import { useResourceWatch } from '@/methods/useResourceWatch'
import OverviewContent from '@/views/Overview/components/OverviewContent.vue'

definePage({ name: 'ClusterOverview' })

type Props = {
  currentCluster: Resource<ClusterSpec>
}

const { currentCluster } = defineProps<Props>()

const { data: clusterStatuses } = useResourceWatch<ClusterStatusSpec>({
  runtime: Runtime.Omni,
  resource: {
    namespace: DefaultNamespace,
    type: ClusterStatusType,
  },
})

const clusters = computed(() =>
  clusterStatuses.value.map((cluster) => ({
    label: cluster.metadata.id!,
    value: cluster.metadata.id!,
  })),
)

const selectedCluster = ref<string>()

watchEffect(() => {
  selectedCluster.value = currentCluster.metadata.id
})
</script>

<template>
  <PageContainer class="flex flex-col gap-6">
    <div role="heading" aria-level="1" :aria-label="selectedCluster">
      <TSelectList
        v-model="selectedCluster"
        variant="breadcrumb"
        :values="clusters"
        searcheable
        @update:model-value="(v) => $router.push({ params: { cluster: v } })"
      />
    </div>

    <OverviewContent :current-cluster="currentCluster" />
  </PageContainer>
</template>
