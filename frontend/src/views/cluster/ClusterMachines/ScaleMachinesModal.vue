<!--
Copyright (c) 2026 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import { computed, ref, watch } from 'vue'

import { Runtime } from '@/api/common/omni.pb'
import type { Resource } from '@/api/grpc'
import {
  type ClusterSpec,
  type MachineClassSpec,
  MachineSetSpecMachineAllocationType,
  type MachineSetStatusSpec,
} from '@/api/omni/specs/omni.pb'
import { ClusterType, DefaultNamespace, LabelCluster, MachineClassType } from '@/api/resources'
import TCheckbox from '@/components/Checkbox/TCheckbox.vue'
import TInput from '@/components/TInput/TInput.vue'
import { scaleMachineSet } from '@/methods/machineset'
import { useResourceGet } from '@/methods/useResourceGet'
import { useResourceWatch } from '@/methods/useResourceWatch'
import { showError } from '@/notification'
import ManagedByTemplatesWarning from '@/views/cluster/ManagedByTemplatesWarning.vue'
import Modal from '@/views/Modals/Modal.vue'

const { machineSet } = defineProps<{
  machineSet: Resource<
    MachineSetStatusSpec & Required<Pick<MachineSetStatusSpec, 'machine_allocation'>>
  >
}>()

const open = defineModel<boolean>('open', { default: false })

const machineCount = ref(0)
const scaling = ref(false)
const useAll = ref(false)

watch(
  open,
  (open, prevOpen) => {
    if (open && !prevOpen) {
      machineCount.value = machineSet.spec.machine_allocation.machine_count ?? 1
      useAll.value =
        machineSet.spec.machine_allocation.allocation_type ===
        MachineSetSpecMachineAllocationType.Unlimited
    }
  },
  { immediate: true },
)

const { data: cluster } = useResourceWatch<ClusterSpec>(() => ({
  skip: !open.value,
  resource: {
    namespace: DefaultNamespace,
    type: ClusterType,
    id: machineSet.metadata.labels?.[LabelCluster] ?? '',
  },
  runtime: Runtime.Omni,
}))

const { data: machineClass } = useResourceGet<MachineClassSpec>(() => ({
  skip: !open.value,
  resource: {
    namespace: DefaultNamespace,
    type: MachineClassType,
    id: machineSet.spec.machine_allocation.name,
  },
  runtime: Runtime.Omni,
}))

const canUseAll = computed(() =>
  machineClass.value ? machineClass.value.spec.auto_provision === undefined : false,
)

async function confirm() {
  scaling.value = true

  try {
    await scaleMachineSet(
      machineSet.metadata.id!,
      machineCount.value,
      useAll.value
        ? MachineSetSpecMachineAllocationType.Unlimited
        : MachineSetSpecMachineAllocationType.Static,
    )

    open.value = false
  } catch (e) {
    showError(`Failed to Scale Machine Set ${machineSet.metadata.id}`, `Error: ${e.message}`)
  } finally {
    scaling.value = false
  }
}
</script>

<template>
  <Modal
    v-model:open="open"
    title="Scale cluster"
    action-label="Scale"
    :loading="scaling"
    @confirm="confirm"
  >
    <ManagedByTemplatesWarning v-if="cluster" warning-style="popup" :resource="cluster" />

    <div class="flex items-center gap-4">
      <TInput
        v-model="machineCount"
        type="number"
        title="Machine count"
        compact
        :min="0"
        :disabled="useAll"
      />

      <TCheckbox v-model="useAll" label="Use all" :disabled="!canUseAll" />
    </div>
  </Modal>
</template>
