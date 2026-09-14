<!--
Copyright (c) 2026 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import { reactiveOmit } from '@vueuse/core'
import type { ClassValue } from 'clsx'
import {
  SelectContent,
  type SelectContentEmits,
  type SelectContentProps,
  useForwardPropsEmits,
} from 'reka-ui'

import { cn } from '@/methods/utils'

const props = withDefaults(defineProps<SelectContentProps & { class?: ClassValue }>(), {
  position: 'popper',
  side: 'bottom',
})
const emit = defineEmits<SelectContentEmits>()

const delegatedProps = reactiveOmit(props, 'class')
const forwarded = useForwardPropsEmits(delegatedProps, emit)
</script>

<template>
  <SelectContent
    v-bind="forwarded"
    :class="
      cn(
        'relative z-50 max-h-[min(--spacing(70),var(--reka-select-content-available-height))] min-w-(--reka-select-trigger-width) translate-y-1 space-y-1 overflow-hidden rounded border border-naturals-n4 bg-naturals-n3 p-1.5 text-xs [--arrow-size:--spacing(4)] slide-in-from-top-2 data-[state=closed]:animate-out data-[state=closed]:fade-out-0 data-[state=closed]:zoom-out-95 data-[state=open]:animate-in data-[state=open]:fade-in-0 data-[state=open]:zoom-in-95',
        props.class,
      )
    "
  >
    <slot></slot>
  </SelectContent>
</template>
