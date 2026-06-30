<!--
Copyright (c) 2026 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import { computed } from 'vue'
import { useRoute } from 'vue-router'

import { Runtime } from '@/api/common/omni.pb'
import type { Resource } from '@/api/grpc'
import type { ClusterSpec } from '@/api/omni/specs/omni.pb'
import {
  ClusterType,
  DefaultNamespace,
  ResourceManagedByClusterTemplates,
  ResourceManagedByGitopsTools,
} from '@/api/resources'
import TAlert from '@/components/TAlert.vue'
import { useResourceWatch } from '@/methods/useResourceWatch'

export type WarningStyle = 'alert' | 'popup' | 'short'

type Props = {
  resource?: Resource
  warningStyle?: WarningStyle
  warningTextTemplates?: string
  warningTextOthers?: string
}

const {
  resource,
  warningStyle = 'alert',
  warningTextTemplates,
  warningTextOthers,
} = defineProps<Props>()

const route = useRoute()
const routeCluster = computed(() => ('cluster' in route.params ? route.params.cluster : undefined))

const { data: routeResource } = useResourceWatch<ClusterSpec>(() => ({
  // If the resource is not passed explicitly, default to the cluster in the route, if exists.
  skip: !!resource || !routeCluster.value,
  resource: {
    type: ClusterType,
    namespace: DefaultNamespace,
    id: routeCluster.value!,
  },
  runtime: Runtime.Omni,
}))

const activeResource = computed(() => resource || routeResource.value)

const resourceIsManagedByTemplates = computed(() => {
  return (
    activeResource.value?.metadata?.annotations?.[ResourceManagedByClusterTemplates] !== undefined
  )
})

const resourceIsManagedByGitops = computed(() => {
  return activeResource.value?.metadata?.annotations?.[ResourceManagedByGitopsTools] !== undefined
})

const warningText = computed(() => {
  if (resourceIsManagedByTemplates.value) {
    if (warningTextTemplates) {
      return warningTextTemplates
    }

    return 'It is recommended to manage it using its template and not through this UI.'
  } else if (resourceIsManagedByGitops.value) {
    if (warningTextOthers) {
      return warningTextOthers
    }

    return 'It is recommended to manage it using the original scripts and not through this UI.'
  }
  return ''
})

const managedByString = computed(() => {
  if (resourceIsManagedByTemplates.value) {
    return 'cluster templates'
  } else if (resourceIsManagedByGitops.value) {
    const tool = activeResource.value?.metadata?.annotations?.[ResourceManagedByGitopsTools]

    return tool ?? 'an external GitOps tool'
  }
  return ''
})

const resourceWord = computed(() => {
  return activeResource.value?.metadata?.type === ClusterType ? 'cluster' : 'resource'
})
</script>

<template>
  <template v-if="resourceIsManagedByTemplates || resourceIsManagedByGitops">
    <div v-if="warningStyle === 'alert'" class="pb-5">
      <TAlert type="warn" :title="`This ${resourceWord} is managed using ${managedByString}.`">
        {{ warningText }}
      </TAlert>
    </div>
    <div v-else-if="warningStyle === 'popup'" class="pb-5 text-xs">
      <p class="py-2 text-primary-p3">
        This {{ resourceWord }} is managed using {{ managedByString }}.
      </p>
      <p class="py-2 font-bold text-primary-p3">
        {{ warningText }}
      </p>
    </div>
    <div v-else class="text-xs">
      <p class="py-2 text-primary-p3">Managed using {{ managedByString }}</p>
    </div>
  </template>
</template>
