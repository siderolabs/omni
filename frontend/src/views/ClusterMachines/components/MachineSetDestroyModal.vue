<!--
Copyright (c) 2026 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import { ref, watchEffect } from 'vue'

import { Runtime } from '@/api/common/omni.pb'
import { ResourceService } from '@/api/grpc'
import type { ClusterSpec } from '@/api/omni/specs/omni.pb'
import { withRuntime } from '@/api/options'
import { ClusterType, DefaultNamespace, MachineSetType } from '@/api/resources'
import ManagedByTemplatesWarning from '@/components/ManagedByTemplatesWarning.vue'
import ConfirmModal from '@/components/Modals/ConfirmModal.vue'
import { useResourceWatch } from '@/methods/useResourceWatch'
import { showError, showSuccess } from '@/notification'

const { machineSetId, clusterId } = defineProps<{
  machineSetId: string
  clusterId: string
}>()

const open = defineModel<boolean>('open', { default: false })

const isDestroying = ref(false)

const { data: cluster } = useResourceWatch<ClusterSpec>(() => ({
  skip: !open.value,
  runtime: Runtime.Omni,
  resource: {
    type: ClusterType,
    namespace: DefaultNamespace,
    id: clusterId,
  },
}))

watchEffect(() => {
  if (open.value) return

  isDestroying.value = false
})

const destroyMachineSet = async () => {
  isDestroying.value = true

  try {
    await ResourceService.Teardown(
      {
        id: machineSetId,
        namespace: DefaultNamespace,
        type: MachineSetType,
      },
      withRuntime(Runtime.Omni),
    )

    showSuccess(
      `The Machine Set ${machineSetId} was Removed From the Cluster`,
      'The machine set is being torn down',
    )

    open.value = false
  } catch (e) {
    showError('Failed to Destroy The Machine Set', e instanceof Error ? e.message : String(e))
  } finally {
    isDestroying.value = false
  }
}
</script>

<template>
  <ConfirmModal
    v-model:open="open"
    title="Destroy Machine Set"
    action-label="Destroy"
    :loading="isDestroying"
    @confirm="destroyMachineSet"
  >
    <template #description>Machine Set {{ machineSetId }}</template>

    <ManagedByTemplatesWarning :resource="cluster" warning-style="popup" />

    <p class="mb-2 text-xs">Please confirm the action.</p>
  </ConfirmModal>
</template>
