<!--
Copyright (c) 2026 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import type { HTMLAttributes } from 'vue'

import { cn } from '@/methods/utils'

interface Props extends /* @vue-ignore */ HTMLAttributes {
  disablePadding?: boolean
  class?: HTMLAttributes['class']
}

const { class: className } = defineProps<Props>()
</script>

<template>
  <main :class="cn({ 'px-4 py-2 md:px-6 md:py-4': !disablePadding }, className)">
    <slot></slot>
  </main>
</template>
