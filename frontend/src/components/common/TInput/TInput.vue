<!--
Copyright (c) 2025 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import type { Ref } from 'vue'
import { computed, onMounted, ref, toRefs, watch } from 'vue'

import type { IconType } from '@/components/common/Icon/TIcon.vue'
import TIcon from '@/components/common/Icon/TIcon.vue'

const emit = defineEmits(['update:model-value', 'blur'])

type propsType = {
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

const props = withDefaults(defineProps<propsType>(), {
  type: 'text',
  step: 1,
})

const { modelValue, focus } = toRefs(props)

const numberValue = computed(() => {
  return parseFloat(modelValue.value as string) ?? 0
})

const updateValue = (value: string | number) => {
  if (props.type === 'number') {
    if (value === undefined || value === '') {
      return
    }

    let numberValue = typeof value === 'number' ? (value as number) : parseFloat(value)

    numberValue = Number(numberValue.toFixed(1))

    if (isNaN(numberValue)) {
      numberValue = 0
    }

    if (props.max !== undefined) {
      numberValue = Math.min(props.max, numberValue)
    }

    if (props.min !== undefined) {
      numberValue = Math.max(props.min, numberValue)
    }

    emit('update:model-value', numberValue)

    return
  }

  emit('update:model-value', value)
}

defineExpose({
  getCaretPosition(): number | void {
    if (!input.value) {
      return
    }

    return input.value.selectionStart
  },
})

const isFocused = ref(false)
const input: Ref<{ focus: () => void; selectionStart: number } | null> = ref(null)

const clearInput = () => {
  updateValue('')

  if (props.onClear) {
    props.onClear()
  }
}

const blurHandler = () => {
  isFocused.value = false
  emit('blur', '')
}

if (focus.value) {
  watch(focus, () => {
    if (focus.value && input.value) {
      input.value?.focus()
    }
  })
}

onMounted(() => {
  if (focus?.value && input.value) {
    input.value.focus()
  }
})
</script>

<template>
  <label
    v-click-outside="() => (isFocused = false)"
    class="input-box"
    :class="[{ focused: isFocused, secondary, primary: !secondary, compact, disabled }]"
    @click.prevent="
      () => {
        isFocused = true
        if ($refs.input) ($refs.input as any).focus()
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
      @input="updateValue($event.target?.['value'].trim())"
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
  @apply flex-1 border-none bg-transparent text-xs text-naturals-n13 placeholder-naturals-n7 outline-hidden transition-colors focus:border-transparent focus:outline-hidden;

  min-width: 0.5rem;
}

.input-box-icon-wrapper {
  min-width: 16px;
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
  -webkit-appearance: none;
  margin: 0;
}

/* Firefox */
input[type='number'] {
  -moz-appearance: textfield;
}

.disabled {
  @apply cursor-not-allowed border-naturals-n6 bg-naturals-n3;
}

.disabled * {
  @apply pointer-events-none text-naturals-n9 select-none;
}
</style>
