<!--
Copyright (c) 2026 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import { useClipboard } from '@vueuse/core'

import TIcon from '@/components/common/Icon/TIcon.vue'

const { text = '' } = defineProps<{ text?: string }>()
const { copy, copied } = useClipboard({ copiedDuring: 1000 })
</script>

<template>
  <button
    aria-label="copy"
    class="relative size-4 hover:brightness-125 active:brightness-75"
    @click.stop="copy(text)"
  >
    <TIcon
      icon="check"
      class="absolute inset-0 text-green-g2 transition-all duration-300"
      :class="[copied ? 'opacity-100' : 'opacity-0']"
    />
    <TIcon
      icon="copy"
      class="absolute inset-0 text-primary-p2 transition-all duration-300"
      :class="[copied ? 'opacity-0' : 'opacity-100']"
    />
  </button>
</template>
