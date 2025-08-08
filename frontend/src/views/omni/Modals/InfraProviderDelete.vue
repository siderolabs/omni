<!--
Copyright (c) 2025 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<template>
  <div class="modal-window">
    <div class="heading">
      <h3 class="text-base text-naturals-N14">Delete Infra Provider {{ $route.query.provider }}</h3>
      <close-button @click="close" />
    </div>
    <p class="text-xs">Please confirm the action.</p>
    <div class="text-yellow-Y1 text-xs my-3">
      The infra provider service will no longer be able to connect to Omni. And it's service account
      key will be removed.
    </div>
    <div class="flex justify-end">
      <t-button @click="deleteProvider" class="w-32 h-9"> Delete </t-button>
    </div>
  </div>
</template>

<script setup lang="ts">
import { showError, showSuccess } from '@/notification'
import { useRoute, useRouter } from 'vue-router'

import CloseButton from '@/views/omni/Modals/CloseButton.vue'
import TButton from '@/components/common/Button/TButton.vue'
import { ResourceService } from '@/api/grpc'
import { InfraProviderNamespace, ProviderType } from '@/api/resources'
import { withRuntime } from '@/api/options'
import { Runtime } from '@/api/common/omni.pb'

const router = useRouter()
const route = useRoute()

const deleteProvider = async () => {
  try {
    await ResourceService.Delete(
      {
        namespace: InfraProviderNamespace,
        id: route.query.provider as string,
        type: ProviderType,
      },
      withRuntime(Runtime.Omni),
    )
  } catch (e) {
    showError('Failed to Delete Infra Provider', e.message)

    return
  } finally {
    close()
  }

  showSuccess(`Infra Provider ${route.query.provider} Deleted`, undefined)
}

let closed = false

const close = () => {
  if (closed) {
    return
  }

  closed = true

  router.go(-1)
}
</script>

<style scoped>
.window {
  @apply rounded bg-naturals-N2 z-30 w-1/3 flex flex-col p-8;
}

.heading {
  @apply flex justify-between items-center mb-5 text-xl text-naturals-N14;
}

code {
  @apply break-all rounded bg-naturals-N4;
}
</style>
