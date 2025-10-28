<!--
Copyright (c) 2025 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts" generic="T extends string | number">
import { computed, onMounted, useTemplateRef, watch } from 'vue'

import type { IconType } from '@/components/common/Icon/TIcon.vue'
import TIcon from '@/components/common/Icon/TIcon.vue'

interface Props {
  icon?: IconType
  title?: string
  overheadTitle?: boolean
  placeholder?: string
  focus?: boolean
  secondary?: boolean
  compact?: boolean
  type?: 'text' | 'number' | 'password'
  max?: number
  min?: number
  disabled?: boolean
  step?: number
  onClear?: () => void
}

const {
  icon = undefined,
  title = '',
  placeholder = '',
  focus,
  type = 'text',
  max = undefined,
  min = undefined,
  step = 1,
  onClear = undefined,
} = defineProps<Props>()

const emit = defineEmits<{
  blur: []
}>()

defineExpose({
  getCaretPosition: () => inputRef.value?.selectionStart,
})

const numberValue = computed(() =>
  typeof modelValue.value === 'number' ? modelValue.value : parseFloat(modelValue.value),
)

const modelValue = defineModel<T>({
  required: true,
})

// This function only exists to handle generic typing issues based on how this component works
function updateValue(value: unknown) {
  modelValue.value = value as T
}

const inputRef = useTemplateRef('input')

const clearInput = () => {
  updateValue('')
  onClear?.()
}

const blurHandler = () => {
  const value = modelValue.value

  if (type === 'number' && value !== '') {
    const numberValue = Number((typeof value === 'number' ? value : parseFloat(value)).toFixed(1))

    updateValue(
      isNaN(numberValue)
        ? 0
        : Math.max(min ?? numberValue, Math.min(max ?? numberValue, numberValue)),
    )
  }

  emit('blur')
}

watch(
  () => focus,
  () => focus && inputRef.value?.focus(),
)

onMounted(() => focus && inputRef.value?.focus())
</script>

<template>
  <label class="flex flex-col gap-4 text-sm font-medium text-naturals-n14">
    <span v-if="title && overheadTitle" class="text-sm">
      {{ title }}
    </span>

    <div
      class="flex items-center justify-start gap-x-2 gap-y-3 rounded border transition-colors focus-within:border-naturals-n5 has-disabled:pointer-events-none has-disabled:cursor-not-allowed has-disabled:border-naturals-n6 has-disabled:bg-naturals-n3 has-disabled:text-naturals-n9 has-disabled:select-none"
      :class="[
        compact ? 'px-2 py-1' : 'p-2',
        secondary ? 'border-transparent' : 'border-naturals-n8',
      ]"
    >
      <slot name="labels"></slot>
      <span v-if="title && !overheadTitle" class="mr-1 min-w-fit text-xs after:content-[':']">
        {{ title }}
      </span>
      <input
        ref="input"
        v-model.trim="modelValue"
        :disabled="disabled"
        :type="type"
        class="peer min-w-2 flex-1 border-none bg-transparent text-xs text-naturals-n13 placeholder-naturals-n7 outline-hidden transition-colors focus:border-transparent focus:outline-hidden disabled:opacity-0"
        :placeholder="placeholder"
        @blur="blurHandler"
      />

      <TIcon
        v-if="icon"
        class="order-first size-4 fill-current text-naturals-n8 transition-colors peer-focus:text-naturals-n14"
        :icon="icon"
      />

      <div v-if="type === 'number'" class="-my-1 flex flex-col select-none">
        <TIcon
          class="h-2 w-2 rotate-180 text-naturals-n12 hover:text-naturals-n14"
          icon="arrow-down"
          @click="updateValue(numberValue + step)"
        />
        <TIcon
          class="h-2 w-2 text-naturals-n12 hover:text-naturals-n14"
          icon="arrow-down"
          @click="updateValue(numberValue - step)"
        />
      </div>

      <TIcon
        v-else-if="modelValue !== '' || onClear"
        role="button"
        aria-label="clear"
        class="size-4 fill-current text-naturals-n8 transition-colors peer-focus:text-naturals-n14"
        icon="close"
        @click="clearInput"
      />
    </div>
  </label>
</template>

<style scoped>
input::-webkit-outer-spin-button,
input::-webkit-inner-spin-button {
  margin: 0;
  appearance: none;
}

/* Firefox */
input[type='number'] {
  appearance: textfield;
}
</style>
