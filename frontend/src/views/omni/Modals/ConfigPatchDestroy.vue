<!--
Copyright (c) 2025 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import { ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'

import { Runtime } from '@/api/common/omni.pb'
import { ResourceService } from '@/api/grpc'
import { withRuntime } from '@/api/options'
import { ConfigPatchType, DefaultNamespace } from '@/api/resources'
import TButton from '@/components/common/Button/TButton.vue'
import { showError, showSuccess } from '@/notification'
import ManagedByTemplatesWarning from '@/views/cluster/ManagedByTemplatesWarning.vue'
import CloseButton from '@/views/omni/Modals/CloseButton.vue'

const router = useRouter()
const route = useRoute()
const scalingDown = ref(false)

let closed = false

const close = () => {
  if (closed) {
    return
  }

  closed = true

  router.go(-1)
}

const destroyPatch = async () => {
  try {
    await ResourceService.Delete(
      {
        namespace: DefaultNamespace,
        type: ConfigPatchType,
        id: route.query.id as string,
      },
      withRuntime(Runtime.Omni),
    )
  } catch (e) {
    showError('Failed to Destroy The Config Patch', e.message)

    return
  } finally {
    close()
  }

  showSuccess(`The Config Patch ${route.query.id} was Destroyed`)
}
</script>

<template>
  <div class="modal-window">
    <div class="heading">
      <h3 class="text-base text-naturals-N14">Destroy the Config Patch {{ route.query.id }} ?</h3>
      <CloseButton @click="close" />
    </div>
    <ManagedByTemplatesWarning warning-style="popup" />
    <p class="text-xs">Please confirm the action.</p>
    <div class="mt-8 flex justify-end gap-4">
      <TButton class="h-9 w-32" :disabled="scalingDown" @click="destroyPatch"> Destroy </TButton>
    </div>
  </div>
</template>

<style scoped>
.heading {
  @apply mb-5 flex items-center justify-between text-xl text-naturals-N14;
}
</style>
