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
      :opts="{
        resource: resource,
        runtime: Runtime.Omni,
        sortByField: 'created'
      }"
      search
      noRecordsAlert
      pagination
      errorsAlert
      ref="list"
    >
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
            @filterLabels="filterByLabel"
            :item="item"/>
        </div>
      </template>
    </t-list>
  </div>
</template>

<script setup lang="ts">
import { useRouter } from "vue-router";
import { DefaultNamespace, ClusterStatusType } from "@/api/resources";
import { Runtime } from "@/api/common/omni.pb";
import { itemID } from "@/api/watch";

import TList from "@/components/common/List/TList.vue";
import TButton from "@/components/common/Button/TButton.vue";
import ClusterItem from "@/views/omni/Clusters/ClusterItem.vue";
import PageHeader from "@/components/common/PageHeader.vue";
import {ref, Ref} from "vue";
import { canCreateClusters } from "@/methods/auth";

const router = useRouter();

const resource = {
  namespace: DefaultNamespace,
  type: ClusterStatusType,
};

const openClusterCreate = () => {
  router.push({ name: "ClusterCreate" })
};

const list: Ref<{addFilterLabel: (label: {key: string, value?: string}) => void} | null> = ref(null);

const filterByLabel = (e: {key: string, value?: string}) => {
    if (list.value) {
        list.value.addFilterLabel(e);
    }
}
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
