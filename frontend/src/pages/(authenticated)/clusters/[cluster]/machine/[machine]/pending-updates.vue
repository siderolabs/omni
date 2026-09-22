<!--
Copyright (c) 2026 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import { computed } from 'vue'
import { useRoute } from 'vue-router'

import { Runtime } from '@/api/common/omni.pb'
import type { MachinePendingUpdatesSpec } from '@/api/omni/specs/omni.pb'
import { DefaultNamespace, MachinePendingUpdatesType } from '@/api/resources'
import DiffRenderer, { type DiffEntry } from '@/components/DiffRenderer'
import PageContainer from '@/components/PageContainer/PageContainer.vue'
import TSpinner from '@/components/Spinner/TSpinner.vue'
import TAlert from '@/components/TAlert.vue'
import { useTitle } from '@/methods/title'
import { useResourceWatch } from '@/methods/useResourceWatch'

definePage({ name: 'NodePendingUpdates' })

const route = useRoute()

const machineId = computed(() => route.params.machine.toString())

const { data, loading } = useResourceWatch<MachinePendingUpdatesSpec>(() => ({
  runtime: Runtime.Omni,
  resource: {
    namespace: DefaultNamespace,
    type: MachinePendingUpdatesType,
    id: machineId.value,
  },
}))

const diffEntries = computed<DiffEntry[]>(() =>
  data.value?.spec.config_diff ? [{ id: machineId.value, diff: data.value.spec.config_diff }] : [],
)

useTitle('Pending Updates')
</script>

<template>
  <PageContainer class="flex h-full flex-col gap-4">
    <template v-if="!loading">
      <TAlert v-if="!diffEntries.length" type="info" title="No Records">
        No pending config updates found for this machine
      </TAlert>

      <DiffRenderer v-else class="h-full" :diffs="diffEntries" with-search />
    </template>

    <TSpinner v-else class="mx-auto my-8 size-6" />
  </PageContainer>
</template>
