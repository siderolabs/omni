<!--
Copyright (c) 2026 Sidero Labs, Inc.

Use of this software is governed by the Business Source License
included in the LICENSE file.
-->
<script setup lang="ts">
import { CollapsibleContent, CollapsibleRoot, CollapsibleTrigger } from 'reka-ui'

import TIcon from '@/components/Icon/TIcon.vue'

const { isDefaultOpened } = defineProps<{
  isDefaultOpened?: boolean
  disableBorderOnExpand?: boolean
}>()

defineSlots<{
  default(): unknown
  secondary(): unknown
  details(): unknown
}>()
</script>

<template>
  <CollapsibleRoot
    v-slot="{ open }"
    class="group flex w-full flex-col rounded-t-sm border-b border-border-strong px-tight py-compact text-xs text-content-default transition-all duration-500 last-of-type:border-none"
    :class="
      !disableBorderOnExpand &&
      'data-[state=open]:mt-micro data-[state=open]:rounded data-[state=open]:last-of-type:border-border-strong'
    "
    role="row"
    :default-open="isDefaultOpened"
  >
    <div class="flex flex-col gap-micro">
      <div class="flex items-center gap-micro">
        <CollapsibleTrigger
          v-if="$slots.details"
          class="cursor-pointer rounded transition-colors hover:bg-surface-inert"
          :aria-label="open ? 'Collapse details' : 'Expand details'"
        >
          <TIcon
            class="size-6 cursor-pointer text-content-secondary transition-transform duration-300 group-data-[state=open]:-rotate-180"
            icon="drop-up"
            aria-hidden="true"
          />
        </CollapsibleTrigger>

        <div class="min-w-0 flex-1 px-micro">
          <slot></slot>
        </div>
      </div>

      <slot name="secondary"></slot>
    </div>

    <CollapsibleContent v-if="$slots.details" class="collapsible-content overflow-hidden">
      <div class="p-tight">
        <slot name="details"></slot>
      </div>
    </CollapsibleContent>
  </CollapsibleRoot>
</template>

<style scoped>
.collapsible-content[data-state='open'] {
  animation: slideDown 200ms ease-out;
}

.collapsible-content[data-state='closed'] {
  animation: slideUp 200ms ease-out;
}

@keyframes slideDown {
  from {
    height: 0;
  }
  to {
    height: var(--reka-collapsible-content-height);
  }
}

@keyframes slideUp {
  from {
    height: var(--reka-collapsible-content-height);
  }
  to {
    height: 0;
  }
}
</style>
