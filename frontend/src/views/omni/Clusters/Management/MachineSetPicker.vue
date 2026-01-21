<!--
Copyright (c) 2026 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import { RadioGroup, RadioGroupOption } from '@headlessui/vue'
import { PopoverContent, PopoverPortal, PopoverRoot, PopoverTrigger } from 'reka-ui'
import { computed, ref, toRefs, watch } from 'vue'

import IconButton from '@/components/common/Button/IconButton.vue'
import TIcon from '@/components/common/Icon/TIcon.vue'
import Tooltip from '@/components/common/Tooltip/Tooltip.vue'

import MachineSetLabel from './MachineSetLabel.vue'

export type PickerOption = {
  id: string
  color: string
  tooltip?: string
  name?: string
  disabled?: boolean
}

const emit = defineEmits(['update:machineSetIndex'])
const showPicker = ref(false)
const optionsView = ref()

const props = defineProps<{
  machineSetIndex?: number
  options: PickerOption[]
}>()

const { options, machineSetIndex } = toRefs(props)

watch(options, () => {
  if (options.value.length < 8) {
    showPicker.value = false
  }
})

const pickedOption = computed(() => {
  return machineSetIndex?.value !== undefined ? options.value[machineSetIndex.value] : undefined
})

const onSelect = (index: number) => {
  if (machineSetIndex.value === index) {
    emit('update:machineSetIndex', undefined)
  } else {
    emit('update:machineSetIndex', index)
  }

  showPicker.value = false
}
</script>

<template>
  <div>
    <RadioGroup
      v-if="options.length < 8"
      :model-value="machineSetIndex"
      class="flex gap-0.5 rounded bg-naturals-n3 p-1"
      @update:model-value="(value) => emit('update:machineSetIndex', value)"
    >
      <RadioGroupOption
        v-for="(option, index) in options"
        :key="index"
        :value="index"
        as="template"
        :disabled="option.disabled"
      >
        <div @click="() => onSelect(index)">
          <Tooltip placement="left" :description="option.tooltip">
            <div class="relative">
              <MachineSetLabel
                :id="option.id"
                class="machine-set-label opacity-75 transition-opacity hover:opacity-100"
                :color="option.color"
                :class="{
                  'opacity-100': machineSetIndex === index && !option.disabled,
                  disabled: option?.disabled,
                }"
                :machine-set-id="option.id"
                :disabled="option?.disabled"
              />
              <div
                v-if="machineSetIndex === index"
                class="pointer-events-none absolute top-0 right-0 bottom-0 left-0 rounded border border-white bg-naturals-n12 mix-blend-overlay"
              />
            </div>
          </Tooltip>
        </div>
      </RadioGroupOption>
    </RadioGroup>

    <div v-else class="relative flex h-8 items-center justify-center rounded bg-naturals-n3">
      <PopoverRoot v-model:open="showPicker">
        <PopoverPortal>
          <PopoverContent
            side="left"
            class="flex origin-(--reka-popover-content-transform-origin) flex-col items-center gap-1 rounded bg-naturals-n3 p-1 text-xs slide-in-from-right-2 data-[state=closed]:animate-out data-[state=closed]:fade-out-0 data-[state=closed]:zoom-out-95 data-[state=open]:animate-in data-[state=open]:fade-in-0 data-[state=open]:zoom-in-95"
          >
            <IconButton
              icon="arrow-up"
              @click="optionsView.$el.scrollBy({ top: -24, behavior: 'smooth' })"
            />
            <RadioGroup
              ref="optionsView"
              :model-value="machineSetIndex"
              class="no-scrollbar scroll flex h-[7.5rem] flex-col items-center gap-0.5 overflow-y-auto"
              @update:model-value="(value) => emit('update:machineSetIndex', value)"
              @scroll.stop
            >
              <RadioGroupOption
                v-for="(option, index) in options"
                :key="index"
                v-slot="{ checked }"
                :value="index"
                as="template"
                :disabled="option.disabled"
              >
                <div @click="() => onSelect(checked)">
                  <Tooltip :description="option.tooltip" placement="left">
                    <div class="relative">
                      <MachineSetLabel
                        class="machine-set-label opacity-75 transition-opacity hover:opacity-100"
                        :color="option.color"
                        :class="{
                          'opacity-100': checked && !option.disabled,
                          disabled: option?.disabled,
                        }"
                        :machine-set-id="option.id"
                        :disabled="option?.disabled"
                      />
                      <div
                        v-if="checked"
                        class="pointer-events-none absolute top-0 right-0 bottom-0 left-0 rounded border border-white bg-naturals-n12 mix-blend-overlay"
                      />
                    </div>
                  </Tooltip>
                </div>
              </RadioGroupOption>
            </RadioGroup>
            <IconButton
              icon="arrow-down"
              @click="optionsView.$el.scrollBy({ top: 24, behavior: 'smooth' })"
            />
          </PopoverContent>
        </PopoverPortal>

        <PopoverTrigger class="group flex h-6 items-center gap-1 px-1">
          <TIcon
            icon="arrow-left"
            class="mx-1 h-3 w-3 text-naturals-n7 transition-all group-hover:scale-125 group-hover:text-naturals-n14"
          />
          <template v-if="pickedOption">
            <MachineSetLabel :machine-set-id="pickedOption?.id" :color="pickedOption?.color" />
            <IconButton
              icon="close"
              @click.stop="() => emit('update:machineSetIndex', undefined)"
            />
          </template>
          <IconButton v-else icon="action-horizontal" />
        </PopoverTrigger>
      </PopoverRoot>
    </div>
  </div>
</template>

<style scoped>
.no-scrollbar::-webkit-scrollbar {
  display: none;
}

/* Hide scrollbar for IE, Edge and Firefox */
.no-scrollbar {
  -ms-overflow-style: none; /* IE and Edge */
  scrollbar-width: none; /* Firefox */
}
</style>
