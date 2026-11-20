<!--
Copyright (c) 2026 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import { useIntervalFn, useNow } from '@vueuse/core'
import prettyBytes from 'pretty-bytes'
import { computed, ref } from 'vue'
import WordHighlighter from 'vue-word-highlighter'

import { Runtime } from '@/api/common/omni.pb'
import {
  TalosContainerImageStatusType,
  TalosContainerInstanceStatusType,
  TalosContainersNamespace,
} from '@/api/resources'
import type {
  ContainerImageStatusSpec,
  ContainerInstanceStatusSpec,
} from '@/api/talos/containers.pb'
import PageContainer from '@/components/PageContainer/PageContainer.vue'
import TSelectList from '@/components/SelectList/TSelectList.vue'
import TSpinner from '@/components/Spinner/TSpinner.vue'
import StatusPill from '@/components/Status/StatusPill.vue'
import TableCell from '@/components/Table/TableCell.vue'
import TableRoot from '@/components/Table/TableRoot.vue'
import TableRow from '@/components/Table/TableRow.vue'
import TAlert from '@/components/TAlert.vue'
import TInput from '@/components/TInput/TInput.vue'
import { useResourceWatch } from '@/methods/useResourceWatch'
import ContainerDetailCell from '@/views/Containers/components/ContainerDetailCell.vue'
import ContainerImage from '@/views/Containers/components/ContainerImage.vue'
import { containerRows, imageRef } from '@/views/Containers/lib/containerRows'
import { formatCPU } from '@/views/Containers/lib/containerStats'
import { formatSince, stateAppearance, stateName } from '@/views/Containers/lib/containerStatus'
import {
  containerHealthFilterOptions,
  type ContainerSort,
  containerSortOptions,
  filterContainers,
} from '@/views/Containers/lib/filterContainers'
import { injectMachineContainers } from '@/views/Containers/lib/machineContainers'
import { useContainerStats } from '@/views/Containers/lib/useContainerStats'

const { machineId } = defineProps<{
  machineId: string
}>()

// The statuses come from the page, which watches them to decide whether this tab shows
const { data: statuses, loading, err, supported } = injectMachineContainers()

const { data: images } = useResourceWatch<ContainerImageStatusSpec>(() => ({
  runtime: Runtime.Talos,
  resource: {
    namespace: TalosContainersNamespace,
    type: TalosContainerImageStatusType,
  },
  context: { node: machineId },
  skip: !supported.value,
}))

const { data: instances } = useResourceWatch<ContainerInstanceStatusSpec>(() => ({
  runtime: Runtime.Talos,
  resource: {
    namespace: TalosContainersNamespace,
    type: TalosContainerInstanceStatusType,
  },
  context: { node: machineId },
  skip: !supported.value,
}))

const { metrics } = useContainerStats(
  () => machineId,
  () => supported.value && statuses.value.length > 0,
)

// Since counts in seconds, so a tick a second is enough
const now = useNow({ scheduler: (cb) => useIntervalFn(cb, 1000) })

const search = ref('')
const healthFilter = ref<string>()
const sort = ref<ContainerSort>('name')

const rows = computed(() =>
  containerRows(statuses.value, images.value, instances.value, metrics.value),
)

const visibleRows = computed(() =>
  filterContainers(rows.value, {
    search: search.value,
    health: healthFilter.value,
    sort: sort.value,
  }),
)
</script>

<template>
  <PageContainer class="flex flex-col gap-compact">
    <TAlert v-if="!supported" type="info" title="No containers">
      Containers need Talos 1.15 or later on this machine.
    </TAlert>

    <TSpinner v-else-if="loading" class="size-6 self-center" />

    <TAlert v-else-if="err" type="error" title="Failed to fetch containers">{{ err }}</TAlert>

    <TAlert v-else-if="!rows.length" type="info" title="No containers">
      Add a ContainerConfig document to this machine's config to run a container.
    </TAlert>

    <template v-else>
      <div class="flex flex-wrap items-center gap-tight">
        <TInput
          v-model="search"
          icon="search"
          placeholder="Filter containers"
          clearable
          class="w-64 max-w-full"
        />

        <p class="grow text-xs text-content-secondary">
          Talos containers declared in this machine's configuration. State is what Talos reports.
        </p>

        <TSelectList v-model="healthFilter" title="Status" :values="containerHealthFilterOptions" />
        <TSelectList
          v-model="sort"
          title="Sort by"
          hide-selected-small-screens
          :values="[...containerSortOptions]"
        />
      </div>

      <TAlert v-if="!visibleRows.length" type="info" title="No matching containers">
        No containers match the filter and status.
      </TAlert>

      <!-- The columns need more room than a narrow page has: the table scrolls, not the page -->
      <div v-else class="overflow-x-auto">
        <TableRoot class="w-full min-w-4xl table-fixed">
          <template #head>
            <TableRow>
              <TableCell th class="w-72">Name</TableCell>
              <TableCell th class="w-32">State</TableCell>
              <TableCell th class="w-20">Restarts</TableCell>
              <TableCell th class="w-20">CPU</TableCell>
              <TableCell th class="w-24">Memory</TableCell>
              <TableCell th class="w-20">Since</TableCell>
              <TableCell th>Detail</TableCell>
            </TableRow>
          </template>

          <template #body>
            <TableRow v-for="row in visibleRows" :key="row.id">
              <!-- The name, with its image under it -->
              <TableCell>
                <div class="truncate text-content-emphasis" :title="row.id">
                  <WordHighlighter
                    :query="search.trim()"
                    :text-to-highlight="row.id"
                    highlight-class="search-match"
                  />
                </div>
                <ContainerImage :image="imageRef(row)" :digest="row.image?.digest" />
              </TableCell>

              <TableCell>
                <StatusPill v-bind="stateAppearance(row.status.state, row.status.health)">
                  {{ stateName(row.status.state) }}
                </StatusPill>
              </TableCell>

              <TableCell class="font-mono">
                {{ row.status.restartCount ?? 0 }}
              </TableCell>

              <TableCell class="font-mono">
                {{ row.metrics?.cpuPercent !== undefined ? formatCPU(row.metrics.cpuPercent) : '' }}
              </TableCell>

              <TableCell class="font-mono">
                {{
                  row.metrics?.memory !== undefined
                    ? prettyBytes(row.metrics.memory, { binary: true })
                    : ''
                }}
              </TableCell>

              <TableCell class="font-mono" :title="row.since?.toLocaleString()">
                {{ row.since && formatSince(row.since, now) }}
              </TableCell>

              <TableCell>
                <ContainerDetailCell :row />
              </TableCell>
            </TableRow>
          </template>
        </TableRoot>
      </div>
    </template>
  </PageContainer>
</template>
