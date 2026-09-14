<!--
Copyright (c) 2026 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import { reactiveOmit } from '@vueuse/core'
import type { ClassValue } from 'clsx'
import {
  SelectItem,
  type SelectItemEmits,
  type SelectItemProps,
  useForwardPropsEmits,
} from 'reka-ui'

import { cn } from '@/methods/utils'

const props = defineProps<SelectItemProps & { class?: ClassValue }>()
// reka names this event `select`; mirroring its API here shadows the native DOM event
// eslint-disable-next-line vue/no-shadow-native-events
const emit = defineEmits<SelectItemEmits>()

const delegatedProps = reactiveOmit(props, 'class')
const forwarded = useForwardPropsEmits(delegatedProps, emit)
</script>

<template>
  <SelectItem
    v-bind="forwarded"
    :class="
      cn(
        'flex cursor-pointer items-center gap-1 p-1.5 font-medium text-naturals-n9 outline-none not-data-disabled:hover:text-naturals-n13 focus:text-naturals-n13 data-disabled:cursor-not-allowed data-disabled:text-naturals-n7 data-disabled:italic data-[state=checked]:text-primary-p3',
        props.class,
      )
    "
  >
    <slot></slot>
  </SelectItem>
</template>
