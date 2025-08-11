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
import { InfraProviderNamespace, ProviderType } from '@/api/resources'
import TButton from '@/components/common/Button/TButton.vue'
import { showError, showSuccess } from '@/notification'
import CloseButton from '@/views/omni/Modals/CloseButton.vue'

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

<template>
  <div class="modal-window">
    <div class="heading">
      <h3 class="text-base text-naturals-n14">Delete Infra Provider {{ $route.query.provider }}</h3>
      <CloseButton @click="close" />
    </div>
    <p class="text-xs">Please confirm the action.</p>
    <div class="my-3 text-xs text-yellow-y1">
      The infra provider service will no longer be able to connect to Omni. And it's service account
      key will be removed.
    </div>
    <div class="flex justify-end">
      <TButton class="h-9 w-32" @click="deleteProvider"> Delete </TButton>
    </div>
  </div>
</template>

<style scoped>
@reference "../../../index.css";

.window {
  @apply z-30 flex w-1/3 flex-col rounded bg-naturals-n2 p-8;
}

.heading {
  @apply mb-5 flex items-center justify-between text-xl text-naturals-n14;
}

code {
  @apply rounded bg-naturals-n4 break-all;
}
</style>
