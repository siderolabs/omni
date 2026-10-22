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
import DiffRenderer, { type DiffEntry } from '@/components/DiffRenderer'
import PageContainer from '@/components/PageContainer/PageContainer.vue'
import TSelectList from '@/components/SelectList/TSelectList.vue'
import TSpinner from '@/components/Spinner/TSpinner.vue'
import TAlert from '@/components/TAlert.vue'
import { installImageDiff } from '@/methods/installimage'
import { formatISO } from '@/methods/time'
import { useTitle } from '@/methods/title'
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

const diffEntries = computed<DiffEntry[]>(() =>
  configDiffs.value
    .toSorted((a, b) =>
      sortOrder.value === 'desc'
        ? compareDesc(parseISO(a.metadata.created!), parseISO(b.metadata.created!))
        : compareAsc(parseISO(a.metadata.created!), parseISO(b.metadata.created!)),
    )
    .flatMap((d) => {
      const created = d.metadata.created ? formatISO(d.metadata.created) : undefined
      const entries: DiffEntry[] = []

      if (d.spec.from_install_image) {
        entries.push({
          id: `${d.metadata.id!}-install-image`,
          diff: installImageDiff(d.spec.from_install_image, d.spec.to_install_image),
          label: created ? `Install image changed on ${created}` : undefined,
        })
      }

      if (d.spec.diff) {
        entries.push({
          id: d.metadata.id!,
          diff: d.spec.diff,
          label: created ? `Patch created on ${created}` : undefined,
        })
      }

      return entries
    }),
)

useTitle('Change History')
</script>

<template>
  <PageContainer class="h-full">
    <template v-if="!diffsLoading">
      <TAlert v-if="!diffEntries.length" type="info" title="No Records">
        No history found for this machine
      </TAlert>

      <DiffRenderer v-else class="h-full" :diffs="diffEntries" with-search>
        <template #extra-controls>
          <TSelectList v-model="sortOrder" title="Sort by" :values="sortOptions" />
        </template>
      </DiffRenderer>
    </template>

    <TSpinner v-else class="mx-auto my-section size-6" />
  </PageContainer>
</template>
