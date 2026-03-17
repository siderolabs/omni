<!--
Copyright (c) 2026 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import { ref, watchEffect } from 'vue'
import { useRoute } from 'vue-router'

import { Runtime } from '@/api/common/omni.pb'
import { EventType } from '@/api/omni/resources/resources.pb'
import type { ClusterSpec } from '@/api/omni/specs/omni.pb'
import { ClusterType, DefaultNamespace } from '@/api/resources'
import PageContainer from '@/components/common/PageContainer/PageContainer.vue'
import TAlert from '@/components/TAlert.vue'
import { current } from '@/context'
import { useResourceWatch } from '@/methods/useResourceWatch'

const bootstrapped = ref(false)

const route = useRoute()

watchEffect(() => {
  current.value = route.params.cluster
})

const { data: cluster } = useResourceWatch<ClusterSpec>(
  () => ({
    runtime: Runtime.Omni,
    resource: {
      type: ClusterType,
      namespace: DefaultNamespace,
      id: route.params.cluster,
    },
  }),
  (message) => {
    if (message.event?.event_type === EventType.BOOTSTRAPPED) {
      bootstrapped.value = true
    }
  },
)
</script>

<template>
  <RouterView v-if="cluster" :current-cluster="cluster" />
  <PageContainer v-else-if="bootstrapped" class="font-sm flex-1">
    <TAlert title="Cluster Not Found" type="error">
      Cluster {{ route.params.cluster }} does not exist.
    </TAlert>
  </PageContainer>
</template>
