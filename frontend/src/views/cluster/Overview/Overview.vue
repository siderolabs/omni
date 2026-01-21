<!--
Copyright (c) 2026 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import type { Resource } from '@/api/grpc'
import type { ClusterSpec } from '@/api/omni/specs/omni.pb'
import PageHeader from '@/components/common/PageHeader.vue'
import OverviewContent from '@/views/cluster/Overview/components/OverviewContent.vue'

type Props = {
  currentCluster: Resource<ClusterSpec>
}

defineProps<Props>()
</script>

<template>
  <div>
    <PageHeader :title="$route.params.cluster as string" />
    <OverviewContent :current-cluster="currentCluster" />
  </div>
</template>
