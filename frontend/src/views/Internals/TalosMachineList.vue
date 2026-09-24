<!--
Copyright (c) 2026 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import { useRouteQuery } from '@vueuse/router'
import { computed } from 'vue'
import WordHighlighter from 'vue-word-highlighter'

import PageContainer from '@/components/PageContainer/PageContainer.vue'
import TSpinner from '@/components/Spinner/TSpinner.vue'
import TableCell from '@/components/Table/TableCell.vue'
import TableRoot from '@/components/Table/TableRoot.vue'
import TableRow from '@/components/Table/TableRow.vue'
import TAlert from '@/components/TAlert.vue'
import TInput from '@/components/TInput/TInput.vue'
import ResourceBreadcrumbs from '@/views/Internals/components/ResourceBreadcrumbs.vue'
import { useMachineOptions } from '@/views/Internals/lib/useMachineOptions'

const search = useRouteQuery('q', '')

const { machines, loading } = useMachineOptions()

const filtered = computed(() => {
  const term = search.value.trim().toLowerCase()

  return machines.value.filter((m) =>
    [m.id, m.name, m.location].some((value) => value?.toLowerCase().includes(term)),
  )
})
</script>

<template>
  <PageContainer class="flex h-full flex-col">
    <div role="heading" aria-level="1" class="mb-2">
      <ResourceBreadcrumbs runtime="talos" />
    </div>

    <p class="mb-4 text-sm text-naturals-n13">
      Talos resources are served by each node. Pick a machine to browse its resources.
    </p>

    <TInput v-model="search" class="mb-4" icon="search" title="Search" />

    <div v-if="loading" class="flex grow items-center justify-center">
      <TSpinner class="size-6" />
    </div>

    <TAlert v-else-if="!filtered.length" type="info" title="No machines match" />

    <div v-else class="min-h-0 grow overflow-auto">
      <TableRoot class="w-full">
        <template #head>
          <TableRow>
            <TableCell th>Name</TableCell>
            <TableCell th>Cluster</TableCell>
            <TableCell th>ID</TableCell>
            <TableCell th>Status</TableCell>
          </TableRow>
        </template>

        <template #body>
          <TableRow
            v-for="m in filtered"
            :key="m.id"
            class="relative"
            :class="m.connected ? 'hover:bg-white/5' : 'opacity-50'"
          >
            <TableCell class="text-naturals-n14">
              <!-- Stretched over the row so the whole row is clickable while staying a real link -->
              <RouterLink
                v-if="m.connected"
                :to="{ name: 'InternalsTalosResources', params: { machine: m.id } }"
                class="after:absolute after:inset-0"
              >
                <WordHighlighter
                  :query="search"
                  :text-to-highlight="m.name"
                  highlight-class="bg-naturals-n14"
                />
              </RouterLink>

              <WordHighlighter
                v-else
                :query="search"
                :text-to-highlight="m.name"
                highlight-class="bg-naturals-n14"
              />
            </TableCell>
            <TableCell>
              <WordHighlighter
                :query="search"
                :text-to-highlight="m.location"
                highlight-class="bg-naturals-n14"
              />
            </TableCell>
            <TableCell class="text-naturals-n10">
              <WordHighlighter
                :query="search"
                :text-to-highlight="m.id"
                highlight-class="bg-naturals-n14"
              />
            </TableCell>
            <TableCell>{{ m.connected ? 'Connected' : 'Disconnected' }}</TableCell>
          </TableRow>
        </template>
      </TableRoot>
    </div>
  </PageContainer>
</template>
