<!--
Copyright (c) 2026 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import { reactiveOmit } from '@vueuse/core'
import { Label, type LabelProps, useForwardProps } from 'reka-ui'
import type { ClassValue } from 'vue'

import { cn } from '@/methods/utils'

const props = defineProps<
  LabelProps & {
    hideSelectedSmallScreens?: boolean
    class?: ClassValue
  }
>()

const delegatedProps = reactiveOmit(props, 'hideSelectedSmallScreens', 'class')
const forwarded = useForwardProps(delegatedProps)
</script>

<template>
  <Label
    v-bind="forwarded"
    :class="
      cn(hideSelectedSmallScreens ? `md:after:content-[':']` : `after:content-[':']`, props.class)
    "
  >
    <slot></slot>
  </Label>
</template>
