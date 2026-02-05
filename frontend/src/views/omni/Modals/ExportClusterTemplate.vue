<!--
Copyright (c) 2026 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import { computed } from 'vue'
import { useRoute, useRouter } from 'vue-router'

import { Runtime } from '@/api/common/omni.pb'
import type { ClusterSpec } from '@/api/omni/specs/omni.pb'
import { ClusterType, DefaultNamespace } from '@/api/resources'
import TButton from '@/components/common/Button/TButton.vue'
import CodeBlock from '@/components/common/CodeBlock/CodeBlock.vue'
import { getDocsLink } from '@/methods'
import { useResourceWatch } from '@/methods/useResourceWatch'
import ManagedByTemplatesWarning from '@/views/cluster/ManagedByTemplatesWarning.vue'
import CloseButton from '@/views/omni/Modals/CloseButton.vue'

const router = useRouter()
const route = useRoute()

let closed = false

const close = () => {
  if (closed) {
    return
  }

  closed = true

  router.go(-1)
}

const clusterId = computed(() => route.params.cluster as string)

const { data: cluster } = useResourceWatch<ClusterSpec>(() => ({
  runtime: Runtime.Omni,
  resource: {
    type: ClusterType,
    namespace: DefaultNamespace,
    id: clusterId.value,
  },
}))
</script>

<template>
  <div class="modal-window">
    <div class="heading">
      <h3 class="text-base text-naturals-n14">Export Cluster Template</h3>
      <CloseButton @click="close" />
    </div>

    <div class="mb-5 flex flex-col gap-2 text-sm">
      <ManagedByTemplatesWarning
        :resource="cluster"
        warning-text="This cluster is already managed using cluster templates. Make sure any external changes to templates have already been applied before exporting this template again, or your changes will be lost."
      />

      <p>
        You can export the
        <a
          class="link-primary"
          :href="getDocsLink('omni', '/reference/cluster-templates')"
          target="_blank"
          rel="noopener noreferrer"
        >
          cluster template
        </a>
        for this cluster using the following
        <code>omnictl</code>
        command
      </p>
      <CodeBlock :code="`omnictl cluster template export -c ${clusterId}`" />
    </div>

    <div class="flex justify-end">
      <TButton class="h-9 w-32" @click="close">Close</TButton>
    </div>
  </div>
</template>

<style scoped>
@reference "../../../index.css";

.heading {
  @apply mb-5 flex items-center justify-between text-xl text-naturals-n14;
}
</style>
