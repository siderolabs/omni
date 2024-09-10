<!--
Copyright (c) 2024 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<template>
  <radio-group :modelValue="modelValue" @update:model-value="(value) => emit('update:modelValue', value)"
    class="flex p-1 bg-naturals-N3 rounded gap-0.5 t-button-group">
    <radio-group-option v-for="(option, index) in options" :key="index" v-slot="{ checked }" :value="option.value"
      as="template" :disabled="option.disabled">
      <div @click="() => (checked && deselectEnabled) ? emit('update:modelValue', null) : null">
        <popper :disabled="!option.tooltip" hover placement="left" :interactive="false" offsetDistance="10">
          <template #content>
            <div class="rounded px-4 py-2 text-naturals-N10 bg-naturals-N4 drop-shadow w-48">
              {{ option.tooltip }}
            </div>
          </template>
          <button type="button" :class="{ checked }" :disabled="option?.disabled">
            <span>
              {{ option?.label || option.value }}
            </span>
          </button>
        </popper>
      </div>
    </radio-group-option>
  </radio-group>
</template>

<script setup lang="ts">
import { RadioGroup, RadioGroupOption, } from '@headlessui/vue'
import Popper from 'vue3-popper';

type Props = {
  modelValue: any
  deselectEnabled?: boolean
  options: {
    label: string
    value: any
    disabled?: boolean
    tooltip?: string
  }[];
}

defineProps<Props>();

const emit = defineEmits(['update:modelValue']);
</script>

<style scoped>
.t-button-group button {
  @apply
    flex
    items-center
    justify-center
    gap-1
    text-xs
    transition-colors
    duration-200
    px-2
    py-0.5
    text-naturals-N10
    border-naturals-N5
    hover:bg-naturals-N5
    hover:text-naturals-N12;
}

.t-button-group button {
  @apply rounded;
}

.t-button-group button[disabled] {
  @apply text-naturals-N8 cursor-not-allowed hover:bg-naturals-N3;
}

.checked {
  @apply bg-naturals-N4;
}

.checked span {
  @apply text-naturals-N12;
}
</style>
