<!--
Copyright (c) 2026 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import { SwitchRoot, SwitchThumb } from 'reka-ui'

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
    <span
      v-if="label || $slots.default"
      class="grow truncate text-xs text-naturals-n11 select-none"
    >
      <slot>{{ label }}</slot>
    </span>

    <SwitchRoot
      v-model="checked"
      :disabled
      class="inline-flex h-5 w-8 rounded-full border border-naturals-n7 bg-naturals-n4 transition-[background] disabled:cursor-not-allowed disabled:brightness-50 data-[state=checked]:border-primary-p3 data-[state=checked]:bg-primary-p3"
    >
      <SwitchThumb
        class="my-auto size-3.5 translate-x-0.5 rounded-full bg-naturals-n13 text-xs transition-transform data-[state=checked]:translate-x-full"
      />
    </SwitchRoot>
  </label>
</template>
