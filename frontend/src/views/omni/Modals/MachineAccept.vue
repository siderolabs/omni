<!--
Copyright (c) 2025 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import pluralize from 'pluralize'
import { computed } from 'vue'
import { useRoute, useRouter } from 'vue-router'

import TButton from '@/components/common/Button/TButton.vue'
import { acceptMachine } from '@/methods/machine'
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

const accept = async () => {
  await Promise.all(
    machines.value.map(async (machine) => {
      try {
        await acceptMachine(machine)
        showSuccess(`The Machine ${machine} was Accepted`)
      } catch (e) {
        showError(`Failed to Accept the Machine ${machine}`, e)
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
        Accept {{ pluralize('Machine', machines.length, true) }}
      </h3>
      <CloseButton @click="close" />
    </div>

    <ul class="list-inside list-disc">
      <li v-for="machine in machines" :key="machine">
        <code>{{ machine }}</code>
      </li>
    </ul>

    <p class="py-2 text-xs">Please confirm the action.</p>

    <div class="text-xs">
      <p class="py-2 font-bold text-primary-p3">
        Accepting the machine will wipe ALL of its disks.
      </p>
    </div>

    <div class="mt-8 flex justify-end gap-4">
      <TButton class="h-9 w-32" icon="check" icon-position="left" @click="accept">Accept</TButton>
    </div>
  </div>
</template>
