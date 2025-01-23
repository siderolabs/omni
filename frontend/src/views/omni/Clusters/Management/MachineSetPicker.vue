<!--
Copyright (c) 2025 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<template>
  <div>
    <radio-group v-if="options.length < 8" :modelValue="machineSetIndex" @update:modelValue="value => emit('update:machineSetIndex', value)" class="flex p-1 bg-naturals-N3 rounded gap-0.5 t-button-group">
      <radio-group-option v-for="(option, index) in options" :key="index" :value="index" as="template"
        :disabled="option.disabled">
        <div @click="() => onSelect(index)">
          <popper :disabled="!option.tooltip" hover placement="left" :interactive="false" offsetDistance="10">
            <template #content>
              <div class="rounded px-4 py-2 text-naturals-N10 bg-naturals-N4 drop-shadow w-48">
                {{ option.tooltip }}
              </div>
            </template>
            <div class="relative">
              <machine-set-label class="opacity-75 hover:opacity-100 transition-opacity machine-set-label"
                :color="option.color" :class="{ 'opacity-100': machineSetIndex === index && !option.disabled, disabled: option?.disabled }"
                :id="option.id"
                :machine-set-id="option.id" :disabled="option?.disabled"/>
              <div v-if="machineSetIndex === index"
                class="absolute top-0 left-0 right-0 bottom-0 bg-naturals-N12 pointer-events-none mix-blend-overlay rounded border-white border"/>
            </div>
          </popper>
        </div>
      </radio-group-option>
    </radio-group>

    <div v-else class="flex items-center justify-center bg-naturals-N3 rounded relative h-8">
      <popper :show="showPicker" placement="left"
              v-click-outside="() => showPicker = false">
        <template #content>
          <div class="bg-naturals-N3 p-1 rounded gap-1 flex flex-col items-center">
            <icon-button icon="arrow-up" @click="optionsView.$el.scrollBy({top: -24, behavior: 'smooth'})"/>
            <radio-group :modelValue="machineSetIndex" @update:modelValue="value => emit('update:machineSetIndex', value)"
              @scroll.stop
              ref="optionsView"
              class="no-scrollbar picker">
              <radio-group-option v-for="(option, index) in options" :key="index" v-slot="{ checked }" :value="index" as="template"
                :disabled="option.disabled">
                <div @click="() => onSelect(checked)">
                  <popper :disabled="!option.tooltip" hover placement="left" :interactive="false" offsetDistance="10">
                    <template #content>
                      <div class="rounded px-4 py-2 text-naturals-N10 bg-naturals-N4 drop-shadow w-48">
                        {{ option.tooltip }}
                      </div>
                    </template>
                    <div class="relative">
                      <machine-set-label class="opacity-75 hover:opacity-100 transition-opacity machine-set-label"
                        :color="option.color" :class="{ 'opacity-100': checked && !option.disabled, disabled: option?.disabled }"
                        :machine-set-id="option.id" :disabled="option?.disabled" />
                      <div v-if="checked"
                        class="absolute top-0 left-0 right-0 bottom-0 bg-naturals-N12 pointer-events-none mix-blend-overlay rounded border-white border"/>
                    </div>
                  </popper>
                </div>
              </radio-group-option>
            </radio-group>
            <icon-button icon="arrow-down" @click="optionsView.$el.scrollBy({top: 24, behavior: 'smooth'})"/>
          </div>
        </template>
        <div class="h-6 px-1 flex gap-1 items-center" @click="() => showPicker = !showPicker" @mouseover="menuHovered = true" @mouseout="menuHovered = false">
          <t-icon icon="arrow-left" class="w-3 mx-1 h-3 text-naturals-N7 transition-all" :class="{'text-white': menuHovered, 'scale-125': menuHovered}"/>
          <template v-if="pickedOption">
            <machine-set-label :machine-set-id="pickedOption?.id" :color="pickedOption?.color"/>
            <icon-button icon="close" @click.stop="() => emit('update:machineSetIndex', undefined)"/>
          </template>
          <icon-button v-else icon="action-horizontal"/>
        </div>
      </popper>
    </div>
  </div>
</template>

<script setup lang="ts">
import { RadioGroup, RadioGroupOption } from "@headlessui/vue";
import { computed, ref, toRefs, watch } from "vue";

import IconButton from "@/components/common/Button/IconButton.vue";
import TIcon from "@/components/common/Icon/TIcon.vue";

import Popper from 'vue3-popper';
import MachineSetLabel from "./MachineSetLabel.vue";

export type PickerOption = {
  id: string,
  color: string,
  tooltip?: string,
  name?: string,
  disabled?: boolean
};

const emit = defineEmits(["update:machineSetIndex"]);
const showPicker = ref(false);
const optionsView = ref();

const props = defineProps<{
  machineSetIndex?: number,
  options: PickerOption[]
}>();

const { options, machineSetIndex } = toRefs(props);

watch(options, () => {
  if (options.value.length < 8) {
    showPicker.value = false;
  }
});

const pickedOption = computed(() => {
  return machineSetIndex?.value !== undefined ? options.value[machineSetIndex.value] : undefined;
});

const onSelect = (index: number) => {
  if (machineSetIndex.value === index) {
    emit('update:machineSetIndex', undefined);
  } else {
    emit('update:machineSetIndex', index);
  }

  showPicker.value = false;
}

const menuHovered = ref(false);
</script>

<style>
.no-scrollbar::-webkit-scrollbar {
  display: none;
}

/* Hide scrollbar for IE, Edge and Firefox */
.no-scrollbar {
  -ms-overflow-style: none;  /* IE and Edge */
  scrollbar-width: none;  /* Firefox */
}
</style>

<style>
.picker {
  @apply flex flex-col items-center gap-0.5 overflow-y-auto;
  height: 120px;
}
</style>
