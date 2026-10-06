<!--
Copyright (c) 2026 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import { useClipboard } from '@vueuse/core'
import type { ButtonHTMLAttributes } from 'vue'

import TIcon from '@/components/Icon/TIcon.vue'

interface Props extends /* @vue-ignore */ ButtonHTMLAttributes {
  text?: string
}

const { text = '' } = defineProps<Props>()
const { copy, copied } = useClipboard({ copiedDuring: 1000 })
</script>

<template>
  <button aria-label="copy" class="group relative size-4" @click.stop="copy(text)">
    <TIcon
      icon="check"
      class="absolute inset-0 size-4 text-status-success-text transition-all duration-300"
      :class="[copied ? 'opacity-100' : 'opacity-0']"
    />
    <TIcon
      icon="copy"
      class="absolute inset-0 size-4 text-content-secondary transition-all duration-300 group-hover:text-content-emphasis"
      :class="[copied ? 'opacity-0' : 'opacity-100']"
    />
  </button>
</template>
