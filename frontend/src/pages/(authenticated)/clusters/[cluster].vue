<!--
Copyright (c) 2026 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import { useRoute } from 'vue-router'

import { Runtime } from '@/api/common/omni.pb'
import type { ClusterSpec } from '@/api/omni/specs/omni.pb'
import { ClusterType, DefaultNamespace } from '@/api/resources'
import PageContainer from '@/components/PageContainer/PageContainer.vue'
import TAlert from '@/components/TAlert.vue'
import { useResourceWatch } from '@/methods/useResourceWatch'

const route = useRoute()

const { data: cluster, loading } = useResourceWatch<ClusterSpec>(() => ({
  runtime: Runtime.Omni,
  resource: {
    type: ClusterType,
    namespace: DefaultNamespace,
    id: route.params.cluster,
  },
}))
</script>

<template>
  <RouterView v-if="cluster" :current-cluster="cluster" />
  <PageContainer v-else-if="!loading" class="font-sm flex-1">
    <TAlert title="Cluster Not Found" type="error">
      Cluster {{ route.params.cluster }} does not exist.
    </TAlert>
  </PageContainer>
</template>
