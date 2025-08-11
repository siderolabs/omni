<!--
Copyright (c) 2025 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import type { Ref } from 'vue'
import { onMounted, ref } from 'vue'

import { Runtime } from '@/api/common/omni.pb'
import type { Resource } from '@/api/grpc'
import { ResourceService } from '@/api/grpc'
import { withContext, withRuntime } from '@/api/options'
import {
  TalosK8sNamespace,
  TalosNodenameID,
  TalosNodenameType,
  TalosRuntimeNamespace,
  TalosServiceType,
} from '@/api/resources'
import TAnimation from '@/components/common/Animation/TAnimation.vue'
import TMenuItem from '@/components/common/MenuItem/TMenuItem.vue'
import { getContext } from '@/context'

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

<template>
  <TAnimation>
    <div class="logs">
      <p class="logs-title">Service logs for:</p>
      <p class="logs-node-name truncate">{{ node }}</p>
      <ul class="logs-list">
        <TMenuItem
          v-for="service in services"
          :key="service?.metadata?.id"
          icon="log"
          :is-item-visible="true"
          :name="service?.metadata?.id"
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
  </TAnimation>
</template>

<style scoped>
.logs {
  @apply w-full py-4;
}
.logs-title {
  @apply mb-2 px-6 text-xs text-naturals-N8;
}
.logs-node-name {
  @apply mb-6 px-6 text-xs text-naturals-N13;
  font-size: 13px;
}
.logs-item {
  @apply mb-4 flex w-full cursor-pointer items-center justify-start;
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
