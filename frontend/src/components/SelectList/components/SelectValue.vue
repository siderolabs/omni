<!--
Copyright (c) 2026 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import { reactiveOmit } from '@vueuse/core'
import type { ClassValue } from 'clsx'
import { SelectValue, type SelectValueProps, useForwardProps } from 'reka-ui'

import { cn } from '@/methods/utils'

const props = defineProps<SelectValueProps & { class?: ClassValue }>()

const delegatedProps = reactiveOmit(props, 'class')
const forwarded = useForwardProps(delegatedProps)
</script>

<template>
  <SelectValue
    v-bind="forwarded"
    :class="cn('flex items-center gap-1 truncate select-none', props.class)"
  >
    <slot></slot>
  </SelectValue>
</template>
