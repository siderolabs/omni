<!--
Copyright (c) 2026 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import { ref } from 'vue'
import WordHighlighter from 'vue-word-highlighter'

import { Runtime } from '@/api/common/omni.pb'
import { InfraMachineType, InfraProviderNamespace, LabelInfraProviderID } from '@/api/resources'
import { itemID } from '@/api/watch'
import TButton from '@/components/common/Button/TButton.vue'
import TCheckbox from '@/components/common/Checkbox/TCheckbox.vue'
import TList from '@/components/common/List/TList.vue'
import PageHeader from '@/components/common/PageHeader.vue'
import StatsItem from '@/components/common/Stats/StatsItem.vue'
import TableCell from '@/components/common/Table/TableCell.vue'
import TableRoot from '@/components/common/Table/TableRoot.vue'
import TableRow from '@/components/common/Table/TableRow.vue'
import MachineAccept from '@/views/omni/Modals/MachineAccept.vue'
import MachineReject from '@/views/omni/Modals/MachineReject.vue'
import MachineUnreject from '@/views/omni/Modals/MachineUnreject.vue'

const selectedMachines = ref(new Set<string>())

const acceptModalOpen = ref(false)
const rejectModalOpen = ref(false)
const unrejectModalOpen = ref(false)
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
      }"
      filter-caption="Acceptance status"
      :filter-options="[
        { desc: 'Pending', query: 'pending' },
        { desc: 'Rejected', query: 'rejected' },
      ]"
      search
      pagination
    >
      <template #header="{ itemsCount }">
        <PageHeader title="Pending Machines">
          <StatsItem title="Machines" :value="itemsCount" icon="nodes" />
        </PageHeader>
      </template>

      <template #extra-controls="{ selectedFilterOption }">
        <template v-if="selectedFilterOption === 'Pending'">
          <TButton
            icon="check"
            variant="highlighted"
            :disabled="!selectedMachines.size"
            @click="acceptModalOpen = true"
          >
            Accept
          </TButton>

          <TButton icon="close" :disabled="!selectedMachines.size" @click="rejectModalOpen = true">
            Reject
          </TButton>
        </template>

        <TButton
          v-else
          icon="close"
          variant="highlighted"
          :disabled="!selectedMachines.size"
          @click="unrejectModalOpen = true"
        >
          Unreject
        </TButton>
      </template>

      <template #default="{ items, searchQuery }">
        <TableRoot class="w-full">
          <template #head>
            <TableRow>
              <TableCell th>ID</TableCell>
              <TableCell th>Provider</TableCell>
            </TableRow>
          </template>

          <template #body>
            <TableRow
              v-for="item in items"
              :key="itemID(item)"
              role="button"
              :aria-label="item.metadata.id"
              @click="
                () =>
                  selectedMachines.has(item.metadata.id!)
                    ? selectedMachines.delete(item.metadata.id!)
                    : selectedMachines.add(item.metadata.id!)
              "
            >
              <TableCell>
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
              </TableCell>

              <TableCell>{{ item.metadata.labels?.[LabelInfraProviderID] }}</TableCell>
            </TableRow>
          </template>
        </TableRoot>
      </template>
    </TList>

    <MachineAccept
      v-model:open="acceptModalOpen"
      :machines="Array.from(selectedMachines)"
      @confirmed="selectedMachines.clear()"
    />

    <MachineReject
      v-model:open="rejectModalOpen"
      :machines="Array.from(selectedMachines)"
      @confirmed="selectedMachines.clear()"
    />

    <MachineUnreject
      v-model:open="unrejectModalOpen"
      :machines="Array.from(selectedMachines)"
      @confirmed="selectedMachines.clear()"
    />
  </div>
</template>
