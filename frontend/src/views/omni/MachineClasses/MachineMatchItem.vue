<!--
Copyright (c) 2025 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<template>
  <t-list-item>
    <template #default>
      <div class="flex text-naturals-N13 text-xs items-center">
        <div class="flex-1 flex gap-2 items-center">
          <router-link :to="{ name: 'MachineLogs', params: { machine: machine?.metadata?.id } }" class="list-item-link pr-2">
            <word-highlighter
              :query="(searchQuery ?? '')"
              split-by-space
              :textToHighlight="machineName"
              highlightClass="bg-naturals-N14"/>
          </router-link>
          <item-labels :resource="machine" @filter-label="(e) => $emit('filterLabels', e)"/>
        </div>
        <div class="flex-initial w-8">
          <div class="flex justify-end"/>
        </div>
      </div>
    </template>
  </t-list-item>
</template>

<script setup lang="ts">
import { computed, toRefs } from "vue";

import { Resource } from "@/api/grpc";
import { MachineStatusSpec } from "@/api/omni/specs/omni.pb";

import TListItem from "@/components/common/List/TListItem.vue";
import ItemLabels from "@/views/omni/ItemLabels/ItemLabels.vue";
import WordHighlighter from "vue-word-highlighter";

const props = defineProps<{
  machine: Resource<MachineStatusSpec>
  searchQuery?: string
}>();

defineEmits(["filterLabels"]);

const { machine } = toRefs(props);

const machineName = computed(() => {
  return machine?.value?.spec?.network?.hostname ?? machine?.value?.metadata?.id
});
</script>
