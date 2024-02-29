<!--
Copyright (c) 2024 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<template>
  <div class="flex flex-col">
    <nodes-header/>
    <node-details-tabs
      :machine="$route.params.machine as string"
      class="mb-4"
    />
    <router-view name="nodeDetails"/>
  </div>
</template>

<script setup lang="ts">
import NodesHeader from "@/views/cluster/Nodes/NodesHeader.vue";
import NodeDetailsTabs from "@/views/cluster/Nodes/components/NodeDetailsTabs.vue";
</script>
