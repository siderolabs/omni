<!--
Copyright (c) 2026 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import { ref, watchEffect } from 'vue'

import { MachineSetSpecUpdateStrategy } from '@/api/omni/specs/omni.pb'
import { LabelWorkerRole } from '@/api/resources'
import TButtonGroup from '@/components/Button/TButtonGroup.vue'
import Modal from '@/components/Modals/Modal.vue'
import TInput from '@/components/TInput/TInput.vue'
import type { MachineSet } from '@/states/cluster-management'
import { state } from '@/states/cluster-management'

const options = [
  {
    label: 'Unrestricted',
    value: MachineSetSpecUpdateStrategy.Unset,
  },
  {
    label: 'Rolling',
    value: MachineSetSpecUpdateStrategy.Rolling,
  },
]

const optionsUpgrade = [
  {
    label: 'Default',
    value: MachineSetSpecUpdateStrategy.Unset,
  },
  {
    label: 'Rolling',
    value: MachineSetSpecUpdateStrategy.Rolling,
  },
]

const { machineSet } = defineProps<{
  machineSet: MachineSet
}>()

const open = defineModel<boolean>('open', { default: false })

const updateParallelism = ref(1)
const deleteParallelism = ref(1)
const upgradeParallelism = ref(1)

const updateStrategy = ref(MachineSetSpecUpdateStrategy.Rolling)
const deleteStrategy = ref(MachineSetSpecUpdateStrategy.Unset)
const upgradeStrategy = ref(MachineSetSpecUpdateStrategy.Unset)

watchEffect(() => {
  if (!open.value) return

  updateParallelism.value = machineSet.updateStrategy?.config?.rolling?.max_parallelism ?? 1
  deleteParallelism.value = machineSet.deleteStrategy?.config?.rolling?.max_parallelism ?? 1
  upgradeParallelism.value = machineSet.upgradeStrategy?.config?.rolling?.max_parallelism ?? 1

  updateStrategy.value = machineSet.updateStrategy?.type ?? MachineSetSpecUpdateStrategy.Rolling
  deleteStrategy.value = machineSet.deleteStrategy?.type ?? MachineSetSpecUpdateStrategy.Unset
  upgradeStrategy.value = machineSet.upgradeStrategy?.type ?? MachineSetSpecUpdateStrategy.Unset
})

const saveAndClose = async () => {
  const ms = state.value.machineSets.find((item) => item.id === machineSet.id)
  if (!ms) {
    return
  }

  if (ms?.role === LabelWorkerRole) {
    ms.updateStrategy = {
      type: updateStrategy.value,
      config: {
        rolling: {
          max_parallelism: updateParallelism.value,
        },
      },
    }

    ms.deleteStrategy = {
      type: deleteStrategy.value,
      config: {
        rolling: {
          max_parallelism: deleteParallelism.value,
        },
      },
    }
  }

  ms.upgradeStrategy = {
    type: upgradeStrategy.value,
  }

  if (upgradeStrategy.value === MachineSetSpecUpdateStrategy.Rolling) {
    ms.upgradeStrategy.config = {
      rolling: {
        max_parallelism: upgradeParallelism.value,
      },
    }
  }

  open.value = false
}
</script>

<template>
  <Modal
    v-model:open="open"
    title="Machine Set Scaling Configuration"
    action-label="Save"
    disable-content-padding
    @confirm="saveAndClose"
  >
    <div class="flex flex-1 flex-col">
      <template v-if="machineSet.role === LabelWorkerRole">
        <div
          class="flex flex-wrap items-center gap-2 border-b border-naturals-n4 px-8 py-2 text-sm"
        >
          <div class="w-32">Update Strategy</div>
          <TButtonGroup v-model="updateStrategy" :options="options" class="flex-1" />
          <template v-if="updateStrategy !== MachineSetSpecUpdateStrategy.Unset">
            <div>Max Parallelism</div>
            <div>
              <TInput v-model="updateParallelism" type="number" class="h-7 w-12" />
            </div>
          </template>
          <div v-else class="flex h-7 items-center">Update All Simultaneously</div>
        </div>
        <div
          class="flex flex-wrap items-center gap-2 border-b border-naturals-n4 px-8 py-2 text-sm"
        >
          <div class="w-32">Delete Strategy</div>
          <TButtonGroup v-model="deleteStrategy" :options="options" class="flex-1" />
          <template v-if="deleteStrategy !== MachineSetSpecUpdateStrategy.Unset">
            <div>Max Parallelism</div>
            <div>
              <TInput v-model="deleteParallelism" type="number" class="h-7 w-12" />
            </div>
          </template>
          <div v-else class="flex h-7 items-center">
            <span>Delete All Simultaneously</span>
          </div>
        </div>
      </template>
      <div class="flex flex-wrap items-center gap-2 border-b border-naturals-n4 px-8 py-2 text-sm">
        <div class="w-32">Upgrade Strategy</div>
        <TButtonGroup v-model="upgradeStrategy" :options="optionsUpgrade" class="flex-1" />
        <template v-if="upgradeStrategy !== MachineSetSpecUpdateStrategy.Unset">
          <div>Max Parallelism</div>
          <div>
            <TInput v-model="upgradeParallelism" type="number" class="h-7 w-12" />
          </div>
        </template>
        <div v-else class="flex h-7 items-center">
          <span>Upgrade One at a Time</span>
        </div>
      </div>
    </div>
  </Modal>
</template>
