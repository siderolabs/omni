<!--
Copyright (c) 2026 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import { Primitive } from 'reka-ui'
import type { ClassValue } from 'vue'

import { cn } from '@/methods/utils'

// The look of the container around TButtonGroupItems. It renders nothing itself: it styles
// the single element it wraps.
const props = defineProps<{ class?: ClassValue }>()
</script>

<template>
  <Primitive as-child :class="cn('flex gap-micro rounded bg-surface-raised p-micro', props.class)">
    <slot></slot>
  </Primitive>
</template>
