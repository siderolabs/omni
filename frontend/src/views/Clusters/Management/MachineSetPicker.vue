<!--
Copyright (c) 2026 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import { RadioGroup, RadioGroupOption } from '@headlessui/vue'
import { PopoverContent, PopoverPortal, PopoverRoot, PopoverTrigger } from 'reka-ui'
import { computed, ref, useTemplateRef, watch } from 'vue'

import IconButton from '@/components/Button/IconButton.vue'
import TIcon from '@/components/Icon/TIcon.vue'
import Tooltip from '@/components/Tooltip/Tooltip.vue'
import MachineSetLabel from '@/views/Clusters/Management/MachineSetLabel.vue'

export type PickerOption = {
  id: string
  markerClass?: string
  tooltip?: string
  name?: string
  disabled?: boolean
}
const showPicker = ref(false)
const optionsView = useTemplateRef('optionsView')
const machineSetIndex = defineModel<number>()

const { options } = defineProps<{
  options: PickerOption[]
}>()

watch(
  () => options,
  () => {
    if (options.length < 8) {
      showPicker.value = false
    }
  },
)

const pickedOption = computed(() => {
  return machineSetIndex.value !== undefined ? options[machineSetIndex.value] : undefined
})

/**
 * Required to support toggling off currently selected item
 */
function toggleOption(option: PickerOption, index: number, checked: boolean) {
  if (!option.disabled) {
    machineSetIndex.value = checked ? undefined : index
  }
}
</script>

<template>
  <RadioGroup
    v-if="options.length < 8"
    v-model="machineSetIndex"
    class="flex divide-x divide-border-strong overflow-hidden rounded border border-border-strong"
  >
    <RadioGroupOption
      v-for="(option, index) in options"
      :key="index"
      v-slot="{ checked }"
      :value="index"
      :disabled="option.disabled"
    >
      <Tooltip placement="left" :description="option.tooltip">
        <MachineSetLabel
          segment
          :marker-class="option.markerClass"
          :disabled="option.disabled"
          :checked="checked"
          @click.stop="toggleOption(option, index, checked)"
        >
          {{ option.id }}
        </MachineSetLabel>
      </Tooltip>
    </RadioGroupOption>
  </RadioGroup>

  <div
    v-else
    class="relative flex h-8 items-center justify-center rounded border border-border-default bg-surface-raised"
  >
    <PopoverRoot v-model:open="showPicker">
      <PopoverPortal>
        <PopoverContent
          side="left"
          class="flex origin-(--reka-popover-content-transform-origin) flex-col items-center gap-micro rounded bg-surface-raised p-micro text-xs slide-in-from-right-2 data-[state=closed]:animate-out data-[state=closed]:fade-out-0 data-[state=closed]:zoom-out-95 data-[state=open]:animate-in data-[state=open]:fade-in-0 data-[state=open]:zoom-in-95"
        >
          <IconButton
            icon="chevron-up"
            @click="optionsView?.$el.scrollBy({ top: -24, behavior: 'smooth' })"
          />
          <RadioGroup
            ref="optionsView"
            v-model="machineSetIndex"
            class="flex h-30 scrollbar-none flex-col items-center gap-micro overflow-y-auto"
            @scroll.stop
          >
            <RadioGroupOption
              v-for="(option, index) in options"
              :key="index"
              v-slot="{ checked }"
              :value="index"
              :disabled="option.disabled"
            >
              <Tooltip :description="option.tooltip" placement="left">
                <MachineSetLabel
                  :marker-class="option.markerClass"
                  :disabled="option.disabled"
                  :checked="checked"
                  @click="showPicker = false"
                >
                  {{ option.id }}
                </MachineSetLabel>
              </Tooltip>
            </RadioGroupOption>
          </RadioGroup>
          <IconButton
            icon="chevron-down"
            @click="optionsView?.$el.scrollBy({ top: 24, behavior: 'smooth' })"
          />
        </PopoverContent>
      </PopoverPortal>

      <PopoverTrigger class="group flex h-6 items-center gap-micro px-micro">
        <TIcon
          icon="chevron-left"
          class="mx-micro h-3 w-3 text-content-muted transition-all group-hover:scale-125 group-hover:text-content-emphasis"
        />
        <template v-if="pickedOption">
          <MachineSetLabel static :marker-class="pickedOption.markerClass">
            {{ pickedOption.id }}
          </MachineSetLabel>

          <IconButton icon="close" @click.stop="machineSetIndex = undefined" />
        </template>
        <IconButton v-else icon="action-horizontal" />
      </PopoverTrigger>
    </PopoverRoot>
  </div>
</template>
