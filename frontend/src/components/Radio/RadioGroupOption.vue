<!--
Copyright (c) 2026 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import { RadioGroupDescription, RadioGroupLabel, RadioGroupOption } from '@headlessui/vue'

defineProps<{
  value: string | number | boolean | Record<string, unknown>
}>()
</script>

<template>
  <RadioGroupOption v-slot="{ checked }" as="template" :value>
    <div class="flex cursor-pointer items-center gap-2.5 rounded-md">
      <div
        class="size-3.5 shrink-0 rounded-full border bg-clip-content p-0.5 transition-colors duration-250"
        :class="
          checked ? 'border-accent-fill bg-accent-fill' : 'border-border-strong bg-transparent'
        "
      ></div>

      <div class="text-xs">
        <RadioGroupLabel as="p" class="font-medium text-content-emphasis">
          <slot></slot>
        </RadioGroupLabel>

        <RadioGroupDescription v-if="$slots.description" as="span">
          <slot name="description"></slot>
        </RadioGroupDescription>
      </div>
    </div>
  </RadioGroupOption>
</template>
