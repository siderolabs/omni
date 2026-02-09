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
            r.spec.acceptance_status = InfraMachineConfigSpecAcceptanceStatus.PENDING
          })

          showSuccess(`The Machine ${machine} was Unrejected`)
        } catch (e) {
          showError(
            `Failed to Unreject the Machine ${machine}`,
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
    :title="`Unreject ${pluralize('Machine', machines.length, true)}`"
    action-label="Unreject"
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
      Unrejecting machines will place them back in the pending machines list. You can accept or
      reject them again later.
    </p>
  </ConfirmModal>
</template>
