<!--
Copyright (c) 2025 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<template>
  <t-list :opts="watchOpts" search pagination>
    <template #default="{ items, searchQuery }">
      <div class="header">
        <div class="list-grid">
          <div>Name</div>
          <div>Mode</div>
          <div>Provider</div>
        </div>
      </div>
      <t-list-item v-for="item in items" :key="item.metadata.id!">
        <div class="text-naturals-N12 relative pr-3" :class="{ 'pl-7': !item.spec.description }">
          <icon-button icon="delete" @click="() => openMachineClassDestroy(item.metadata.id!)" class="absolute right-0 my-auto top-0 bottom-0"/>
          <div class="list-grid">
            <div>
              <router-link
                :to="{ name: 'MachineClassEdit', params: { classname: item.metadata.id } }"
                class="list-item-link">
                <WordHighlighter
                  highlight-class="bg-naturals-N14"
                  :query="searchQuery">
                    {{ item.metadata.id }}
                </WordHighlighter>
              </router-link>
            </div>
            <div class="flex">
              <tag>
                {{ item.spec.auto_provision ? 'Auto Provision' : 'Manual' }}
              </tag>
            </div>
            <div v-if="item.spec.auto_provision">
              {{ item.spec.auto_provision.provider_id }}
            </div>
          </div>
        </div>
      </t-list-item>
    </template>
  </t-list>
</template>

<script setup lang="ts">
import { Runtime } from "@/api/common/omni.pb";
import { DefaultNamespace, MachineClassType } from "@/api/resources";
import { WatchOptions } from "@/api/watch";
import { computed } from "vue";
import { useRouter } from "vue-router";

import WordHighlighter from "vue-word-highlighter";
import TList from "@/components/common/List/TList.vue";
import TListItem from "@/components/common/List/TListItem.vue";
import IconButton from "@/components/common/Button/IconButton.vue";
import Tag from "@/components/common/Tag/Tag.vue";

const watchOpts = computed((): WatchOptions => {
  return {
    resource: {
      namespace: DefaultNamespace,
      type: MachineClassType,
    },
    runtime: Runtime.Omni,
  }
});

const router = useRouter();

const openMachineClassDestroy = (id: string) => {
  router.push({ query: { modal: 'machineClassDestroy', classname: id } })
}
</script>

<style scoped>
.header {
  @apply bg-naturals-N2 mb-1 py-2 px-6 text-xs pl-10;
}

.list-grid {
  @apply grid grid-cols-3 items-center justify-center pr-12 gap-1;
}
</style>
