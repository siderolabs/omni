<!--
Copyright (c) 2026 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import pluralize from 'pluralize'
import { computed } from 'vue'
import { useRoute, useRouter } from 'vue-router'

import TButton from '@/components/common/Button/TButton.vue'
import { rejectMachine } from '@/methods/machine'
import { showError, showSuccess } from '@/notification'
import CloseButton from '@/views/omni/Modals/CloseButton.vue'

const router = useRouter()
const route = useRoute()

const machines = computed(() => {
  const { machine } = route.query

  const arr = Array.isArray(machine) ? machine : [machine]
  return arr.filter((m) => typeof m === 'string')
})

let closed = false

const close = () => {
  if (closed) {
    return
  }

  closed = true

  router.go(-1)
}

const reject = async () => {
  await Promise.all(
    machines.value.map(async (machine) => {
      try {
        await rejectMachine(machine)
        showSuccess(`The Machine ${machine} was Rejected`)
      } catch (e) {
        showError(`Failed to Reject the Machine ${machine}`, e)
      }
    }),
  )

  close()
}
</script>

<template>
  <div class="modal-window">
    <div class="mb-5 flex items-center justify-between text-xl text-naturals-n14">
      <h3 class="text-base text-naturals-n14">
        Reject {{ pluralize('Machine', machines.length, true) }}
      </h3>
      <CloseButton @click="close" />
    </div>

    <ul class="list-inside list-disc">
      <li v-for="machine in machines" :key="machine">
        <code>{{ machine }}</code>
      </li>
    </ul>

    <p class="py-2 text-xs">Please confirm the action.</p>
    <p class="py-2 text-xs text-primary-p2">
      Rejected machine will not appear in the UI anymore. You can use omnictl to accept it again.
    </p>

    <div class="mt-8 flex justify-end gap-4">
      <TButton class="h-9 w-32" icon="close" icon-position="left" @click="reject">Reject</TButton>
    </div>
  </div>
</template>
