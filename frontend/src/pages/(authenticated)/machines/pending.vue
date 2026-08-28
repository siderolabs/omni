<!--
Copyright (c) 2026 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import { ref, watch } from 'vue'
import WordHighlighter from 'vue-word-highlighter'

import { Runtime } from '@/api/common/omni.pb'
import type { InfraMachineSpec } from '@/api/omni/specs/infra.pb'
import { InfraMachineType, InfraProviderNamespace, LabelInfraProviderID } from '@/api/resources'
import TButton from '@/components/Button/TButton.vue'
import TCheckbox from '@/components/Checkbox/TCheckbox.vue'
import PageContainer from '@/components/PageContainer/PageContainer.vue'
import PageHeader from '@/components/PageHeader.vue'
import Pagination from '@/components/Pagination/Pagination.vue'
import TSelectList from '@/components/SelectList/TSelectList.vue'
import TSpinner from '@/components/Spinner/TSpinner.vue'
import StatsItem from '@/components/Stats/StatsItem.vue'
import TableCell from '@/components/Table/TableCell.vue'
import TableRoot from '@/components/Table/TableRoot.vue'
import TableRow from '@/components/Table/TableRow.vue'
import TAlert from '@/components/TAlert.vue'
import TInput from '@/components/TInput/TInput.vue'
import { useResourcePagination } from '@/methods/resource/useResourcePagination'
import { useResourceSearch } from '@/methods/resource/useResourceSearch'
import { useResourceWatch } from '@/methods/useResourceWatch'
import MachineAccept from '@/views/Machines/components/MachineAccept.vue'
import MachineDeleteModal from '@/views/Machines/components/MachineDeleteModal.vue'
import MachineReject from '@/views/Machines/components/MachineReject.vue'
import MachineUnreject from '@/views/Machines/components/MachineUnreject.vue'

definePage({ name: 'MachinesPending' })

const selectedMachines = ref(new Set<string>())

const acceptModalOpen = ref(false)
const deleteModalOpen = ref(false)
const rejectModalOpen = ref(false)
const unrejectModalOpen = ref(false)

const filterValue = ref('')
const filterOptions = [
  { label: 'Pending', value: 'pending' },
  { label: 'Rejected', value: 'rejected' },
]

const {
  watchOptions: searchState,
  searchQuery,
  selectedFilterOption,
} = useResourceSearch({
  filterValue,
  filterOptions,
})

const {
  total,
  watchOptions: paginationState,
  currentPage,
  currentPageSize,
  pageCount,
  pageSizeSelectValues,
} = useResourcePagination({
  resetOn: [searchState],
})

watch(selectedFilterOption, () => selectedMachines.value.clear())

const { data, loading, err } = useResourceWatch<InfraMachineSpec>(
  () => ({
    runtime: Runtime.Omni,
    resource: {
      type: InfraMachineType,
      namespace: InfraProviderNamespace,
    },
    ...paginationState.value,
    ...searchState.value,
  }),
  { total },
)

function unselectDeletedMachines(machineIds: string[]) {
  machineIds.forEach((id) => selectedMachines.value.delete(id))
}
</script>

<template>
  <PageContainer class="flex h-full flex-col gap-2">
    <PageHeader title="Pending Machines">
      <StatsItem title="Machines" :value="total" icon="nodes" />
    </PageHeader>

    <div class="flex grow flex-col gap-4 overflow-auto">
      <TInput v-model="filterValue" icon="search" />

      <div class="flex justify-between gap-2">
        <div class="flex grow gap-2">
          <template v-if="selectedFilterOption === 'pending'">
            <TButton
              icon="check"
              variant="highlighted"
              :disabled="!selectedMachines.size"
              @click="acceptModalOpen = true"
            >
              Accept
            </TButton>

            <TButton
              icon="close"
              :disabled="!selectedMachines.size"
              @click="rejectModalOpen = true"
            >
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

          <TButton
            variant="primary"
            icon="delete"
            :disabled="!selectedMachines.size"
            @click="deleteModalOpen = true"
          >
            Delete selected
          </TButton>
        </div>

        <div class="flex flex-wrap items-center gap-2">
          <TSelectList
            v-model="selectedFilterOption"
            title="Acceptance status"
            :values="filterOptions"
          />

          <TSelectList
            v-model="currentPageSize"
            title="Items per Page"
            :values="pageSizeSelectValues"
          />
        </div>
      </div>

      <div class="grow overflow-auto">
        <div v-if="loading" class="flex size-full items-center justify-center">
          <TSpinner class="size-6" />
        </div>

        <TAlert v-else-if="err" title="Failed to Fetch Data" type="error">{{ err }}.</TAlert>

        <TAlert v-else-if="data.length === 0" type="info" title="No Records">
          No entries of the requested resource type are found on the server.
        </TAlert>

        <TableRoot v-show="!loading && !err && data.length > 0" class="max-h-full w-full">
          <template #head>
            <TableRow>
              <TableCell th>ID</TableCell>
              <TableCell th>Provider</TableCell>
            </TableRow>
          </template>

          <template #body>
            <TableRow
              v-for="item in data"
              :key="item.metadata.id"
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
                    split-by-space
                    highlight-class="bg-naturals-n14"
                  />
                </div>
              </TableCell>

              <TableCell>{{ item.metadata.labels?.[LabelInfraProviderID] }}</TableCell>
            </TableRow>
          </template>
        </TableRoot>
      </div>
    </div>

    <Pagination v-model:current-page="currentPage" :page-count="pageCount" />

    <MachineAccept
      v-model:open="acceptModalOpen"
      :machines="Array.from(selectedMachines)"
      @confirmed="selectedMachines.clear()"
    />

    <MachineDeleteModal
      v-model:open="deleteModalOpen"
      :machines="Array.from(selectedMachines).map((id) => ({ id }))"
      @deleted="unselectDeletedMachines"
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
  </PageContainer>
</template>
