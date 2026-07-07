<!--
Copyright (c) 2026 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import { useRouteQuery } from '@vueuse/router'
import { compareAsc, compareDesc, parseISO } from 'date-fns'
import { computed } from 'vue'
import { useRoute } from 'vue-router'

import { Runtime } from '@/api/common/omni.pb'
import type { MachineConfigDiffSpec } from '@/api/omni/specs/omni.pb'
import { DefaultNamespace, LabelMachine, MachineConfigDiffType } from '@/api/resources'
import DiffRenderer from '@/components/DiffRenderer/DiffRenderer.vue'
import PageContainer from '@/components/PageContainer/PageContainer.vue'
import TSelectList from '@/components/SelectList/TSelectList.vue'
import TSpinner from '@/components/Spinner/TSpinner.vue'
import TAlert from '@/components/TAlert.vue'
import { formatISO } from '@/methods/time'
import { useResourceWatch } from '@/methods/useResourceWatch'

definePage({ name: 'NodeConfigDiffs' })

const route = useRoute()

const machineId = computed(() => route.params.machine.toString())

const { data: configDiffs, loading: diffsLoading } = useResourceWatch<MachineConfigDiffSpec>(
  () => ({
    runtime: Runtime.Omni,
    resource: {
      namespace: DefaultNamespace,
      type: MachineConfigDiffType,
    },
    selectors: [`${LabelMachine}=${machineId.value}`],
  }),
)

const sortOrder = useRouteQuery<'asc' | 'desc'>('sort', 'desc')

const sortOptions = [
  { label: 'Creation Time ⬇', value: 'desc' as const },
  { label: 'Creation Time ⬆', value: 'asc' as const },
]

const combinedDiff = computed(() =>
  configDiffs.value
    .toSorted((a, b) =>
      sortOrder.value === 'desc'
        ? compareDesc(parseISO(a.metadata.created!), parseISO(b.metadata.created!))
        : compareAsc(parseISO(a.metadata.created!), parseISO(b.metadata.created!)),
    )
    .map((d) => `# Created on ${formatISO(d.metadata.created!)}\n${d.spec.diff}`)
    .join('\n'),
)
</script>

<template>
  <PageContainer class="h-full">
    <template v-if="!diffsLoading">
      <TAlert v-if="!combinedDiff" type="info" title="No Records">
        No previously applied config diffs found for this machine
      </TAlert>

      <DiffRenderer v-else class="h-full" :diff="combinedDiff" with-search>
        <template #extra-controls>
          <TSelectList v-model="sortOrder" title="Sort by" :values="sortOptions" />
        </template>
      </DiffRenderer>
    </template>

    <TSpinner v-else class="mx-auto my-8 size-6" />
  </PageContainer>
</template>
