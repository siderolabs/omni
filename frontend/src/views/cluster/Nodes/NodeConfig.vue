<!--
Copyright (c) 2025 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import type { Ref } from 'vue'
import { computed, ref } from 'vue'
import { useRoute } from 'vue-router'

import { Runtime } from '@/api/common/omni.pb'
import type { Resource } from '@/api/grpc'
import type { RedactedClusterMachineConfigSpec } from '@/api/omni/specs/omni.pb'
import { DefaultNamespace, RedactedClusterMachineConfigType } from '@/api/resources'
import Watch from '@/api/watch'
import CodeEditor from '@/components/common/CodeEditor/CodeEditor.vue'
import { getContext } from '@/context'

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
</script>

<template>
  <CodeEditor v-model:value="config" :options="{ readOnly: true }" />
</template>

<style>
.monaco-editor-vue3 h4 {
  @apply font-bold;
}
</style>
