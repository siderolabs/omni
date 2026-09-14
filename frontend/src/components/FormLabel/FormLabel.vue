<!--
Copyright (c) 2026 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import { reactiveOmit } from '@vueuse/core'
import type { ClassValue } from 'clsx'
import { Label, type LabelProps, useForwardProps } from 'reka-ui'

import { cn } from '@/methods/utils'

const props = defineProps<LabelProps & { class?: ClassValue }>()

const delegatedProps = reactiveOmit(props, 'class')
const forwarded = useForwardProps(delegatedProps)
</script>

<template>
  <Label
    v-bind="forwarded"
    :class="cn('inline-block text-sm font-medium text-naturals-n14', props.class)"
  >
    <slot></slot>
  </Label>
</template>
