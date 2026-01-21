<!--
Copyright (c) 2026 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import NodeDetailsTabs from '@/views/cluster/Nodes/components/NodeDetailsTabs.vue'
import NodesHeader from '@/views/cluster/Nodes/NodesHeader.vue'
</script>

<template>
  <div class="flex h-full flex-col">
    <NodesHeader />
    <NodeDetailsTabs :machine="$route.params.machine as string" class="mb-4" />
    <RouterView class="grow" />
  </div>
</template>
