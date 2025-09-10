<!--
Copyright (c) 2025 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import { useRoute, useRouter } from 'vue-router'

import TButton from '@/components/common/Button/TButton.vue'
import { rejectMachine } from '@/methods/machine'
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

const reject = async () => {
  try {
    await rejectMachine(route.query.machine as string)
  } catch (e) {
    showError(`Failed to Reject the Machine ${route.query.machine}`, e.message)
  }

  close()

  showSuccess(`The Machine ${route.query.machine} was Rejected`)
}
</script>

<template>
  <div class="modal-window">
    <div class="heading">
      <h3 class="text-base text-naturals-n14">Reject the Machine {{ $route.query.machine }} ?</h3>
      <CloseButton @click="close" />
    </div>

    <p class="py-2 text-xs">Please confirm the action.</p>
    <p class="py-2 text-xs text-primary-p2">
      Rejected machine will not appear in the UI anymore. You can use omnictl to accept it again.
    </p>

    <div class="mt-8 flex justify-end gap-4">
      <TButton class="h-9 w-32" icon="close" icon-position="left" @click="reject">Reject</TButton>
    </div>
  </div>
</template>

<style scoped>
@reference "../../../index.css";

.heading {
  @apply mb-5 flex items-center justify-between text-xl text-naturals-n14;
}
</style>
