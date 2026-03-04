<!--
Copyright (c) 2026 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import type { HTMLAttributes } from 'vue'

interface Props extends /* @vue-ignore */ HTMLAttributes {
  labelClass?: HTMLAttributes['class']
  disabled?: boolean
  checked?: boolean
}

defineProps<Props>()
</script>

<template>
  <div
    class="resource-label select-none"
    :class="[
      'rounded border',
      checked ? 'border-current/50' : 'border-transparent',
      disabled
        ? 'cursor-not-allowed opacity-50'
        : ['cursor-pointer opacity-75 transition-opacity hover:opacity-100', labelClass],
    ]"
  >
    <slot></slot>
  </div>
</template>
