<!--
Copyright (c) 2026 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import { Primitive } from 'reka-ui'
import type { ClassValue } from 'vue'

import { cn } from '@/methods/utils'

// The look of one item in a button group, shared by TButtonGroup (picks a value) and
// TButtonGroupNav (picks a page). It renders nothing itself: it styles the single element
// it wraps, which marks its own state with data-state="checked" and data-disabled.
const props = defineProps<{ class?: ClassValue }>()
</script>

<template>
  <Primitive
    as-child
    :class="
      cn(
        'rounded border-border-strong text-xs text-content-muted transition-colors duration-200 hover:bg-surface-inert hover:text-content-default data-disabled:cursor-not-allowed data-disabled:text-content-muted data-disabled:hover:bg-surface-raised data-[state=checked]:bg-surface-inert data-[state=checked]:text-content-emphasis data-[state=checked]:ring-1 data-[state=checked]:ring-border-strong',
        props.class,
      )
    "
  >
    <slot></slot>
  </Primitive>
</template>
