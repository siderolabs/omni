<!--
Copyright (c) 2026 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import pluralize from 'pluralize'
import { computed, ref, useId, watch } from 'vue'

import type { Resource } from '@/api/grpc'
import type { MachineClassSpec } from '@/api/omni/specs/omni.pb'
import { PatchBaseWeightMachineSet } from '@/api/resources'
import IconButton from '@/components/Button/IconButton.vue'
import TButton from '@/components/Button/TButton.vue'
import TButtonGroup from '@/components/Button/TButtonGroup.vue'
import TCheckbox from '@/components/Checkbox/TCheckbox.vue'
import ConfigPatchEditModal from '@/components/Modals/ConfigPatchEditModal.vue'
import TSelectList from '@/components/SelectList/TSelectList.vue'
import TSpinner from '@/components/Spinner/TSpinner.vue'
import TInput from '@/components/TInput/TInput.vue'
import type { MachineSet } from '@/states/cluster-management'
import { PatchID } from '@/states/cluster-management'
import MachineSetConfigEditModal from '@/views/Clusters/components/MachineSetConfigEditModal.vue'

enum AllocationMode {
  Manual = 'Manual',
  MachineClass = 'Machine Class',
  RequestSet = 'Machine Request Set',
}

const { machineClasses } = defineProps<{
  talosVersion?: string
  noRemove?: boolean
  machineClasses?: Resource<MachineClassSpec>[]
}>()

defineEmits<{
  onRemove: []
}>()

const machineSet = defineModel<MachineSet>({ required: true })

const allocationModes = computed(() => [
  {
    label: AllocationMode.Manual,
    value: AllocationMode.Manual,
  },
  {
    label: AllocationMode.MachineClass,
    value: AllocationMode.MachineClass,
    disabled: !machineClasses?.length,
    tooltip: !machineClasses?.length ? 'No Machine Classes Available' : undefined,
  },
])

const configPatchEditModalOpen = ref(false)
const machineSetConfigEditModalOpen = ref(false)

const machineClassOptions = computed(() => machineClasses?.map((r) => r.metadata.id!) || [])
const selectedMachineClass = computed(() => {
  const className = machineSet.value.machineAllocation?.name

  return machineClasses && className
    ? machineClasses.find((r) => r.metadata.id === className)
    : undefined
})

// Normalise sizing incase the selected class does not support unlimited
watch(selectedMachineClass, (selectedMachineClass) => {
  if (
    selectedMachineClass?.spec.auto_provision &&
    machineSet.value.machineAllocation?.size === 'unlimited'
  ) {
    machineSet.value.machineAllocation.size = 1
  }
})

const onSavePatchConfig = (config: string) => {
  if (!config) {
    delete machineSet.value.patches[PatchID.Default]

    return
  }

  machineSet.value.patches[PatchID.Default] = {
    data: config,
    weight: PatchBaseWeightMachineSet,
  }
}

const labelId = useId()
</script>

<template>
  <li
    class="my-1 flex items-center gap-2 rounded border border-naturals-n5 bg-naturals-n3 px-2 py-2 pr-3 text-xs text-naturals-n13"
    :aria-labelledby="labelId"
  >
    <div class="w-10">
      <span class="resource-label" :class="machineSet.labelClass">{{ machineSet.id }}</span>
    </div>

    <div class="flex flex-1 flex-wrap items-center gap-x-4 gap-y-1">
      <div :id="labelId" class="w-32 truncate" :title="machineSet.name">
        {{ machineSet.name }}
      </div>
      <div class="flex items-center gap-2">
        Allocation Mode:
        <TButtonGroup
          :model-value="
            machineSet.machineAllocation ? AllocationMode.MachineClass : AllocationMode.Manual
          "
          :options="allocationModes"
          @update:model-value="
            machineSet.machineAllocation =
              $event === AllocationMode.MachineClass
                ? { name: machineClassOptions[0], size: 1 }
                : undefined
          "
        />
      </div>
      <template v-if="machineSet.machineAllocation">
        <TSelectList
          v-if="machineClasses"
          v-model="machineSet.machineAllocation.name"
          class="h-6 w-48"
          title="Name"
          :default-value="machineClassOptions[0]"
          :values="machineClassOptions"
        />
        <TSpinner v-else class="h-4 w-4" />
      </template>
      <TCheckbox
        v-if="machineSet.machineAllocation && !selectedMachineClass?.spec.auto_provision"
        :model-value="machineSet.machineAllocation.size === 'unlimited'"
        label="Use All Available Machines"
        class="h-6"
        @update:model-value="machineSet.machineAllocation.size = $event ? 'unlimited' : 1"
      />
      <div v-if="machineSet.machineAllocation?.size !== 'unlimited'" class="w-32">
        <TInput
          v-if="machineSet.machineAllocation"
          v-model="machineSet.machineAllocation.size"
          class="h-6"
          title="Size"
          type="number"
          :min="0"
          compact
        />
        <div v-else>{{ pluralize('Machines', Object.keys(machineSet.machines).length, true) }}</div>
      </div>
    </div>
    <div class="flex w-24 items-center justify-end gap-2">
      <TButton v-if="!noRemove" class="h-6" size="sm" @click="$emit('onRemove')">Remove</TButton>
      <div class="flex justify-center gap-1">
        <IconButton icon="chart-bar" @click="machineSetConfigEditModalOpen = true" />
        <IconButton
          :icon="machineSet.patches[PatchID.Default] ? 'settings-toggle' : 'settings'"
          @click="configPatchEditModalOpen = true"
        />
      </div>
    </div>

    <ConfigPatchEditModal
      :id="`Machine Set ${machineSet.name}`"
      v-model:open="configPatchEditModalOpen"
      :config="machineSet.patches[PatchID.Default]?.data ?? ''"
      :talos-version="talosVersion"
      @save="onSavePatchConfig"
    />

    <MachineSetConfigEditModal
      v-model:open="machineSetConfigEditModalOpen"
      :machine-set="machineSet"
    />
  </li>
</template>
