<!--
Copyright (c) 2025 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import type { Resource } from '@/api/grpc'
import type { ClusterSpec } from '@/api/omni/specs/omni.pb'
import PageHeader from '@/components/common/PageHeader.vue'

import Patches from './Patches.vue'

defineProps<{
  currentCluster?: Resource<ClusterSpec>
}>()
</script>

<template>
  <div class="flex flex-col gap-2 overflow-y-auto">
    <div class="flex items-start">
      <PageHeader class="flex-1" :title="`Cluster ${$route.params.cluster} Config Patches`" />
    </div>
    <Patches :cluster="currentCluster" />
  </div>
</template>
