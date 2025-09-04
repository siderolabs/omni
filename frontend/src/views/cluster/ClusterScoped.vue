<!--
Copyright (c) 2025 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import { computed, ref } from 'vue'
import { useRoute } from 'vue-router'

import { Runtime } from '@/api/common/omni.pb'
import type { Resource } from '@/api/grpc'
import { EventType } from '@/api/omni/resources/resources.pb'
import type { ClusterSpec } from '@/api/omni/specs/omni.pb'
import { ClusterType, DefaultNamespace } from '@/api/resources'
import Watch from '@/api/watch'
import TAlert from '@/components/TAlert.vue'
import { getContext } from '@/context'

const bootstrapped = ref(false)
const cluster = ref<Resource<ClusterSpec>>()

const watch = new Watch(cluster, (message) => {
  if (message.event?.event_type === EventType.BOOTSTRAPPED) {
    bootstrapped.value = true
  }
})

const route = useRoute()
const context = getContext(route)

watch.setup(
  computed(() => {
    return {
      runtime: Runtime.Omni,
      resource: {
        type: ClusterType,
        namespace: DefaultNamespace,
        id: route.params.cluster as string,
      },
      context,
    }
  }),
)
</script>

<template>
  <RouterView v-if="cluster" :current-cluster="cluster" />
  <div v-else-if="bootstrapped" class="font-sm flex-1">
    <TAlert title="Cluster Not Found" type="error">
      Cluster {{ route.params.cluster as string }} does not exist.
    </TAlert>
  </div>
</template>
