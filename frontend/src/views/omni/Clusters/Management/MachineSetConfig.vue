<!--
Copyright (c) 2025 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import pluralize from 'pluralize'
import type { Ref } from 'vue'
import { computed, ref, toRefs, watch } from 'vue'

import type { Resource } from '@/api/grpc'
import type { MachineClassSpec } from '@/api/omni/specs/omni.pb'
import { LabelWorkerRole, PatchBaseWeightMachineSet } from '@/api/resources'
import IconButton from '@/components/common/Button/IconButton.vue'
import TButton from '@/components/common/Button/TButton.vue'
import TButtonGroup from '@/components/common/Button/TButtonGroup.vue'
import TCheckbox from '@/components/common/Checkbox/TCheckbox.vue'
import TSelectList from '@/components/common/SelectList/TSelectList.vue'
import TSpinner from '@/components/common/Spinner/TSpinner.vue'
import TInput from '@/components/common/TInput/TInput.vue'
import { showModal } from '@/modal'
import type { ConfigPatch, MachineSet } from '@/states/cluster-management'
import { PatchID } from '@/states/cluster-management'
import MachineSetLabel from '@/views/omni/Clusters/Management/MachineSetLabel.vue'
import ConfigPatchEdit from '@/views/omni/Modals/ConfigPatchEdit.vue'

import MachineSetConfigEdit from '../../Modals/MachineSetConfigEdit.vue'

const emit = defineEmits(['update:modelValue'])

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
      disabled: !machineClasses?.value?.length,
      tooltip: !machineClasses?.value?.length ? 'No Machine Classes Available' : undefined,
    },
  ]

  return res
})

const props = defineProps<{
  noRemove?: boolean
  onRemove?: () => void
  machineClasses?: Resource<MachineClassSpec>[]
  modelValue: MachineSet
}>()

const { machineClasses, modelValue } = toRefs(props)

const machineClassOptions = computed(() => {
  return machineClasses?.value?.map((r: Resource) => r.metadata.id!) || []
})

const selectedMachineClass = computed(() => {
  return machineClasses?.value?.find((item) => item.metadata.id === sourceName.value)
})

const allocationMode = ref(
  modelValue.value.machineAllocation ? AllocationMode.MachineClass : AllocationMode.Manual,
)
const useMachineClasses = computed(() => allocationMode.value === AllocationMode.MachineClass)
const sourceName = ref(modelValue.value.machineAllocation?.name)
const machineCount = ref(modelValue.value.machineAllocation?.size ?? 1)
const patches: Ref<Record<string, ConfigPatch>> = ref(modelValue.value.patches)
const unlimited = ref(modelValue.value.machineAllocation?.size === 'unlimited')
const allMachines = computed(() => {
  if (selectedMachineClass?.value?.spec.auto_provision) {
    return false
  }

  return unlimited.value
})

watch(modelValue, () => {
  sourceName.value = modelValue.value.machineAllocation?.name
  machineCount.value =
    typeof modelValue.value.machineAllocation?.size === 'number'
      ? modelValue.value.machineAllocation?.size
      : 1
  patches.value = modelValue.value.patches

  if (modelValue.value.machineAllocation) {
    allocationMode.value = AllocationMode.MachineClass
  }
})

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
    ...modelValue.value,
    machineAllocation: mc,
    patches: patches.value,
  }

  emit('update:modelValue', machineSet)
})

const openMachineSetConfig = () => {
  showModal(MachineSetConfigEdit, {
    machineSet: modelValue.value,
  })
}

const openPatchConfig = () => {
  showModal(ConfigPatchEdit, {
    tabs: [
      {
        config: patches.value[PatchID.Default]?.data ?? '',
        id: `Machine Set ${modelValue.value.name}`,
      },
    ],
    onSave(config: string) {
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
    },
  })
}
</script>

<template>
  <div
    class="my-1 flex items-center gap-2 rounded border border-naturals-n5 bg-naturals-n3 px-2 py-2 pr-3 text-xs text-naturals-n13"
  >
    <MachineSetLabel :color="modelValue.color" class="w-10" :machine-set-id="modelValue.id" />
    <div class="flex flex-1 flex-wrap items-center gap-x-4 gap-y-1">
      <div class="w-32 truncate" :title="modelValue.name">
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
        :checked="unlimited"
        label="Use All Available Machines"
        class="h-6"
        @click="unlimited = !unlimited"
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
      <TButton v-if="!noRemove" class="h-6" type="compact" @click="onRemove">Remove</TButton>
      <div class="flex justify-center gap-1">
        <IconButton
          v-if="modelValue.role === LabelWorkerRole"
          icon="chart-bar"
          @click="openMachineSetConfig"
        />
        <IconButton
          :icon="patches[PatchID.Default] ? 'settings-toggle' : 'settings'"
          @click="openPatchConfig"
        />
      </div>
    </div>
  </div>
</template>
