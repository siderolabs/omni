<!--
Copyright (c) 2025 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import { useRoute, useRouter } from 'vue-router'

import TButton from '@/components/common/Button/TButton.vue'
import { acceptMachine } from '@/methods/machine'
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
    await acceptMachine(route.query.machine as string)
  } catch (e) {
    showError(`Failed to Accept the Machine ${route.query.machine}`, e.message)
  }

  close()

  showSuccess(`The Machine ${route.query.machine} was Accepted`)
}
</script>

<template>
  <div class="modal-window">
    <div class="heading">
      <h3 class="text-base text-naturals-n14">Accept the Machine {{ $route.query.machine }} ?</h3>
      <CloseButton @click="close" />
    </div>

    <p class="py-2 text-xs">Please confirm the action.</p>

    <div class="text-xs">
      <p class="py-2 font-bold text-primary-p3">
        Accepting the machine will wipe ALL of its disks.
      </p>
    </div>

    <div class="mt-8 flex justify-end gap-4">
      <TButton class="h-9 w-32" icon="check" icon-position="left" @click="reject"> Accept </TButton>
    </div>
  </div>
</template>

<style scoped>
@reference "../../../index.css";

.heading {
  @apply mb-5 flex items-center justify-between text-xl text-naturals-n14;
}
</style>
