<!--
Copyright (c) 2025 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<template>
  <div class="modal-window">
    <div class="heading">
      <h3 class="text-base text-naturals-N14">
        Destroy the Machine Set {{ $route.query.machineSet }} ?
      </h3>
      <close-button @click="close(true)" />
    </div>
    <managed-by-templates-warning warning-style="popup" />
    <p class="text-xs mb-2">Please confirm the action.</p>
    <div v-if="warning" class="text-yellow-Y1 text-xs mt-3">{{ warning }}</div>
    <div class="flex items-end gap-4 mt-2">
      <div class="flex-1" />
      <t-button @click="destroyMachineSet" class="w-32 h-9">
        <span>Destroy</span>
      </t-button>
    </div>
  </div>
</template>

<script setup lang="ts">
import type { Ref } from 'vue'
import { ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { showError, showSuccess } from '@/notification'

import CloseButton from '@/views/omni/Modals/CloseButton.vue'
import TButton from '@/components/common/Button/TButton.vue'
import { ResourceService } from '@/api/grpc'
import { DefaultNamespace, MachineSetType } from '@/api/resources'
import { Runtime } from '@/api/common/omni.pb'
import { withRuntime } from '@/api/options'
import ManagedByTemplatesWarning from '@/views/cluster/ManagedByTemplatesWarning.vue'

const router = useRouter()
const route = useRoute()
const warning: Ref<string | null> = ref(null)

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

const destroyMachineSet = async () => {
  try {
    await ResourceService.Teardown(
      {
        id: route.query.machineSet as string,
        namespace: DefaultNamespace,
        type: MachineSetType,
      },
      withRuntime(Runtime.Omni),
    )
  } catch (e) {
    close(true)

    showError('Failed to Destroy The Machine Set', e.message)

    return
  }

  close(true)

  showSuccess(
    `The Machine Set ${route.query.machineSet} was Removed From the Cluster`,
    'The machine set is being torn down',
  )
}
</script>

<style scoped>
.heading {
  @apply flex justify-between items-center mb-5 text-xl text-naturals-N14;
}
</style>
