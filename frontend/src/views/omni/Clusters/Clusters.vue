<!--
Copyright (c) 2024 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<template>
  <div>
    <div class="flex gap-1 items-start">
      <page-header title="Clusters" class="flex-1"/>
      <t-button :disabled="!canCreateClusters" @click="openClusterCreate" type="highlighted">Create Cluster</t-button>
    </div>
    <t-list
      :opts="watchOpts"
      search
      noRecordsAlert
      pagination
      errorsAlert
      :filterValue="filterValue"
    >
      <template #input>
        <labels-input :completions-resource="{
          id: ClusterStatusType,
          type: LabelsCompletionType,
          namespace: VirtualNamespace,
        }"
        class="w-full"
        v-model:filter-labels="filterLabels"
        v-model:filter-value="filterValue"/>
      </template>
      <template #default="{ items, searchQuery }">
        <div class="flex flex-col gap-2">
          <div class="clusters-header">
            <div class="clusters-grid">
              <div class="pl-6">Name</div>
              <div>Machines Healthy</div>
              <div>Phase</div>
              <div>Labels</div>
            </div>
            <div>Actions</div>
          </div>
          <cluster-item v-for="(item, index) in items"
            :key="itemID(item)"
            :defaultOpen="index === 0"
            :search-query="searchQuery"
            @filterLabels="label => addLabel(filterLabels, label)"
            :item="item"/>
        </div>
      </template>
    </t-list>
  </div>
</template>

<script setup lang="ts">
import { useRouter } from "vue-router";
import { DefaultNamespace, ClusterStatusType, LabelsCompletionType, VirtualNamespace } from "@/api/resources";
import { Runtime } from "@/api/common/omni.pb";
import { WatchOptions, itemID } from "@/api/watch";

import TList from "@/components/common/List/TList.vue";
import TButton from "@/components/common/Button/TButton.vue";
import ClusterItem from "@/views/omni/Clusters/ClusterItem.vue";
import PageHeader from "@/components/common/PageHeader.vue";
import { computed, ref } from "vue";
import { canCreateClusters } from "@/methods/auth";
import LabelsInput from "../ItemLabels/LabelsInput.vue";
import { addLabel, selectors, Label } from "@/methods/labels";

const router = useRouter();

const watchOpts = computed<WatchOptions>(() => {
  return {
    runtime: Runtime.Omni,
    resource: {
      namespace: DefaultNamespace,
      type: ClusterStatusType,
    },
    selectors: selectors(filterLabels.value),
    sortByField: "created",
  }
});

const filterValue = ref("");
const filterLabels = ref<Label[]>([]);

const openClusterCreate = () => {
  router.push({ name: "ClusterCreate" })
};
</script>

<style scoped>
.clusters-grid {
  @apply flex-1 grid grid-cols-4 pr-2;
}

.clusters-header {
  @apply flex items-center bg-naturals-N2 mb-1 px-3 py-2.5;
}

.clusters-header > * {
  @apply text-xs;
}
</style>
