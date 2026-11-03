<!--
Copyright (c) 2026 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import { reactiveOmit } from '@vueuse/core'
import type { ClassValue } from 'clsx'
import { SelectScrollUpButton, type SelectScrollUpButtonProps, useForwardProps } from 'reka-ui'

import { cn } from '@/methods/utils'

const props = defineProps<SelectScrollUpButtonProps & { class?: ClassValue }>()

const delegatedProps = reactiveOmit(props, 'class')
const forwarded = useForwardProps(delegatedProps)
</script>

<template>
  <SelectScrollUpButton
    v-bind="forwarded"
    :class="
      cn(
        // No z-index on the button itself, so it doesn't form a stacking context: the gradient
        // (z-5) sits below sticky group labels (z-10), while the icon (z-20) stays above them.
        // eslint-disable-next-line design-system/no-off-menu-spacing --arrow-size is on system
        'relative -mb-(--arrow-size) *:relative *:z-20 before:absolute before:inset-0 before:z-5 before:bg-linear-to-b before:from-surface-raised before:from-25% before:to-transparent',
        props.class,
      )
    "
  >
    <slot></slot>
  </SelectScrollUpButton>
</template>
