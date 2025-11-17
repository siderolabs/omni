<!--
Copyright (c) 2025 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts" generic="T extends string | number | boolean | Record<string, any>">
import {
  RadioGroup,
  RadioGroupDescription,
  RadioGroupLabel,
  RadioGroupOption,
} from '@headlessui/vue'

defineProps<{
  label: string
  options: {
    label: string
    description?: string
    value: T
  }[]
}>()

const model = defineModel<T>()
</script>

<template>
  <RadioGroup v-model="model">
    <RadioGroupLabel class="mb-3 block text-sm font-medium text-naturals-n14">
      {{ label }}
    </RadioGroupLabel>

    <div class="flex flex-col items-start gap-4">
      <RadioGroupOption
        v-for="option in options"
        :key="option.label"
        v-slot="{ checked }"
        as="template"
        :value="option.value"
      >
        <div class="flex cursor-pointer items-center gap-2.5 rounded-md">
          <div
            class="size-3.5 shrink-0 rounded-full border bg-clip-content p-0.5 transition-colors duration-250"
            :class="
              checked ? 'border-primary-p4 bg-primary-p4' : 'border-naturals-n5 bg-transparent'
            "
          ></div>

          <div class="text-xs">
            <RadioGroupLabel as="p" class="font-medium text-naturals-n14">
              {{ option.label }}
            </RadioGroupLabel>

            <RadioGroupDescription v-if="option.description" as="span">
              {{ option.description }}
            </RadioGroupDescription>
          </div>
        </div>
      </RadioGroupOption>
    </div>
  </RadioGroup>
</template>
