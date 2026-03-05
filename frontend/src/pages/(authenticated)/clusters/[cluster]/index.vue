<!--
Copyright (c) 2026 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import type { Resource } from '@/api/grpc'
import type { ClusterSpec } from '@/api/omni/specs/omni.pb'
import PageContainer from '@/components/common/PageContainer/PageContainer.vue'
import PageHeader from '@/components/common/PageHeader.vue'
import OverviewContent from '@/views/cluster/Overview/components/OverviewContent.vue'

definePage({ name: 'ClusterOverview' })

type Props = {
  currentCluster: Resource<ClusterSpec>
}

defineProps<Props>()
</script>

<template>
  <PageContainer>
    <PageHeader :title="$route.params.cluster" />
    <OverviewContent :current-cluster="currentCluster" />
  </PageContainer>
</template>
