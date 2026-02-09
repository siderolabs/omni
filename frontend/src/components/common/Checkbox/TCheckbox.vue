<!--
Copyright (c) 2026 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import { CheckboxIndicator, CheckboxRoot } from 'reka-ui'

import TIcon from '@/components/common/Icon/TIcon.vue'

type Props = {
  label?: string
  disabled?: boolean
  indeterminate?: boolean
}

const { label = '' } = defineProps<Props>()

const checked = defineModel<boolean>({ default: false })
</script>

<template>
  <label class="inline-flex cursor-pointer items-center gap-2 has-disabled:cursor-not-allowed">
    <CheckboxRoot
      v-model="checked"
      :disabled
      class="flex size-3.5 items-center justify-center rounded-xs border border-naturals-n7 transition-colors data-disabled:border-naturals-n5 data-disabled:bg-naturals-n4 not-data-disabled:data-[state=checked]:border-primary-p6 not-data-disabled:data-[state=checked]:bg-primary-p6"
    >
      <CheckboxIndicator class="transition-opacity data-[state=unchecked]:opacity-0" force-mount>
        <TIcon
          class="size-full fill-current text-primary-p3"
          :icon="indeterminate ? 'minus' : 'check'"
        />
      </CheckboxIndicator>
    </CheckboxRoot>

    <span
      v-if="label || $slots.default"
      class="block flex-1 truncate text-xs text-naturals-n11 select-none"
    >
      <slot>{{ label }}</slot>
    </span>
  </label>
</template>
