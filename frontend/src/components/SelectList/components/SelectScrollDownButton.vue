<!--
Copyright (c) 2026 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import { reactiveOmit } from '@vueuse/core'
import type { ClassValue } from 'clsx'
import { SelectScrollDownButton, type SelectScrollDownButtonProps, useForwardProps } from 'reka-ui'

import { cn } from '@/methods/utils'

const props = defineProps<SelectScrollDownButtonProps & { class?: ClassValue }>()

const delegatedProps = reactiveOmit(props, 'class')
const forwarded = useForwardProps(delegatedProps)
</script>

<template>
  <SelectScrollDownButton
    v-bind="forwarded"
    :class="
      cn(
        // See SelectScrollUpButton for the layering
        // eslint-disable-next-line design-system/no-off-menu-spacing --arrow-size is on system
        'relative -mt-(--arrow-size) *:relative *:z-20 before:absolute before:inset-0 before:z-5 before:bg-linear-to-t before:from-surface-raised before:from-25% before:to-transparent',
        props.class,
      )
    "
  >
    <slot></slot>
  </SelectScrollDownButton>
</template>
