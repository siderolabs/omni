<!--
Copyright (c) 2025 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import { computed } from 'vue'
import { useRoute } from 'vue-router'

import { Runtime } from '@/api/common/omni.pb'
import type { Resource } from '@/api/grpc'
import type { ClusterSpec } from '@/api/omni/specs/omni.pb'
import { ClusterType, DefaultNamespace, ResourceManagedByClusterTemplates } from '@/api/resources'
import TAlert from '@/components/TAlert.vue'
import { useResourceWatch } from '@/methods/useResourceWatch'

export type WarningStyle = 'alert' | 'popup' | 'short'

type Props = {
  resource?: Resource
  warningStyle?: WarningStyle
}

const { resource, warningStyle = 'alert' } = defineProps<Props>()
const route = useRoute()

const { data: routeResource } = useResourceWatch<ClusterSpec>(() => ({
  // If the resource is not passed explicitly, default to the cluster in the route, if exists.
  skip: !!resource || !route.params.cluster,
  resource: {
    type: ClusterType,
    namespace: DefaultNamespace,
    id: route.params.cluster as string,
  },
  runtime: Runtime.Omni,
}))

const activeResource = computed(() => resource || routeResource.value)

const resourceIsManagedByTemplates = computed(() => {
  return (
    activeResource.value?.metadata?.annotations?.[ResourceManagedByClusterTemplates] !== undefined
  )
})

const resourceWord = computed(() => {
  return activeResource.value?.metadata?.type === ClusterType ? 'cluster' : 'resource'
})
</script>

<template>
  <template v-if="resourceIsManagedByTemplates">
    <div v-if="warningStyle === 'alert'" class="pb-5">
      <TAlert type="warn" :title="`This ${resourceWord} is managed using cluster templates.`">
        It is recommended to manage it using its template and not through this UI.
      </TAlert>
    </div>
    <div v-else-if="warningStyle === 'popup'" class="pb-5 text-xs">
      <p class="py-2 text-primary-p3">
        This {{ resourceWord }} is managed using cluster templates.
      </p>
      <p class="py-2 font-bold text-primary-p3">
        It is recommended to manage it using its template and not through this UI.
      </p>
    </div>
    <div v-else class="text-xs">
      <p class="py-2 text-primary-p3">Managed using cluster templates</p>
    </div>
  </template>
</template>
