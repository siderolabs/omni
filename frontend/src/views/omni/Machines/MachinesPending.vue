<!--
Copyright (c) 2025 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import { ref } from 'vue'
import { useRouter } from 'vue-router'
import WordHighlighter from 'vue-word-highlighter'

import { Runtime } from '@/api/common/omni.pb'
import {
  InfraMachineType,
  InfraProviderNamespace,
  LabelInfraProviderID,
  LabelMachinePendingAccept,
} from '@/api/resources'
import { itemID } from '@/api/watch'
import TButton from '@/components/common/Button/TButton.vue'
import TCheckbox from '@/components/common/Checkbox/TCheckbox.vue'
import TList from '@/components/common/List/TList.vue'
import PageHeader from '@/components/common/PageHeader.vue'
import StatsItem from '@/components/common/Stats/StatsItem.vue'

const router = useRouter()

const selectedMachines = ref(new Set<string>())

function acceptMachines(...ids: string[]) {
  router.push({ query: { modal: 'machineAccept', machine: ids } })
}

function rejectMachines(...ids: string[]) {
  router.push({ query: { modal: 'machineReject', machine: ids } })
}
</script>

<template>
  <div>
    <TList
      :opts="{
        runtime: Runtime.Omni,
        resource: {
          type: InfraMachineType,
          namespace: InfraProviderNamespace,
        },
        selectors: [LabelMachinePendingAccept],
      }"
      search
      pagination
    >
      <template #header="{ itemsCount }">
        <PageHeader title="Pending Machines">
          <StatsItem title="Machines" :value="itemsCount" icon="nodes" />
        </PageHeader>
      </template>

      <template #extra-controls>
        <TButton
          icon="check"
          type="highlighted"
          :disabled="!selectedMachines.size"
          @click="acceptMachines(...selectedMachines)"
        >
          Accept
        </TButton>

        <TButton
          icon="close"
          :disabled="!selectedMachines.size"
          @click="rejectMachines(...selectedMachines)"
        >
          Reject
        </TButton>
      </template>

      <template #default="{ items, searchQuery }">
        <table class="w-full text-xs text-naturals-n13">
          <thead class="bg-naturals-n2 text-left">
            <tr class="[&>th]:p-2">
              <th>ID</th>
              <th>Provider</th>
            </tr>
          </thead>

          <tbody>
            <tr
              v-for="item in items"
              :key="itemID(item)"
              class="border-b border-naturals-n5 last-of-type:border-none hover:bg-white/5 [&>td]:px-2 [&>td]:py-4"
              role="button"
              :aria-label="item.metadata.id"
              @click="
                () =>
                  selectedMachines.has(item.metadata.id!)
                    ? selectedMachines.delete(item.metadata.id!)
                    : selectedMachines.add(item.metadata.id!)
              "
            >
              <td>
                <div class="flex items-center gap-2">
                  <TCheckbox
                    :model-value="selectedMachines.has(item.metadata.id!)"
                    class="pointer-events-none"
                  />

                  <WordHighlighter
                    :query="searchQuery"
                    :text-to-highlight="item.metadata.id"
                    highlight-class="bg-naturals-n14"
                  />
                </div>
              </td>

              <td>{{ item.metadata.labels?.[LabelInfraProviderID] }}</td>
            </tr>
          </tbody>
        </table>
      </template>
    </TList>
  </div>
</template>
