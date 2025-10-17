<!--
Copyright (c) 2025 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import { vOnClickOutside } from '@vueuse/components'
import { computed, onMounted, ref, useTemplateRef, watch } from 'vue'

import type { IconType } from '@/components/common/Icon/TIcon.vue'
import TIcon from '@/components/common/Icon/TIcon.vue'

interface Props {
  icon?: IconType
  title?: string
  modelValue: string | number
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
  modelValue,
  placeholder = '',
  focus,
  type = 'text',
  max = undefined,
  min = undefined,
  step = 1,
  onClear = undefined,
} = defineProps<Props>()

const emit = defineEmits(['update:model-value', 'blur'])

defineExpose({
  getCaretPosition: () => inputRef.value?.selectionStart,
})

const numberValue = computed(() =>
  typeof modelValue === 'number' ? modelValue : parseFloat(modelValue),
)

const updateValue = (value: string | number) => {
  if (type !== 'number') {
    emit('update:model-value', value)
    return
  }

  if (value === undefined || value === '') return

  let numberValue = Number((typeof value === 'number' ? value : parseFloat(value)).toFixed(1))

  if (isNaN(numberValue)) numberValue = 0

  emit(
    'update:model-value',
    Math.max(min ?? numberValue, Math.min(max ?? numberValue, numberValue)),
  )
}

const isFocused = ref(false)
const inputRef = useTemplateRef('input')

const clearInput = () => {
  updateValue('')
  onClear?.()
}

const blurHandler = () => {
  isFocused.value = false
  emit('blur', '')
}

watch(
  () => focus,
  () => focus && inputRef.value?.focus(),
)

onMounted(() => focus && inputRef.value?.focus())
</script>

<template>
  <label
    v-on-click-outside="() => (isFocused = false)"
    class="input-box"
    :class="[{ focused: isFocused, secondary, primary: !secondary, compact, disabled }]"
    @click.prevent="
      () => {
        isFocused = true
        inputRef?.focus()
      }
    "
  >
    <TIcon v-if="icon" class="input-box-icon" :icon="icon" />
    <slot name="labels" />
    <span v-if="title" class="mr-1 min-w-fit text-xs">{{ title }}:</span>
    <input
      ref="input"
      :class="{ 'opacity-0': disabled }"
      :readonly="disabled"
      :value="modelValue"
      :type="type"
      class="input-box-input"
      :placeholder="placeholder"
      @input="updateValue(($event.target as HTMLInputElement)?.value.trim())"
      @focus="isFocused = true"
      @blur="blurHandler"
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
    <div v-else-if="modelValue !== '' || onClear" @click.prevent="clearInput">
      <TIcon class="input-box-icon" icon="close" />
    </div>
  </label>
</template>

<style scoped>
@reference "../../../index.css";

.input-box {
  @apply flex items-center justify-start gap-2 gap-y-3 rounded border border-naturals-n8 p-2 transition-colors;
}

.compact {
  @apply p-1 px-2;
}

.input-box-icon {
  @apply h-4 w-4 cursor-pointer fill-current text-naturals-n8 transition-colors;
}

.input-box-input {
  @apply min-w-2 flex-1 border-none bg-transparent text-xs text-naturals-n13 placeholder-naturals-n7 outline-hidden transition-colors focus:border-transparent focus:outline-hidden;
}

.input-box-icon-wrapper {
  @apply min-w-4;
}

.secondary {
  @apply border-transparent;
}

.focused {
  @apply border border-solid border-naturals-n5;
}

.focused .input-box-icon {
  @apply text-naturals-n14;
}

input::-webkit-outer-spin-button,
input::-webkit-inner-spin-button {
  @apply m-0 appearance-none;
}

/* Firefox */
input[type='number'] {
  appearance: textfield;
}

.disabled {
  @apply cursor-not-allowed border-naturals-n6 bg-naturals-n3;
}

.disabled * {
  @apply pointer-events-none text-naturals-n9 select-none;
}
</style>
