<!--
Copyright (c) 2025 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<template>
  <div class="flex flex-col gap-2 overflow-y-auto">
    <div class="flex items-start">
      <page-header class="flex-1" :title="`Cluster ${$route.params.cluster} Config Patches`"/>
    </div>
    <patches :cluster="currentCluster"/>
  </div>
</template>

<script setup lang="ts">
import { Resource } from "@/api/grpc";
import { ClusterSpec } from "@/api/omni/specs/omni.pb";

import Patches from "./Patches.vue";
import PageHeader from "@/components/common/PageHeader.vue";

defineProps<{
  currentCluster?: Resource<ClusterSpec>,
}>();
</script>
