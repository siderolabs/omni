<!--
Copyright (c) 2026 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import { computed, defineAsyncComponent } from 'vue'
import { useRoute } from 'vue-router'

import { Runtime } from '@/api/common/omni.pb'
import type { ClusterSpec, RedactedClusterMachineConfigSpec } from '@/api/omni/specs/omni.pb'
import { ClusterType, DefaultNamespace, RedactedClusterMachineConfigType } from '@/api/resources'
import { useResourceWatch } from '@/methods/useResourceWatch'

const CodeEditor = defineAsyncComponent(
  () => import('@/components/common/CodeEditor/CodeEditor.vue'),
)

const route = useRoute()

const { data: configResource } = useResourceWatch<RedactedClusterMachineConfigSpec>(() => ({
  runtime: Runtime.Omni,
  resource: {
    type: RedactedClusterMachineConfigType,
    namespace: DefaultNamespace,
    id: route.params.machine as string,
  },
}))

const { data: cluster } = useResourceWatch<ClusterSpec>(() => ({
  runtime: Runtime.Omni,
  resource: {
    type: ClusterType,
    namespace: DefaultNamespace,
    id: route.params.cluster as string,
  },
}))

const config = computed(() => configResource.value?.spec.data ?? '')
</script>

<template>
  <CodeEditor
    v-model:value="config"
    :options="{ readOnly: true }"
    :talos-version="cluster?.spec.talos_version"
    class="py-4"
  />
</template>

<style>
@reference "../../../index.css";

.monaco-editor-vue3 h4 {
  @apply font-bold;
}
</style>
