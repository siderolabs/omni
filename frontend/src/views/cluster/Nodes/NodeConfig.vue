<!--
Copyright (c) 2025 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import { computed, defineAsyncComponent } from 'vue'
import { useRoute } from 'vue-router'

import { Runtime } from '@/api/common/omni.pb'
import type { RedactedClusterMachineConfigSpec } from '@/api/omni/specs/omni.pb'
import { DefaultNamespace, RedactedClusterMachineConfigType } from '@/api/resources'
import { getContext } from '@/context'
import { useResourceWatch } from '@/methods/useResourceWatch'

const CodeEditor = defineAsyncComponent(
  () => import('@/components/common/CodeEditor/CodeEditor.vue'),
)

const route = useRoute()
const context = getContext(route)

const { data: configs } = useResourceWatch<RedactedClusterMachineConfigSpec>(() => ({
  runtime: Runtime.Omni,
  resource: {
    type: RedactedClusterMachineConfigType,
    namespace: DefaultNamespace,
    id: route.params.machine as string,
  },
  context,
}))

const config = computed(() => {
  return configs.value?.[0]?.spec?.data || ''
})
</script>

<template>
  <CodeEditor v-model:value="config" :options="{ readOnly: true }" />
</template>

<style>
@reference "../../../index.css";

.monaco-editor-vue3 h4 {
  @apply font-bold;
}
</style>
