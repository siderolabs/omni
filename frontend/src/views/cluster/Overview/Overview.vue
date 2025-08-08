<!--
Copyright (c) 2025 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<template>
  <div>
    <page-header :title="$route.params.cluster as string" />
    <overview-content :currentCluster="currentCluster" />
  </div>
</template>

<script setup lang="ts">
import OverviewContent from '@/views/cluster/Overview/components/OverviewContent.vue'
import PageHeader from '@/components/common/PageHeader.vue'
import type { Resource } from '@/api/grpc'
import type { ClusterSpec } from '@/api/omni/specs/omni.pb'

type Props = {
  currentCluster: Resource<ClusterSpec>
}

defineProps<Props>()
</script>
