<!--
Copyright (c) 2025 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import { RadioGroup, RadioGroupOption } from '@headlessui/vue'
import { computed, ref, toRefs, watch } from 'vue'
import Popper from 'vue3-popper'

import IconButton from '@/components/common/Button/IconButton.vue'
import TIcon from '@/components/common/Icon/TIcon.vue'

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

const menuHovered = ref(false)
</script>

<template>
  <div>
    <RadioGroup
      v-if="options.length < 8"
      :model-value="machineSetIndex"
      class="t-button-group flex gap-0.5 rounded bg-naturals-n3 p-1"
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
          <Popper
            :disabled="!option.tooltip"
            hover
            placement="left"
            :interactive="false"
            offset-distance="10"
          >
            <template #content>
              <div class="w-48 rounded bg-naturals-n4 px-4 py-2 text-naturals-n10 drop-shadow-sm">
                {{ option.tooltip }}
              </div>
            </template>
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
          </Popper>
        </div>
      </RadioGroupOption>
    </RadioGroup>

    <div v-else class="relative flex h-8 items-center justify-center rounded bg-naturals-n3">
      <Popper v-click-outside="() => (showPicker = false)" :show="showPicker" placement="left">
        <template #content>
          <div class="flex flex-col items-center gap-1 rounded bg-naturals-n3 p-1">
            <IconButton
              icon="arrow-up"
              @click="optionsView.$el.scrollBy({ top: -24, behavior: 'smooth' })"
            />
            <RadioGroup
              ref="optionsView"
              :model-value="machineSetIndex"
              class="no-scrollbar picker"
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
                  <Popper
                    :disabled="!option.tooltip"
                    hover
                    placement="left"
                    :interactive="false"
                    offset-distance="10"
                  >
                    <template #content>
                      <div
                        class="w-48 rounded bg-naturals-n4 px-4 py-2 text-naturals-n10 drop-shadow-sm"
                      >
                        {{ option.tooltip }}
                      </div>
                    </template>
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
                  </Popper>
                </div>
              </RadioGroupOption>
            </RadioGroup>
            <IconButton
              icon="arrow-down"
              @click="optionsView.$el.scrollBy({ top: 24, behavior: 'smooth' })"
            />
          </div>
        </template>
        <div
          class="flex h-6 items-center gap-1 px-1"
          @click="() => (showPicker = !showPicker)"
          @mouseover="menuHovered = true"
          @mouseout="menuHovered = false"
        >
          <TIcon
            icon="arrow-left"
            class="mx-1 h-3 w-3 text-naturals-n7 transition-all"
            :class="{ 'text-white': menuHovered, 'scale-125': menuHovered }"
          />
          <template v-if="pickedOption">
            <MachineSetLabel :machine-set-id="pickedOption?.id" :color="pickedOption?.color" />
            <IconButton
              icon="close"
              @click.stop="() => emit('update:machineSetIndex', undefined)"
            />
          </template>
          <IconButton v-else icon="action-horizontal" />
        </div>
      </Popper>
    </div>
  </div>
</template>

<style>
.no-scrollbar::-webkit-scrollbar {
  display: none;
}

/* Hide scrollbar for IE, Edge and Firefox */
.no-scrollbar {
  -ms-overflow-style: none; /* IE and Edge */
  scrollbar-width: none; /* Firefox */
}
</style>

<style>
@reference "../../../../index.css";

.picker {
  @apply flex flex-col items-center gap-0.5 overflow-y-auto;
  height: 120px;
}
</style>
