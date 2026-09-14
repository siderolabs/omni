<!--
Copyright (c) 2026 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import { reactiveOmit } from '@vueuse/core'
import type { ClassValue } from 'clsx'
import { SelectItemText, type SelectItemTextProps, useForwardProps } from 'reka-ui'

import { cn } from '@/methods/utils'

const props = defineProps<SelectItemTextProps & { class?: ClassValue }>()

const delegatedProps = reactiveOmit(props, 'class')
const forwarded = useForwardProps(delegatedProps)
</script>

<template>
  <SelectItemText v-bind="forwarded" :class="cn('truncate px-1 transition-all', props.class)">
    <slot></slot>
  </SelectItemText>
</template>
