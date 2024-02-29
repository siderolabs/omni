<!--
Copyright (c) 2024 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<template>
  <div>
    <page-header title="Machines"/>
    <t-list
      :opts="watchOpts"
      search
      pagination
      :sortOptions="sortOptions"
      ref="list"
      >
      <template #default="{ items, searchQuery }">
        <machine-item v-for="item in items" :key="itemID(item)" :machine="item" @filterLabels="filterByLabel" :search-query="searchQuery"/>
      </template>
    </t-list>
  </div>
</template>

<script setup lang="ts">
import { Runtime } from "@/api/common/omni.pb";
import { MetricsNamespace, MachineStatusLinkType } from "@/api/resources";
import { itemID, WatchOptions } from "@/api/watch";

import TList from "@/components/common/List/TList.vue";
import MachineItem from "@/views/omni/Machines/MachineItem.vue";
import PageHeader from "@/components/common/PageHeader.vue";
import { Ref, ref } from "vue";

const watchOpts: WatchOptions = {
    runtime: Runtime.Omni,
    resource: {
      type: MachineStatusLinkType,
      namespace: MetricsNamespace,
    },
  };

const sortOptions = [
  {id: 'machine_created_at', desc: 'Creation Time ⬆', descending: true},
  {id: 'machine_created_at', desc: 'Creation Time ⬇'},
  {id: 'hostname', desc: 'Hostname ⬆'},
  {id: 'hostname', desc: 'Hostname ⬇', descending: true},
  {id: 'id', desc: 'ID ⬆'},
  {id: 'id', desc: 'ID ⬇', descending: true},
  {id: 'last_alive', desc: 'Last Active Time ⬆', descending: true},
  {id: 'last_alive', desc: 'Last Active Time ⬇'},
];

const list: Ref<{addFilterLabel: (label: {key: string, value?: string}) => void} | null> = ref(null);

const filterByLabel = (e: {key: string, value?: string}) => {
  if (list.value) {
    list.value.addFilterLabel(e);
  }
}
</script>
