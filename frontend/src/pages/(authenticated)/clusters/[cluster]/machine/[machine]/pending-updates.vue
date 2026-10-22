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
import { installImageDiff } from '@/methods/installimage'
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

const diffEntries = computed<DiffEntry[]>(() => {
  const spec = data.value?.spec
  const entries: DiffEntry[] = []

  if (spec?.upgrade) {
    entries.push({
      id: `${machineId.value}-install-image`,
      diff: installImageDiff(
        {
          talos_version: spec.upgrade.from_version,
          schematic_id: spec.upgrade.from_schematic,
          image_factory_host: spec.upgrade.from_image_factory_host,
        },
        {
          talos_version: spec.upgrade.to_version,
          schematic_id: spec.upgrade.to_schematic,
          image_factory_host: spec.upgrade.to_image_factory_host,
        },
      ),
      label: 'Install image',
    })
  }

  if (spec?.config_diff) {
    entries.push({ id: machineId.value, diff: spec.config_diff, label: 'Machine config' })
  }

  return entries
})

useTitle('Pending Updates')
</script>

<template>
  <PageContainer class="flex h-full flex-col gap-compact">
    <template v-if="!loading">
      <TAlert v-if="!diffEntries.length" type="info" title="No Records">
        No pending updates found for this machine
      </TAlert>

      <DiffRenderer v-else class="h-full" :diffs="diffEntries" with-search />
    </template>

    <TSpinner v-else class="mx-auto my-section size-6" />
  </PageContainer>
</template>
