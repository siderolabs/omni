<!--
Copyright (c) 2026 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import { useRouteQuery } from '@vueuse/router'
import { computed } from 'vue'
import WordHighlighter from 'vue-word-highlighter'

import { MetaNamespace, ResourceDefinitionType } from '@/api/resources'
import PageContainer from '@/components/PageContainer/PageContainer.vue'
import TSpinner from '@/components/Spinner/TSpinner.vue'
import TableCell from '@/components/Table/TableCell.vue'
import TableRoot from '@/components/Table/TableRoot.vue'
import TableRow from '@/components/Table/TableRow.vue'
import TAlert from '@/components/TAlert.vue'
import TInput from '@/components/TInput/TInput.vue'
import { useResourceWatch } from '@/methods/useResourceWatch'
import ResourceBreadcrumbs from '@/views/Internals/components/ResourceBreadcrumbs.vue'
import type { ResourceDefinitionSpec } from '@/views/Internals/lib/resourceDefinition'
import { type ResourceTarget, useResourceRuntime } from '@/views/Internals/lib/useResourceRuntime'

const search = useRouteQuery('q', '')

const { target } = defineProps<{ target: ResourceTarget }>()

const { watchOptions, resourceRoute } = useResourceRuntime(() => target)

const { data, loading, err } = useResourceWatch<ResourceDefinitionSpec>(() => ({
  ...watchOptions.value,
  resource: {
    namespace: MetaNamespace,
    type: ResourceDefinitionType,
  },
}))

const definitions = computed(() => {
  const term = search.value.trim().toLowerCase()

  return data.value
    .map((item) => item.spec)
    .filter((spec) =>
      [spec.type, spec.displayType, spec.defaultNamespace, ...(spec.allAliases ?? [])].some(
        (value) => value?.toLowerCase().includes(term),
      ),
    )
    .sort((a, b) => (a.displayType ?? '').localeCompare(b.displayType ?? ''))
})
</script>

<template>
  <PageContainer class="flex h-full flex-col">
    <div role="heading" aria-level="1" class="mb-2">
      <ResourceBreadcrumbs
        :runtime="target.runtime"
        :machine="target.runtime === 'talos' ? target.machine : undefined"
      />
    </div>

    <p class="mb-4 text-sm text-naturals-n13">
      Every resource type registered in the selected runtime. Types your role cannot read will show
      an error when opened.
    </p>

    <TInput v-model="search" class="mb-4" icon="search" title="Search" />

    <TAlert v-if="err" type="error" title="Failed to load resource definitions">
      {{ err }}
    </TAlert>

    <div v-else-if="loading" class="flex grow items-center justify-center">
      <TSpinner class="size-6" />
    </div>

    <TAlert v-else-if="!definitions.length" type="info" title="No resource types match" />

    <div v-else class="min-h-0 grow overflow-auto">
      <TableRoot class="w-full">
        <template #head>
          <TableRow>
            <TableCell th>Type</TableCell>
            <TableCell th>Namespace</TableCell>
            <TableCell th>Sensitivity</TableCell>
            <TableCell th>Aliases</TableCell>
          </TableRow>
        </template>

        <template #body>
          <TableRow v-for="spec in definitions" :key="spec.type" class="relative hover:bg-white/5">
            <TableCell>
              <!-- Stretched over the row so the whole row is clickable while staying a real link -->
              <RouterLink
                :to="resourceRoute(spec.type!)"
                class="text-naturals-n14 after:absolute after:inset-0"
              >
                <WordHighlighter
                  :query="search"
                  :text-to-highlight="spec.displayType"
                  highlight-class="bg-naturals-n14"
                />
              </RouterLink>
              <WordHighlighter
                class="block font-mono text-naturals-n10"
                :query="search"
                :text-to-highlight="spec.type"
                highlight-class="bg-naturals-n14"
              />
            </TableCell>
            <TableCell>
              <WordHighlighter
                :query="search"
                :text-to-highlight="spec.defaultNamespace"
                highlight-class="bg-naturals-n14"
              />
            </TableCell>
            <TableCell>
              <span v-if="spec.sensitivity === 'sensitive'" class="resource-label label-orange">
                Sensitive
              </span>
            </TableCell>
            <TableCell class="text-naturals-n10">
              {{ spec.aliases?.join(', ') }}
            </TableCell>
          </TableRow>
        </template>
      </TableRoot>
    </div>
  </PageContainer>
</template>
