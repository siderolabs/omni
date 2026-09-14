<!--
Copyright (c) 2026 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script lang="ts">
import type { ClassValue } from 'clsx'
import type { SelectTriggerProps as RekaSelectTriggerProps } from 'reka-ui'

export interface SelectTriggerProps extends RekaSelectTriggerProps {
  variant?: 'default' | 'breadcrumb'
  class?: ClassValue
}
</script>

<script setup lang="ts">
import { reactiveOmit } from '@vueuse/core'
import { SelectTrigger, useForwardProps } from 'reka-ui'

import { cn } from '@/methods/utils'

const props = withDefaults(defineProps<SelectTriggerProps>(), {
  variant: 'default',
})

const delegatedProps = reactiveOmit(props, 'variant', 'class')
const forwarded = useForwardProps(delegatedProps)
</script>

<template>
  <SelectTrigger
    v-bind="forwarded"
    :class="
      cn(
        'flex max-h-full w-full items-center justify-between gap-1 rounded text-naturals-n14 transition-colors disabled:cursor-not-allowed disabled:opacity-50',
        {
          'border border-naturals-n7 bg-naturals-n2 px-3 py-2.25 text-xs': variant === 'default',
          'p-2 leading-none hover:bg-naturals-n4': variant === 'breadcrumb',
        },
        props.class,
      )
    "
  >
    <slot></slot>
  </SelectTrigger>
</template>
