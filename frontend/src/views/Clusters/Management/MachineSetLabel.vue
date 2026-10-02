<!--
Copyright (c) 2026 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import type { HTMLAttributes } from 'vue'

interface Props extends /* @vue-ignore */ HTMLAttributes {
  markerClass?: HTMLAttributes['class']
  disabled?: boolean
  checked?: boolean
  static?: boolean
  segment?: boolean
}

defineProps<Props>()
</script>

<template>
  <div
    class="inline-flex items-center gap-micro text-xs whitespace-nowrap transition-colors select-none"
    :class="[
      segment ? 'h-full px-snug py-micro' : 'rounded-sm px-tight py-micro',
      static
        ? 'bg-surface-hover text-content-default'
        : disabled
          ? 'cursor-not-allowed text-content-disabled'
          : checked
            ? segment
              ? 'cursor-pointer bg-surface-inert text-content-emphasis'
              : 'cursor-pointer bg-surface-inert text-content-emphasis ring-1 ring-border-strong'
            : segment
              ? 'cursor-pointer text-content-default hover:bg-surface-hover hover:text-content-emphasis'
              : 'cursor-pointer text-content-secondary hover:bg-surface-hover hover:text-content-default',
    ]"
  >
    <span
      aria-hidden="true"
      class="size-2 shrink-0 rounded-full"
      :class="disabled && !static ? 'bg-content-disabled' : markerClass"
    />
    <slot></slot>
  </div>
</template>
