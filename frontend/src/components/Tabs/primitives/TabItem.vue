<!--
Copyright (c) 2026 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import { Primitive } from 'reka-ui'
import type { ClassValue } from 'vue'

import { cn } from '@/methods/utils'

// The look of one tab, shared by TabButton (shows a panel) and TabNav (links to a page). It
// renders nothing itself: it styles the single element it wraps, which marks its own state
// with data-state="active" and data-disabled.
const props = defineProps<{ class?: ClassValue }>()
</script>

<template>
  <Primitive
    as-child
    :class="
      cn(
        'text-sm transition-colors not-data-disabled:hover:text-content-default data-disabled:cursor-not-allowed data-disabled:text-content-disabled data-[state=active]:text-content-default',
        props.class,
      )
    "
  >
    <slot></slot>
  </Primitive>
</template>
