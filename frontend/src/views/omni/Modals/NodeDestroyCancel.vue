<!--
Copyright (c) 2026 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import { useRoute, useRouter } from 'vue-router'

import { Runtime } from '@/api/common/omni.pb'
import type { Resource } from '@/api/grpc'
import type { ClusterMachineSpec } from '@/api/omni/specs/omni.pb'
import { ClusterMachineType, DefaultNamespace } from '@/api/resources'
import TButton from '@/components/common/Button/TButton.vue'
import TSpinner from '@/components/common/Spinner/TSpinner.vue'
import { ClusterCommandError, restoreNode } from '@/methods/cluster'
import { setupNodenameWatch } from '@/methods/node'
import { useResourceWatch } from '@/methods/useResourceWatch'
import { showError, showSuccess } from '@/notification'
import CloseButton from '@/views/omni/Modals/CloseButton.vue'

const router = useRouter()
const route = useRoute()

const { data, loading } = useResourceWatch<ClusterMachineSpec>({
  skip: !route.query.machine,
  resource: {
    namespace: DefaultNamespace,
    type: ClusterMachineType,
    id: route.query.machine?.toString() ?? '',
  },
  runtime: Runtime.Omni,
})

let closed = false

const close = (goBack?: boolean) => {
  if (closed) {
    return
  }

  if (!goBack && !route.query.goback) {
    router.push({ name: 'ClusterOverview', params: { cluster: route.query.cluster as string } })

    return
  }

  closed = true

  router.go(-1)
}

const node = setupNodenameWatch(route.query.machine as string)

const restore = async (clusterMachine: Resource<ClusterMachineSpec>) => {
  if (!route.query.machine) {
    showError('Failed to Restore The Machine Set Node', 'The machine id not resolved')

    close(true)

    return
  }

  try {
    await restoreNode(clusterMachine)
  } catch (e) {
    if (e instanceof ClusterCommandError) {
      showError(e.errorNotification.title, e.errorNotification.details)

      close(true)

      return
    }

    close(true)

    showError('Failed to Restore The Node', e.message)

    return
  }

  close()

  showSuccess(`The Machine ${node.value} was Restored`)
}
</script>

<template>
  <div class="modal-window">
    <div class="heading">
      <h3 class="text-base text-naturals-n14">
        Cancel Destroy of Node {{ node ?? $route.query.machine }} ?
      </h3>
      <CloseButton @click="close(true)" />
    </div>

    <div v-if="loading" class="flex size-full items-center justify-center">
      <TSpinner class="size-6" />
    </div>

    <template v-else-if="data && data.metadata.phase !== 'Running'">
      <p class="mb-2 text-xs">Please confirm the action.</p>

      <div class="mt-2 flex items-end gap-4">
        <div class="flex-1" />
        <TButton class="h-9" @click="restore(data)">Restore Machine</TButton>
      </div>
    </template>

    <template v-else>
      <p class="mb-2 text-xs">Restoring the machine is not possible at this stage.</p>

      <div class="mt-2 flex items-end gap-4">
        <div class="flex-1" />
        <TButton class="h-9" @click="close(true)">Close</TButton>
      </div>
    </template>
  </div>
</template>

<style scoped>
@reference "../../../index.css";

.heading {
  @apply mb-5 flex items-center justify-between text-xl text-naturals-n14;
}
</style>
