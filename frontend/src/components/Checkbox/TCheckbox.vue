<!--
Copyright (c) 2026 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import { CheckboxIndicator, CheckboxRoot } from 'reka-ui'

import TIcon from '@/components/Icon/TIcon.vue'

type Props = {
  label?: string
  disabled?: boolean
  indeterminate?: boolean
}

const { label = '' } = defineProps<Props>()

const checked = defineModel<boolean>({ default: false })
</script>

<template>
  <label class="inline-flex cursor-pointer items-center gap-tight has-disabled:cursor-not-allowed">
    <CheckboxRoot
      v-model="checked"
      :disabled
      class="flex size-3.5 items-center justify-center rounded-xs border border-border-control transition-colors data-disabled:border-border-strong data-disabled:bg-surface-hover not-data-disabled:data-[state=checked]:border-accent-fill not-data-disabled:data-[state=checked]:bg-accent-fill"
    >
      <CheckboxIndicator class="transition-opacity data-[state=unchecked]:opacity-0" force-mount>
        <TIcon
          class="size-full text-content-on-accent in-data-disabled:text-content-disabled"
          :icon="indeterminate ? 'minus' : 'check'"
        />
      </CheckboxIndicator>
    </CheckboxRoot>

    <span
      v-if="label || $slots.default"
      class="block flex-1 truncate text-xs text-content-secondary select-none"
    >
      <slot>{{ label }}</slot>
    </span>
  </label>
</template>
