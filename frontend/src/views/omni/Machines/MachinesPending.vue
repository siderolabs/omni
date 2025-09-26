<!--
Copyright (c) 2025 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
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
import TList from '@/components/common/List/TList.vue'
import PageHeader from '@/components/common/PageHeader.vue'
import StatsItem from '@/components/common/Stats/StatsItem.vue'
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

      <template #default="{ items, searchQuery }">
        <table class="w-full text-xs text-naturals-n13">
          <thead class="bg-naturals-n2 text-left">
            <tr class="[&>th]:p-2">
              <th>ID</th>
              <th>Provider</th>
              <th></th>
            </tr>
          </thead>

          <tbody>
            <tr
              v-for="item in items"
              :key="itemID(item)"
              class="border-b border-naturals-n5 last-of-type:border-none [&>td]:px-2 [&>td]:py-4"
            >
              <td>
                <WordHighlighter
                  :query="searchQuery"
                  :text-to-highlight="item.metadata.id"
                  highlight-class="bg-naturals-n14"
                />
              </td>

              <td>{{ item.metadata.labels?.[LabelInfraProviderID] }}</td>

              <td class="text-right">
                <div class="inline-flex items-center gap-2">
                  <TButton
                    icon="check"
                    type="highlighted"
                    @click="
                      $router.push({ query: { modal: 'machineAccept', machine: item.metadata.id } })
                    "
                  >
                    Accept
                  </TButton>

                  <TButton
                    icon="close"
                    @click="
                      $router.push({ query: { modal: 'machineReject', machine: item.metadata.id } })
                    "
                  >
                    Reject
                  </TButton>
                </div>
              </td>
            </tr>
          </tbody>
        </table>
      </template>
    </TList>
  </div>
</template>
