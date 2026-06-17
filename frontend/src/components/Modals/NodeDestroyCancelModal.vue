<!--
Copyright (c) 2026 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import { computed, ref, watchEffect } from 'vue'

import { Runtime } from '@/api/common/omni.pb'
import type { ClusterMachineSpec } from '@/api/omni/specs/omni.pb'
import { ClusterMachineType, DefaultNamespace } from '@/api/resources'
import ConfirmModal from '@/components/Modals/ConfirmModal.vue'
import { ClusterCommandError, restoreNode } from '@/methods/cluster'
import { useNodeName } from '@/methods/node'
import { useResourceWatch } from '@/methods/useResourceWatch'
import { showError, showSuccess } from '@/notification'

const { machineId } = defineProps<{
  machineId: string
}>()

const open = defineModel<boolean>('open', { default: false })

const nodeName = useNodeName(
  () => machineId,
  () => ({ skip: !open.value }),
)

const isRestoring = ref(false)

watchEffect(() => {
  if (open.value) return

  isRestoring.value = false
})

const {
  data: clusterMachine,
  loading: clusterMachineLoading,
  err: clusterMachineErr,
} = useResourceWatch<ClusterMachineSpec>(() => ({
  skip: !open.value,
  resource: {
    namespace: DefaultNamespace,
    type: ClusterMachineType,
    id: machineId,
  },
  runtime: Runtime.Omni,
}))

const canRestore = computed(
  () => clusterMachine.value && clusterMachine.value.metadata.phase !== 'Running',
)

async function restore() {
  if (!canRestore.value || !clusterMachine.value) return

  isRestoring.value = true

  try {
    await restoreNode(clusterMachine.value)

    showSuccess(`The Machine ${nodeName.value} was Restored`)
    open.value = false
  } catch (e) {
    if (e instanceof ClusterCommandError) {
      showError(e.errorNotification.title, e.errorNotification.details)
    } else {
      showError('Failed to Restore The Node', e instanceof Error ? e.message : String(e))
    }
  } finally {
    isRestoring.value = false
  }
}
</script>

<template>
  <ConfirmModal
    v-model:open="open"
    title="Cancel machine destroy"
    action-label="Restore Machine"
    :loading="clusterMachineLoading || isRestoring"
    :disabled="!canRestore || isRestoring || !!clusterMachineErr"
    @confirm="restore"
  >
    <template #description>Node {{ nodeName }}</template>

    <div class="flex flex-col gap-2 text-xs">
      <p v-if="clusterMachineErr" class="text-xs text-red-r1">
        {{ clusterMachineErr }}
      </p>

      <p v-else-if="canRestore">Please confirm the action.</p>
      <p v-else class="text-yellow-y1">Restoring the machine is not possible at this stage.</p>
    </div>
  </ConfirmModal>
</template>
