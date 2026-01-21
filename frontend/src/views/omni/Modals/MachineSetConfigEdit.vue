<!--
Copyright (c) 2026 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import { ref, toRefs } from 'vue'

import { MachineSetSpecUpdateStrategy } from '@/api/omni/specs/omni.pb'
import TButton from '@/components/common/Button/TButton.vue'
import TButtonGroup from '@/components/common/Button/TButtonGroup.vue'
import TInput from '@/components/common/TInput/TInput.vue'
import { closeModal } from '@/modal'
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

interface Props {
  machineSet: MachineSet
}

const props = defineProps<Props>()

const { machineSet } = toRefs(props)

const updateParallelism = ref(
  machineSet.value.updateStrategy?.config?.rolling?.max_parallelism ?? 1,
)
const deleteParallelism = ref(
  machineSet.value.deleteStrategy?.config?.rolling?.max_parallelism ?? 1,
)

const updateStrategy = ref<MachineSetSpecUpdateStrategy>(
  machineSet.value.updateStrategy?.type ?? MachineSetSpecUpdateStrategy.Rolling,
)
const deleteStrategy = ref<MachineSetSpecUpdateStrategy>(
  machineSet.value.deleteStrategy?.type ?? MachineSetSpecUpdateStrategy.Unset,
)

const close = () => {
  closeModal()
}

const saveAndClose = async () => {
  const ms = state.value.machineSets.find((item) => item.id === machineSet.value.id)
  if (ms) {
    if (updateStrategy.value !== undefined) {
      ms.updateStrategy = {
        type: updateStrategy.value,
        config: {
          rolling: {
            max_parallelism: updateParallelism.value,
          },
        },
      }
    }

    if (deleteStrategy.value !== undefined) {
      ms.deleteStrategy = {
        type: deleteStrategy.value,
        config: {
          rolling: {
            max_parallelism: deleteParallelism.value,
          },
        },
      }
    }
  }

  close()
}
</script>

<template>
  <div class="modal-window">
    <div class="my-7 flex items-center px-8">
      <div class="heading">Machine Set Scaling Configuration</div>
    </div>
    <div class="flex flex-1 flex-col">
      <div class="flex flex-wrap items-center gap-2 border-b border-naturals-n4 px-8 py-2 text-sm">
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
      <div class="flex flex-wrap items-center gap-2 border-b border-naturals-n4 px-8 py-2 text-sm">
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
    </div>
    <div class="flex justify-between gap-4 rounded-b bg-naturals-n3 p-4">
      <TButton type="secondary" @click="close">Cancel</TButton>
      <TButton @click="saveAndClose">Save</TButton>
    </div>
  </div>
</template>

<style scoped>
@reference "../../../index.css";

.modal-window {
  @apply p-0;
}
.heading {
  @apply text-xl text-naturals-n14;
}
</style>
