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
        'z-10 -mb-(--arrow-size) bg-linear-to-b from-naturals-n3 from-25% to-transparent',
        props.class,
      )
    "
  >
    <slot></slot>
  </SelectScrollUpButton>
</template>
