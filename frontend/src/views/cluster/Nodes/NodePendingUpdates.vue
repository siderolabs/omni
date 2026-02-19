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
import DiffRenderer from '@/components/common/DiffRenderer/DiffRenderer.vue'
import TSpinner from '@/components/common/Spinner/TSpinner.vue'
import TAlert from '@/components/TAlert.vue'
import { useResourceWatch } from '@/methods/useResourceWatch'

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
</script>

<template>
  <div class="flex flex-col gap-4 py-4">
    <template v-if="!loading">
      <TAlert v-if="!data?.spec.config_diff" type="info" title="No Records">
        No pending config updates found for this machine
      </TAlert>

      <DiffRenderer v-else :diff="data.spec.config_diff" with-search />
    </template>

    <TSpinner v-else class="mx-auto my-8 size-6" />
  </div>
</template>
