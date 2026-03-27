<!--
Copyright (c) 2026 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
interface Props {
  title?: string
  overheadTitle?: boolean
  placeholder?: string
  disabled?: boolean
}

const { title, placeholder } = defineProps<Props>()

const modelValue = defineModel<string>()
</script>

<template>
  <label class="flex flex-col gap-4 text-sm font-medium text-naturals-n14">
    <span v-if="title && overheadTitle" class="text-sm">
      {{ title }}
    </span>

    <div
      class="flex max-h-full items-center justify-start gap-x-2 gap-y-3 rounded border border-naturals-n8 p-2 transition-colors focus-within:border-naturals-n5 has-disabled:pointer-events-none has-disabled:cursor-not-allowed has-disabled:border-naturals-n6 has-disabled:bg-naturals-n3 has-disabled:text-naturals-n9 has-disabled:select-none"
    >
      <span v-if="title && !overheadTitle" class="mr-1 min-w-fit text-xs after:content-[':']">
        {{ title }}
      </span>

      <textarea
        v-model.trim="modelValue"
        :disabled="disabled"
        class="peer min-w-2 flex-1 border-none bg-transparent text-xs text-naturals-n13 placeholder-naturals-n7 outline-hidden transition-colors focus:border-transparent focus:outline-hidden disabled:opacity-0"
        :placeholder="placeholder"
      ></textarea>
    </div>
  </label>
</template>
