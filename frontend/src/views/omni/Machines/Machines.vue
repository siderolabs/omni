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
      :filterValue="filterValue"
      >
      <template #input>
        <labels-input :completions-resource="{
          id: MachineStatusType,
          type: LabelsCompletionType,
          namespace: VirtualNamespace,
        }"
        class="w-full"
        v-model:filter-labels="filterLabels"
        v-model:filter-value="filterValue"/>
      </template>
      <template #default="{ items, searchQuery }">
        <machine-item v-for="item in items" :key="itemID(item)" :machine="item" @filterLabels="(label) => addLabel(filterLabels, label)" :search-query="searchQuery"/>
      </template>
    </t-list>
  </div>
</template>

<script setup lang="ts">
import { Runtime } from "@/api/common/omni.pb";
import { MetricsNamespace, MachineStatusLinkType, LabelsCompletionType, VirtualNamespace, MachineStatusType } from "@/api/resources";
import { itemID, WatchOptions } from "@/api/watch";

import TList from "@/components/common/List/TList.vue";
import MachineItem from "@/views/omni/Machines/MachineItem.vue";
import PageHeader from "@/components/common/PageHeader.vue";
import { computed, ref } from "vue";
import LabelsInput from "@/views/omni/ItemLabels/LabelsInput.vue";
import { Label, addLabel, selectors } from "@/methods/labels";

const watchOpts = computed<WatchOptions>(() => {
  return {
    runtime: Runtime.Omni,
    resource: {
      type: MachineStatusLinkType,
      namespace: MetricsNamespace,
    },
    selectors: selectors(filterLabels.value),
  }
});

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

const filterLabels = ref<Label[]>([]);
const filterValue = ref("");
</script>
