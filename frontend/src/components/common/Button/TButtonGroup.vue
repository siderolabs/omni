<!--
Copyright (c) 2025 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import { RadioGroup, RadioGroupOption } from '@headlessui/vue'

import Tooltip from '@/components/common/Tooltip/Tooltip.vue'

type Props = {
  modelValue: any
  deselectEnabled?: boolean
  options: {
    label: string
    value: any
    disabled?: boolean
    tooltip?: string
  }[]
}

defineProps<Props>()

const emit = defineEmits(['update:modelValue'])
</script>

<template>
  <RadioGroup
    :model-value="modelValue"
    class="t-button-group flex gap-0.5 rounded bg-naturals-n3 p-1"
    @update:model-value="(value) => emit('update:modelValue', value)"
  >
    <RadioGroupOption
      v-for="(option, index) in options"
      :key="index"
      v-slot="{ checked }"
      :value="option.value"
      as="template"
      :disabled="option.disabled"
    >
      <div @click="() => (checked && deselectEnabled ? emit('update:modelValue', null) : null)">
        <Tooltip :description="option.tooltip" placement="top">
          <button type="button" :class="{ checked }" :disabled="option?.disabled">
            <span>
              {{ option?.label || option.value }}
            </span>
          </button>
        </Tooltip>
      </div>
    </RadioGroupOption>
  </RadioGroup>
</template>

<style scoped>
@reference "../../../index.css";

.t-button-group button {
  @apply flex items-center justify-center gap-1 border-naturals-n5 px-2 py-0.5 text-xs text-naturals-n10 transition-colors duration-200 hover:bg-naturals-n5 hover:text-naturals-n12;
}

.t-button-group button {
  @apply rounded;
}

.t-button-group button[disabled] {
  @apply cursor-not-allowed text-naturals-n8 hover:bg-naturals-n3;
}

.checked {
  @apply bg-naturals-n4;
}

.checked span {
  @apply text-naturals-n12;
}

.popper {
  margin: 0 !important;
  border: 0 !important;
  display: block !important;
  z-index: auto !important;
}
</style>
