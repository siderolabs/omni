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
import type { ClusterSpec } from '@/api/omni/specs/omni.pb'
import { ClusterType, DefaultNamespace, ResourceManagedByClusterTemplates } from '@/api/resources'
import Watch from '@/api/watch'
import TAlert from '@/components/TAlert.vue'

export type WarningStyle = 'alert' | 'popup' | 'short'

type Props = {
  cluster?: Resource<ClusterSpec>
  warningStyle?: WarningStyle
}

const props = withDefaults(defineProps<Props>(), {
  warningStyle: 'alert',
})

const cluster: Ref<Resource<ClusterSpec> | undefined> = ref(props.cluster)

// If the cluster is not passed explicitly, watch it from the route, if cluster exists in the route params.
if (!props.cluster) {
  const route = useRoute()
  const clusterWatch = new Watch(cluster)

  clusterWatch.setup(
    computed(() => {
      if (!route.params.cluster) {
        return undefined
      }

      return {
        resource: {
          type: ClusterType,
          namespace: DefaultNamespace,
          id: route.params.cluster as string,
        },
        runtime: Runtime.Omni,
      }
    }),
  )
}

const clusterIsManagedByTemplates = computed(() => {
  return cluster.value?.metadata?.annotations?.[ResourceManagedByClusterTemplates] !== undefined
})
</script>

<template>
  <template v-if="clusterIsManagedByTemplates">
    <div v-if="warningStyle === 'alert'" class="pb-5">
      <TAlert type="warn" title="This cluster is managed using cluster templates.">
        It is recommended to manage it using its template and not through this UI.
      </TAlert>
    </div>
    <div v-else-if="warningStyle === 'popup'" class="pb-5 text-xs">
      <p class="py-2 text-primary-P3">This cluster is managed using cluster templates.</p>
      <p class="py-2 font-bold text-primary-P3">
        It is recommended to manage it using its template and not through this UI.
      </p>
    </div>
    <div v-else class="text-xs">
      <p class="py-2 text-primary-P3">Managed using cluster templates</p>
    </div>
  </template>
</template>
