<!--
Copyright (c) 2026 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import { computed } from 'vue'
import { useRoute } from 'vue-router'

import { Runtime } from '@/api/common/omni.pb'
import type { MachineConfigDiffSpec } from '@/api/omni/specs/omni.pb'
import { DefaultNamespace, LabelMachine, MachineConfigDiffType } from '@/api/resources'
import DiffRenderer from '@/components/common/DiffRenderer/DiffRenderer.vue'
import TSpinner from '@/components/common/Spinner/TSpinner.vue'
import TAlert from '@/components/TAlert.vue'
import { formatISO } from '@/methods/time'
import { useResourceWatch } from '@/methods/useResourceWatch'

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

const combinedDiff = computed(() =>
  configDiffs.value
    .map((d) => `# Created on ${formatISO(d.metadata.created!)}\n${d.spec.diff}`)
    .join('\n'),
)
</script>

<template>
  <div class="h-full py-4">
    <template v-if="!diffsLoading">
      <TAlert v-if="!combinedDiff" type="info" title="No Records">
        No previously applied config diffs found for this machine
      </TAlert>

      <DiffRenderer v-else class="h-full" :diff="combinedDiff" with-search />
    </template>

    <TSpinner v-else class="mx-auto my-8 size-6" />
  </div>
</template>
