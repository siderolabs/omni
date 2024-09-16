<!--
Copyright (c) 2024 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<template>
  <label
    @click.prevent="
      () => {
        isFocused = true;
        if ($refs.input) ($refs.input as any).focus();
      }
    "
    v-click-outside="() => (isFocused = false)"
    class="input-box"
    :class="[{ focused: isFocused, secondary, primary: !secondary, compact, disabled }]"
  >
    <t-icon class="input-box-icon" v-if="icon" :icon="icon"/>
    <slot name="labels"/>
    <template v-if="!disabled">
      <span v-if="title" class="text-xs min-w-fit mr-1">{{title}}:</span>
      <input
        @input="updateValue($event.target?.['value'].trim())"
        ref="input"
        :value="modelValue"
        :type="type"
        class="input-box-input"
        @focus="isFocused = true"
        @blur="blurHandler"
        :placeholder="placeholder"
      />
      <div v-if="type === 'number'" class="flex flex-col select-none">
        <t-icon class="hover:text-naturals-N14 w-2" icon="arrow-up" @click="updateValue(numberValue + step)"/>
        <t-icon class="hover:text-naturals-N14 w-2" icon="arrow-down"  @click="updateValue(numberValue - step)"/>
      </div>
      <div v-else-if="modelValue !== '' || onClear" @click.prevent="clearInput">
        <t-icon
          class="input-box-icon"
          icon="close"
        />
      </div>
    </template>
  </label>
</template>

<script setup lang="ts">
import { watch, ref, toRefs, onMounted, Ref, computed } from "vue";

import TIcon, { IconType } from "@/components/common/Icon/TIcon.vue";

const emit = defineEmits(["update:model-value", "blur"]);

type propsType = {
  icon?: IconType,
  title?: string,
  modelValue: string | number,
  placeholder?: string,
  focus?: boolean,
  secondary?: boolean,
  compact?: boolean,
  type?: "text" | "number" | "password",
  max?: number,
  min?: number,
  disabled?: boolean
  step?: number
  onClear?: () => void
}

const props = withDefaults(
  defineProps<propsType>(),
  {
    type: "text",
    step: 1
  }
);

const { modelValue, focus } = toRefs(props);

const numberValue = computed(() => {
  return parseFloat(modelValue.value as string) ?? 0;
});

const updateValue = (value: string | number) => {
  if (props.type === "number") {
    if (value === undefined || value === '') {
      return;
    }

    let numberValue = typeof value === "number" ? value as number : parseFloat(value);

    numberValue = Number(numberValue.toFixed(1));

    if (isNaN(numberValue)) {
      numberValue = 0;
    }

    if (props.max !== undefined) {
      numberValue = Math.min(props.max, numberValue);
    }

    if (props.min !== undefined) {
      numberValue = Math.max(props.min, numberValue);
    }

    emit("update:model-value", numberValue);

    return;
  }

  emit("update:model-value", value);
}

defineExpose({
  getCaretPosition(): number | void {
    if (!input.value) {
      return;
    }

    return input.value.selectionStart;
  }
})

const isFocused = ref(false);
const input: Ref<{focus: () => void, selectionStart: number} | null> = ref(null);

const clearInput = () => {
  updateValue("");

  if (props.onClear) {
    props.onClear();
  }
};

const blurHandler = () => {
  isFocused.value = false;
  emit("blur", "");
};

if (focus.value) {
  watch(focus, () => {
    if (focus.value && input.value) {
      input.value?.focus();
    }
  });
}

onMounted(() => {
  if (focus?.value && input.value) {
    input.value.focus();
  }
});
</script>

<style scoped>
.input-box {
  @apply flex justify-start items-center p-2 border border-naturals-N8 rounded transition-colors gap-2 gap-y-3;
}

.compact {
  @apply p-1 px-2;
}

.input-box-icon {
  @apply fill-current text-naturals-N8 transition-colors cursor-pointer w-4 h-4;
}

.input-box-input {
  @apply bg-transparent border-none outline-none flex-1 text-naturals-N13 focus:outline-none  focus:border-transparent text-xs transition-colors placeholder-naturals-N7;

  min-width: 0.5rem;
}

.input-box-icon-wrapper {
  min-width: 16px;
}

.secondary {
  @apply border-transparent;
}

.focused {
  @apply border border-solid border-naturals-N5;
}

.focused .input-box-icon {
  @apply text-naturals-N14;
}

input::-webkit-outer-spin-button,
input::-webkit-inner-spin-button {
  -webkit-appearance: none;
  margin: 0;
}

/* Firefox */
input[type=number] {
  -moz-appearance: textfield;
}

.disabled {
  @apply cursor-not-allowed bg-naturals-N3 border-naturals-N6;
}
</style>
