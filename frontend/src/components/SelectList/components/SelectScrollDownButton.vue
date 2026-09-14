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
        'z-10 -mt-(--arrow-size) bg-linear-to-t from-naturals-n3 from-25% to-transparent',
        props.class,
      )
    "
  >
    <slot></slot>
  </SelectScrollDownButton>
</template>
