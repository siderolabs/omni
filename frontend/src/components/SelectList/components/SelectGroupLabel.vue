<!--
Copyright (c) 2026 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import { reactiveOmit } from '@vueuse/core'
import type { ClassValue } from 'clsx'
import { SelectLabel, type SelectLabelProps, useForwardProps } from 'reka-ui'

import { cn } from '@/methods/utils'

const props = defineProps<SelectLabelProps & { class?: ClassValue }>()

const delegatedProps = reactiveOmit(props, 'class')
const forwarded = useForwardProps(delegatedProps)
</script>

<template>
  <SelectLabel
    v-bind="forwarded"
    :class="
      cn(
        'sticky top-0 z-10 -mx-1.5 bg-naturals-n3 px-1.5 py-1 pl-[calc(--spacing(1.5)+(--spacing(4)))] text-[0.625rem] font-semibold tracking-wider text-naturals-n12 uppercase',
        props.class,
      )
    "
  >
    <slot></slot>
  </SelectLabel>
</template>
