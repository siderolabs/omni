<!--
Copyright (c) 2025 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts" generic="T extends string | number | boolean | Record<string, unknown>">
import { RadioGroup, RadioGroupLabel } from '@headlessui/vue'

defineProps<{
  label: string
}>()

const model = defineModel<T>()
</script>

<template>
  <RadioGroup v-model="model">
    <RadioGroupLabel class="mb-3 block text-sm font-medium text-naturals-n14">
      {{ label }}
    </RadioGroupLabel>

    <div class="flex flex-col items-start gap-4">
      <slot></slot>
    </div>
  </RadioGroup>
</template>
