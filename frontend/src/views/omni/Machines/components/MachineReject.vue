<!--
Copyright (c) 2026 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import pluralize from 'pluralize'
import { ref } from 'vue'

import { InfraMachineConfigSpecAcceptanceStatus } from '@/api/omni/specs/omni.pb'
import { updateInfraMachineConfig } from '@/methods/machine'
import { showError, showSuccess } from '@/notification'
import ConfirmModal from '@/views/omni/Modals/ConfirmModal.vue'

const { machines } = defineProps<{ machines: string[] }>()
const open = defineModel<boolean>('open', { default: false })
const emit = defineEmits<{ confirmed: [] }>()

const loading = ref(false)

const confirm = async () => {
  try {
    loading.value = true

    await Promise.all(
      machines.map(async (machine) => {
        try {
          await updateInfraMachineConfig(machine, (r) => {
            r.spec.acceptance_status = InfraMachineConfigSpecAcceptanceStatus.REJECTED
          })

          showSuccess(`The Machine ${machine} was Rejected`)
        } catch (e) {
          showError(
            `Failed to Reject the Machine ${machine}`,
            e instanceof Error ? e.message : String(e),
          )
        }
      }),
    )
  } finally {
    loading.value = false
    emit('confirmed')
  }

  open.value = false
}
</script>

<template>
  <ConfirmModal
    v-model:open="open"
    :title="`Reject ${pluralize('Machine', machines.length, true)}`"
    action-label="Reject"
    :loading="loading"
    @confirm="confirm"
  >
    <ul class="list-inside list-disc">
      <li v-for="machine in machines" :key="machine">
        <code>{{ machine }}</code>
      </li>
    </ul>

    <p class="py-2 text-xs">Please confirm the action.</p>
    <p class="py-2 text-xs text-primary-p2">
      Rejected machines will be removed from the pending machines list. You can use the rejected
      machines list or omnictl to accept them again.
    </p>
  </ConfirmModal>
</template>
