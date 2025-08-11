<!--
Copyright (c) 2025 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import { ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'

import TButton from '@/components/common/Button/TButton.vue'
import { removeMachine } from '@/methods/machine'
import { showError, showSuccess } from '@/notification'
import CloseButton from '@/views/omni/Modals/CloseButton.vue'

const router = useRouter()
const route = useRoute()
const disabled = ref(false)

let closed = false

const close = () => {
  if (closed) {
    return
  }

  closed = true
  disabled.value = false

  router.go(-1)
}

const remove = async () => {
  disabled.value = true

  try {
    await removeMachine(route.query.machine as string)
  } catch (e) {
    close()

    showError('Failed to remove the machine', e.message)

    return
  }

  close()

  showSuccess(`The Machine ${route.query.machine} was Removed`)
}
</script>

<template>
  <div class="modal-window">
    <div class="heading">
      <h3 class="text-base text-naturals-n14">Destroy the Machine {{ $route.query.machine }} ?</h3>
      <CloseButton @click="close" />
    </div>

    <p class="py-2 text-xs">Please confirm the action.</p>

    <div v-if="$route.query.cluster" class="text-xs">
      <p class="py-2 text-primary-p3">
        The machine <code>{{ $route.query.machine }}</code> is part of the cluster
        <code>{{ $route.query.cluster }}</code
        >. Destroying the machine should be only used as a last resort, e.g. in a case of a hardware
        failure.
      </p>
      <p class="py-2 font-bold text-primary-p3">
        The machine will need to wiped and reinstalled to be used again with Omni.
      </p>

      <p class="py-2">
        If you want to remove the machine from the cluster, please use the
        <RouterLink
          :to="{ name: 'ClusterOverview', params: { cluster: $route.query.cluster as string } }"
          >cluster overview page</RouterLink
        >.
      </p>
    </div>

    <div class="mt-8 flex justify-end gap-4">
      <TButton
        class="h-9 w-32"
        icon="delete"
        icon-position="left"
        :disabled="disabled"
        @click="remove"
      >
        Remove
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
