<!--
Copyright (c) 2025 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<template>
  <div>
    <t-list
      :opts="watchOpts"
      search
      pagination
      >
      <template #header="{ itemsCount, filtered }">
        <div class="flex gap-4">
          <page-header title="Pending Machines">
            <stats-item pluralized-text="Machine" :count="itemsCount" icon="nodes" :text="filtered ? ' Found' : ' Total'"/>
          </page-header>
        </div>
        <machine-tabs/>
      </template>
      <template #default="{ items, searchQuery }">
        <div class="header">
          <p>ID</p>
          <p>Provider</p>
        </div>
        <t-list-item v-for="item in items" :key="itemID(item)">
          <div class="flex gap-2 items-center">
            <div class="grid grid-cols-2 flex-1 gap-2">
              <WordHighlighter
                  :query="searchQuery"
                  :textToHighlight="item.metadata.id"
                  highlightClass="bg-naturals-N14"
              />
              <span>{{ item.metadata.labels?.[LabelInfraProviderID] }}</span>
            </div>
            <t-button icon="check" type="highlighted" @click="() => acceptMachine(item)">Accept</t-button>
            <t-button icon="close" @click="() => rejectMachine(item)">Reject</t-button>
          </div>
        </t-list-item>
      </template>
    </t-list>
  </div>
</template>

<script setup lang="ts">
import { Runtime } from "@/api/common/omni.pb";
import { InfraMachineType, InfraProviderNamespace, LabelMachinePendingAccept, LabelInfraProviderID } from "@/api/resources";
import { itemID, WatchOptions } from "@/api/watch";

import TList from "@/components/common/List/TList.vue";
import PageHeader from "@/components/common/PageHeader.vue";
import { computed } from "vue";
import StatsItem from "@/components/common/Stats/StatsItem.vue";
import TListItem from "@/components/common/List/TListItem.vue";
import { Resource } from "@/api/grpc";
import TButton from "@/components/common/Button/TButton.vue";
import WordHighlighter from "vue-word-highlighter";
import { useRouter } from "vue-router";
import MachineTabs from "./MachineTabs.vue";

const router = useRouter();

const watchOpts = computed<WatchOptions>(() => {
  return {
    runtime: Runtime.Omni,
    resource: {
      type: InfraMachineType,
      namespace: InfraProviderNamespace,
    },
    selectors: [`${LabelMachinePendingAccept}`],
  };
});

const acceptMachine = (item: Resource) => {
  router.push({
    query: { modal: "machineAccept", machine: item.metadata.id },
  });
};

const rejectMachine = (item: Resource) => {
  router.push({
    query: { modal: "machineReject", machine: item.metadata.id },
  });
};
</script>

<style scoped>
.header {
  @apply grid grid-cols-2 p-2 gap-2 bg-naturals-N2 pr-56 pl-3 text-xs text-naturals-N13;
}
</style>
