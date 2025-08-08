<!--
Copyright (c) 2025 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<template>
  <t-animation>
    <div class="logs">
      <p class="logs-title">Service logs for:</p>
      <p class="logs-node-name truncate">{{ node }}</p>
      <ul class="logs-list">
        <t-menu-item
          icon="log"
          :isItemVisible="true"
          v-for="service in services"
          :name="service?.metadata?.id"
          :key="service?.metadata?.id"
          :route="{
            name: 'Logs',
            query: {
              cluster: $route.query.cluster,
              namespace: $route.query.namespace,
              uid: $route.query.uid,
            },
            params: {
              machine: $route.params.machine,
              service: service?.metadata?.id,
            },
          }"
        />
      </ul>
    </div>
  </t-animation>
</template>

<script setup lang="ts">
import type { Ref } from 'vue'
import { ref, onMounted } from 'vue'

import TMenuItem from '@/components/common/MenuItem/TMenuItem.vue'
import { getContext } from '@/context'
import type { Resource } from '@/api/grpc'
import { ResourceService } from '@/api/grpc'
import { Runtime } from '@/api/common/omni.pb'
import TAnimation from '@/components/common/Animation/TAnimation.vue'
import {
  TalosK8sNamespace,
  TalosNodenameID,
  TalosNodenameType,
  TalosRuntimeNamespace,
  TalosServiceType,
} from '@/api/resources'
import { withContext, withRuntime } from '@/api/options'

defineProps<{ name: string }>()

const services: Ref<any[]> = ref([])
const node = ref()
const context = getContext()

onMounted(async () => {
  const response = await ResourceService.List(
    {
      type: TalosServiceType,
      namespace: TalosRuntimeNamespace,
    },
    withRuntime(Runtime.Talos),
    withContext(context),
  )

  for (const items of response) {
    services.value = services.value.concat(items)
  }
})

onMounted(async () => {
  interface Nodename {
    nodename?: string
  }

  const nodename: Resource<Nodename> = await ResourceService.Get(
    {
      type: TalosNodenameType,
      id: TalosNodenameID,
      namespace: TalosK8sNamespace,
    },
    withRuntime(Runtime.Talos),
    withContext(context),
  )

  node.value = nodename.spec.nodename
})
</script>

<style scoped>
.logs {
  @apply w-full py-4;
}
.logs-title {
  @apply text-xs text-naturals-N8 mb-2 px-6;
}
.logs-node-name {
  @apply text-xs text-naturals-N13 mb-6 px-6;
  font-size: 13px;
}
.logs-item {
  @apply w-full flex justify-start items-center mb-4 cursor-pointer;
}
.logs-icon {
  @apply mr-4 fill-current text-naturals-N10;
  width: 16px;
  height: 16px;
}
.logs-log-name {
  @apply text-xs text-naturals-N10;
}
</style>
