<!--
Copyright (c) 2026 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import PageContainer from '@/components/common/PageContainer/PageContainer.vue'
import PageHeader from '@/components/common/PageHeader.vue'
import Patches from '@/views/cluster/Config/Patches.vue'

definePage({ name: 'ClusterConfigPatches' })
</script>

<template>
  <PageContainer class="flex flex-col gap-2 overflow-y-auto">
    <div class="flex items-start">
      <PageHeader class="flex-1" :title="`Cluster ${$route.params.cluster} Config Patches`" />
    </div>
    <Patches :cluster="$route.params.cluster" />
  </PageContainer>
</template>
