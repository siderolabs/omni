<!--
Copyright (c) 2026 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import type { Ref } from 'vue'
import { ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'

import { Runtime } from '@/api/common/omni.pb'
import { ResourceService } from '@/api/grpc'
import { withRuntime } from '@/api/options'
import { DefaultNamespace, MachineSetType } from '@/api/resources'
import TButton from '@/components/common/Button/TButton.vue'
import { showError, showSuccess } from '@/notification'
import ManagedByTemplatesWarning from '@/views/cluster/ManagedByTemplatesWarning.vue'
import CloseButton from '@/views/omni/Modals/CloseButton.vue'

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

<template>
  <div class="modal-window">
    <div class="heading">
      <h3 class="text-base text-naturals-n14">
        Destroy the Machine Set {{ $route.query.machineSet }} ?
      </h3>
      <CloseButton @click="close(true)" />
    </div>
    <ManagedByTemplatesWarning warning-style="popup" />
    <p class="mb-2 text-xs">Please confirm the action.</p>
    <div v-if="warning" class="mt-3 text-xs text-yellow-y1">{{ warning }}</div>
    <div class="mt-2 flex items-end gap-4">
      <div class="flex-1" />
      <TButton class="h-9 w-32" @click="destroyMachineSet">
        <span>Destroy</span>
      </TButton>
    </div>
  </div>
</template>

<style scoped>
@reference "../../../index.css";

.heading {
  @apply mb-5 flex items-center justify-between text-xl text-naturals-n14;
}
</style>
