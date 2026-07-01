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

const emit = defineEmits<{
  'update:modelValue': [MachineSet]
}>()

enum AllocationMode {
  Manual = 'Manual',
  MachineClass = 'Machine Class',
  RequestSet = 'Machine Request Set',
}

const allocationModes = computed(() => {
  const res = [
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
  ]

  return res
})

const { machineClasses, modelValue } = defineProps<{
  talosVersion?: string
  noRemove?: boolean
  onRemove?: () => void
  machineClasses?: Resource<MachineClassSpec>[]
  modelValue: MachineSet
}>()

const machineClassOptions = computed(() => {
  return machineClasses?.map((r: Resource) => r.metadata.id!) || []
})

const selectedMachineClass = computed(() => {
  return machineClasses?.find((item) => item.metadata.id === sourceName.value)
})

const configPatchEditModalOpen = ref(false)
const machineSetConfigEditModalOpen = ref(false)
const allocationMode = ref(
  modelValue.machineAllocation ? AllocationMode.MachineClass : AllocationMode.Manual,
)
const useMachineClasses = computed(() => allocationMode.value === AllocationMode.MachineClass)
const sourceName = ref(modelValue.machineAllocation?.name)
const machineCount = ref(modelValue.machineAllocation?.size ?? 1)
const patches = ref(modelValue.patches)
const unlimited = ref(modelValue.machineAllocation?.size === 'unlimited')
const allMachines = computed(() => {
  if (selectedMachineClass?.value?.spec.auto_provision) {
    return false
  }

  return unlimited.value
})

watch(
  () => modelValue,
  () => {
    sourceName.value = modelValue.machineAllocation?.name
    machineCount.value =
      typeof modelValue.machineAllocation?.size === 'number'
        ? modelValue.machineAllocation?.size
        : 1
    patches.value = modelValue.patches

    if (modelValue.machineAllocation) {
      allocationMode.value = AllocationMode.MachineClass
    }
  },
)

watch([sourceName, machineCount, useMachineClasses, patches, allMachines], () => {
  if (useMachineClasses.value && !sourceName.value && machineClassOptions.value.length > 0) {
    sourceName.value = machineClassOptions.value[0]
  }

  const mc =
    useMachineClasses.value && sourceName.value !== undefined
      ? {
          name: sourceName.value,
          size: allMachines.value ? 'unlimited' : machineCount.value,
        }
      : undefined

  const machineSet: MachineSet = {
    ...modelValue,
    machineAllocation: mc,
    patches: patches.value,
  }

  emit('update:modelValue', machineSet)
})

const onSavePatchConfig = (config: string) => {
  if (!config) {
    delete patches.value[PatchID.Default]

    return
  }

  patches.value = {
    [PatchID.Default]: {
      data: config,
      weight: PatchBaseWeightMachineSet,
    },
    ...patches.value,
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
      <span class="resource-label" :class="modelValue.labelClass">{{ modelValue.id }}</span>
    </div>

    <div class="flex flex-1 flex-wrap items-center gap-x-4 gap-y-1">
      <div :id="labelId" class="w-32 truncate" :title="modelValue.name">
        {{ modelValue.name }}
      </div>
      <div class="flex items-center gap-2">
        Allocation Mode:
        <TButtonGroup v-model="allocationMode" :options="allocationModes" />
      </div>
      <template v-if="useMachineClasses">
        <TSelectList
          v-if="machineClasses"
          class="h-6 w-48"
          title="Name"
          :default-value="sourceName ?? machineClassOptions[0]"
          :values="machineClassOptions"
          @checked-value="
            (value: string) => {
              sourceName = value
            }
          "
        />
        <TSpinner v-else class="h-4 w-4" />
      </template>
      <TCheckbox
        v-if="useMachineClasses && !selectedMachineClass?.spec.auto_provision"
        v-model="unlimited"
        label="Use All Available Machines"
        class="h-6"
      />
      <div v-if="!allMachines" class="w-32">
        <TInput
          v-if="useMachineClasses"
          v-model="machineCount"
          class="h-6"
          title="Size"
          type="number"
          :min="0"
          compact
        />
        <div v-else>{{ pluralize('Machines', Object.keys(modelValue.machines).length, true) }}</div>
      </div>
    </div>
    <div class="flex w-24 items-center justify-end gap-2">
      <TButton v-if="!noRemove" class="h-6" size="sm" @click="onRemove">Remove</TButton>
      <div class="flex justify-center gap-1">
        <IconButton icon="chart-bar" @click="machineSetConfigEditModalOpen = true" />
        <IconButton
          :icon="patches[PatchID.Default] ? 'settings-toggle' : 'settings'"
          @click="configPatchEditModalOpen = true"
        />
      </div>
    </div>

    <ConfigPatchEditModal
      :id="`Machine Set ${modelValue.name}`"
      v-model:open="configPatchEditModalOpen"
      :config="patches[PatchID.Default]?.data ?? ''"
      :talos-version="talosVersion"
      @save="onSavePatchConfig"
    />

    <MachineSetConfigEditModal
      v-model:open="machineSetConfigEditModalOpen"
      :machine-set="modelValue"
    />
  </li>
</template>
