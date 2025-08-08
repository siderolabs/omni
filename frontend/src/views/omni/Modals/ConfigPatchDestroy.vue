<!--
Copyright (c) 2025 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<template>
  <div class="modal-window">
    <div class="heading">
      <h3 class="text-base text-naturals-N14">Destroy the Config Patch {{ route.query.id }} ?</h3>
      <close-button @click="close" />
    </div>
    <managed-by-templates-warning warning-style="popup" />
    <p class="text-xs">Please confirm the action.</p>
    <div class="flex justify-end gap-4 mt-8">
      <t-button @click="destroyPatch" class="w-32 h-9" :disabled="scalingDown"> Destroy </t-button>
    </div>
  </div>
</template>

<script setup lang="ts">
import { DefaultNamespace, ConfigPatchType } from '@/api/resources'
import { ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { showError, showSuccess } from '@/notification'
import { withRuntime } from '@/api/options'
import { Runtime } from '@/api/common/omni.pb'
import { ResourceService } from '@/api/grpc'

import TButton from '@/components/common/Button/TButton.vue'
import CloseButton from '@/views/omni/Modals/CloseButton.vue'
import ManagedByTemplatesWarning from '@/views/cluster/ManagedByTemplatesWarning.vue'

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

<style scoped>
.heading {
  @apply flex justify-between items-center mb-5 text-xl text-naturals-N14;
}
</style>
