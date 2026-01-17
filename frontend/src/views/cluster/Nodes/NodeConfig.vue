<!--
Copyright (c) 2025 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import type { Ref } from 'vue'
import { computed, defineAsyncComponent, ref } from 'vue'
import { useRoute } from 'vue-router'

import { Runtime } from '@/api/common/omni.pb'
import type { Resource } from '@/api/grpc'
import type { ClusterSpec, RedactedClusterMachineConfigSpec } from '@/api/omni/specs/omni.pb'
import { ClusterType, DefaultNamespace, RedactedClusterMachineConfigType } from '@/api/resources'
import Watch from '@/api/watch'
import { getContext } from '@/context'
import { useResourceWatch } from '@/methods/useResourceWatch'

const CodeEditor = defineAsyncComponent(
  () => import('@/components/common/CodeEditor/CodeEditor.vue'),
)

const route = useRoute()

const configs: Ref<Resource<RedactedClusterMachineConfigSpec>[]> = ref([])
const watch = new Watch(configs)

const config = computed(() => {
  return configs.value?.[0]?.spec?.data || ''
})

const context = getContext(route)

watch.setup(
  computed(() => {
    return {
      runtime: Runtime.Omni,
      resource: {
        type: RedactedClusterMachineConfigType,
        namespace: DefaultNamespace,
        id: route.params.machine as string,
      },
      context,
    }
  }),
)

const { data: cluster } = useResourceWatch<ClusterSpec>(() => ({
  runtime: Runtime.Omni,
  resource: {
    type: ClusterType,
    namespace: DefaultNamespace,
    id: route.params.cluster as string,
  },
}))
</script>

<template>
  <CodeEditor
    v-model:value="config"
    :options="{ readOnly: true }"
    :talos-version="cluster?.spec.talos_version"
  />
</template>

<style>
@reference "../../../index.css";

.monaco-editor-vue3 h4 {
  @apply font-bold;
}
</style>
