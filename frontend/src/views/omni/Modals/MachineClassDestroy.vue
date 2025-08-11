<!--
Copyright (c) 2025 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import { useRoute, useRouter } from 'vue-router'

import { Runtime } from '@/api/common/omni.pb'
import { ResourceService } from '@/api/grpc'
import { withRuntime } from '@/api/options'
import { DefaultNamespace, MachineClassType } from '@/api/resources'
import TButton from '@/components/common/Button/TButton.vue'
import { showError, showSuccess } from '@/notification'
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

const destroy = async () => {
  try {
    await ResourceService.Delete(
      {
        id: route.query.classname as string,
        namespace: DefaultNamespace,
        type: MachineClassType,
      },
      withRuntime(Runtime.Omni),
    )
  } catch (e) {
    showError('Failed to remove the machine class', e.message)

    close()

    return
  }

  close()

  showSuccess(`The Machine Class ${route.query.classname} was Destroyed`)
}
</script>

<template>
  <div class="modal-window">
    <div class="heading">
      <h3 class="text-base text-naturals-n14">
        Destroy the Machine Class {{ $route.query.classname }} ?
      </h3>
      <CloseButton @click="close" />
    </div>

    <p class="py-2 text-xs">Please confirm the action.</p>

    <div class="mt-8 flex justify-end gap-4">
      <TButton class="h-9 w-32" icon="delete" icon-position="left" @click="destroy">
        Destroy
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
